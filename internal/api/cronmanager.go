package api

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// cronSchedule tracks a running cron schedule entry.
type cronSchedule struct {
	ScriptID   string
	Name       string
	CronExpr   string
	TimeoutSec int
	NextRunAt  time.Time
	PrevRunAt  time.Time
	stopCh     chan struct{}
}

var (
	cronSchedules map[string]*cronSchedule
	cronMu        sync.Mutex
)

// InitCronScheduler starts the cron runner and loads existing schedules from DB.
func InitCronScheduler() {
	cronSchedules = make(map[string]*cronSchedule)
	loadCronSchedules()
	// Background tick loop: check every minute if any schedule needs re-registration.
	go cronTickLoop()
}

func cronTickLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cronMu.Lock()
		entries := make([]*cronSchedule, 0, len(cronSchedules))
		for _, s := range cronSchedules {
			entries = append(entries, s)
		}
		cronMu.Unlock()

		now := time.Now()
		for _, s := range entries {
			if !s.NextRunAt.IsZero() && now.After(s.NextRunAt) {
				go func(sc *cronSchedule) {
					runScriptByID(sc.ScriptID, sc.TimeoutSec)
					cronMu.Lock()
					if cs, ok := cronSchedules[sc.ScriptID]; ok {
						cs.PrevRunAt = time.Now()
						cs.NextRunAt = nextCronTime(cs.CronExpr, time.Now())
						updateNextRunAt(cs.ScriptID, cs.NextRunAt)
					}
					cronMu.Unlock()
				}(s)
			}
		}
	}
}

func loadCronSchedules() {
	type scriptRow struct {
		ID         string `db:"id"`
		Name       string `db:"name"`
		Enabled    bool   `db:"enabled"`
		CronExpr   string `db:"cron_expression"`
		TimeoutSec int    `db:"timeout_secs"`
	}
	var scripts []scriptRow
	if err := pb.DB().NewQuery(`
        SELECT id, name, enabled, COALESCE(cron_expression,'') as cron_expression, COALESCE(timeout_secs,30) as timeout_secs
        FROM scripts
        WHERE enabled = 1 AND cron_expression != ''
    `).All(&scripts); err != nil {
		return
	}
	for _, s := range scripts {
		registerCron(s.ID, s.Name, s.CronExpr, s.TimeoutSec)
	}
}

func registerCron(scriptID, name, cronExpr string, timeoutSec int) {
	cronMu.Lock()
	defer cronMu.Unlock()

	// Stop existing entry if any
	if existing, ok := cronSchedules[scriptID]; ok {
		close(existing.stopCh)
		delete(cronSchedules, scriptID)
	}

	if cronExpr == "" {
		return
	}

	next := nextCronTime(cronExpr, time.Now())
	if next.IsZero() {
		pb.Logger().Warn("Invalid cron expression", "script", name, "expr", cronExpr)
		return
	}

	entry := &cronSchedule{
		ScriptID:   scriptID,
		Name:       name,
		CronExpr:   cronExpr,
		TimeoutSec: timeoutSec,
		NextRunAt:  next,
		stopCh:     make(chan struct{}),
	}
	cronSchedules[scriptID] = entry

	// Update next_run_at in DB (without holding lock)
	nextCopy := next
	go func() {
		rec, err := pb.FindRecordById("scripts", scriptID)
		if err != nil {
			return
		}
		rec.Set("next_run_at", nextCopy.UTC().Format(time.RFC3339))
		_ = pb.Save(rec)
	}()
}

func unregisterCron(scriptID string) {
	cronMu.Lock()
	defer cronMu.Unlock()
	if existing, ok := cronSchedules[scriptID]; ok {
		close(existing.stopCh)
		delete(cronSchedules, scriptID)
	}
}

func updateNextRunAt(scriptID string, next time.Time) {
	go func() {
		rec, err := pb.FindRecordById("scripts", scriptID)
		if err != nil {
			return
		}
		nextStr := ""
		if !next.IsZero() {
			nextStr = next.UTC().Format(time.RFC3339)
		}
		rec.Set("next_run_at", nextStr)
		_ = pb.Save(rec)
	}()
}

// getCronSchedules returns all active cron schedules.
func getCronSchedules(e *core.RequestEvent) error {
	cronMu.Lock()
	defer cronMu.Unlock()

	type schedEntry struct {
		ScriptID string `json:"script_id"`
		Name     string `json:"name"`
		CronExpr string `json:"cron_expression"`
		Next     string `json:"next_run_at"`
		Prev     string `json:"prev_run_at"`
	}

	entries := make([]schedEntry, 0, len(cronSchedules))
	for _, s := range cronSchedules {
		prevStr := ""
		if !s.PrevRunAt.IsZero() {
			prevStr = s.PrevRunAt.UTC().Format(time.RFC3339)
		}
		nextStr := ""
		if !s.NextRunAt.IsZero() {
			nextStr = s.NextRunAt.UTC().Format(time.RFC3339)
		}
		entries = append(entries, schedEntry{
			ScriptID: s.ScriptID,
			Name:     s.Name,
			CronExpr: s.CronExpr,
			Next:     nextStr,
			Prev:     prevStr,
		})
	}
	return e.JSON(http.StatusOK, map[string]any{"schedules": entries})
}

// runScriptByID fetches and runs a script by its PocketBase ID.
func runScriptByID(id string, timeoutSec int) {
	record, err := pb.FindRecordById("scripts", id)
	if err != nil {
		pb.Logger().Warn("cronmanager: script not found", "id", id)
		return
	}

	code := record.GetString("code")
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	timeout := time.Duration(timeoutSec) * time.Second

	output, runErr, duration := runScript(code, timeout, nil)

	status := "success"
	errMsg := ""
	if runErr != nil {
		status = "error"
		errMsg = runErr.Error()
	}

	record.Set("last_run_status", status)
	record.Set("last_run_output", output)
	record.Set("last_run_duration_ms", float64(duration.Milliseconds()))
	record.Set("last_run_error", errMsg)
	_ = pb.Save(record)
}

// ── cron expression parser ────────────────────────────────────────────────────
// Supports standard 5-field cron: minute hour dom month dow
// Fields may be: * (wildcard), a single integer, or a comma-separated list.
// Step syntax (*/n or start/n) is also supported.

// nextCronTime returns the next time after `from` that matches `expr`.
// Returns zero time if the expression is invalid or no match found within a year.
func nextCronTime(expr string, from time.Time) time.Time {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return time.Time{}
	}
	minField, hourField, domField, monField, dowField := fields[0], fields[1], fields[2], fields[3], fields[4]

	// Start search from the next minute
	t := from.Truncate(time.Minute).Add(time.Minute)

	// Search up to a year ahead to avoid infinite loops on invalid exprs
	limit := t.Add(366 * 24 * time.Hour)
	for t.Before(limit) {
		if !matchField(monField, int(t.Month()), 1, 12) {
			// advance to next month
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}
		if !matchField(domField, t.Day(), 1, 31) {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}
		if !matchField(dowField, int(t.Weekday()), 0, 6) {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}
		if !matchField(hourField, t.Hour(), 0, 23) {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			continue
		}
		if !matchField(minField, t.Minute(), 0, 59) {
			t = t.Add(time.Minute)
			continue
		}
		return t
	}
	return time.Time{}
}

// matchField returns true if val matches the cron field expression.
func matchField(field string, val, min, max int) bool {
	if field == "*" {
		return true
	}
	// Handle step: */n or start/n
	if strings.Contains(field, "/") {
		parts := strings.SplitN(field, "/", 2)
		step, err := strconv.Atoi(parts[1])
		if err != nil || step <= 0 {
			return false
		}
		start := min
		if parts[0] != "*" {
			start, err = strconv.Atoi(parts[0])
			if err != nil {
				return false
			}
		}
		if val < start {
			return false
		}
		return (val-start)%step == 0
	}
	// Handle comma-separated list
	for _, part := range strings.Split(field, ",") {
		// Handle range: a-b
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, err1 := strconv.Atoi(bounds[0])
			hi, err2 := strconv.Atoi(bounds[1])
			if err1 == nil && err2 == nil && val >= lo && val <= hi {
				return true
			}
			continue
		}
		n, err := strconv.Atoi(part)
		if err == nil && n == val {
			return true
		}
	}
	// Handle numeric aliases for months and weekdays are not needed for basic cron
	_ = min
	_ = max
	return false
}
