package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lichti/zaplab/internal/whatsapp"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// ── Script Triggers (Event Hooks) ─────────────────────────────────────────────
//
// Collection: script_triggers
// Fields: script_id, event_type, jid_filter, text_pattern, enabled

// InitTriggerDispatch wires the trigger dispatch function into the whatsapp package
// to avoid an import cycle (api imports whatsapp, not vice-versa).
func InitTriggerDispatch() {
	whatsapp.TriggerDispatchFunc = dispatchTriggers
}

// dispatchTriggers is called in a goroutine from persist.go after every event save.
// It loads enabled triggers matching evtType and executes their scripts with the
// event payload available as the `event` variable.
func dispatchTriggers(evtType string, rawJSON []byte) {
	type triggerRow struct {
		ID          string  `db:"id"`
		ScriptID    string  `db:"script_id"`
		EventType   string  `db:"event_type"`
		JIDFilter   string  `db:"jid_filter"`
		TextPattern string  `db:"text_pattern"`
		Enabled     bool    `db:"enabled"`
		TimeoutSecs float64 `db:"timeout_secs"`
		Code        string  `db:"code"`
	}
	var triggers []triggerRow
	err := pb.DB().NewQuery(`
		SELECT t.id, t.script_id, t.event_type, t.jid_filter, t.text_pattern, t.enabled,
		       COALESCE(s.timeout_secs, 10) AS timeout_secs, COALESCE(s.code, '') AS code
		FROM script_triggers t
		LEFT JOIN scripts s ON s.id = t.script_id
		WHERE t.enabled = 1 AND t.event_type = {:evt}`).
		Bind(map[string]any{"evt": evtType}).
		All(&triggers)
	if err != nil || len(triggers) == 0 {
		return
	}

	// Parse raw JSON for JID and text extraction
	var rawMap map[string]any
	_ = json.Unmarshal(rawJSON, &rawMap)

	// Extract chat JID for JID filter matching
	chatJID := extractString(rawMap, "Info", "Chat")

	// Extract message text for text_pattern matching
	msgText := extractMessageText(rawMap)

	for _, t := range triggers {
		t := t // capture
		if t.Code == "" {
			continue
		}
		// Apply optional JID filter
		if t.JIDFilter != "" && !strings.Contains(chatJID, t.JIDFilter) {
			continue
		}
		// Apply optional text pattern
		if t.TextPattern != "" && !strings.Contains(strings.ToLower(msgText), strings.ToLower(t.TextPattern)) {
			continue
		}

		timeout := time.Duration(t.TimeoutSecs * float64(time.Second))
		if timeout <= 0 {
			timeout = 10 * time.Second
		}

		// Inject event payload as JS variable
		var eventVal any
		_ = json.Unmarshal(rawJSON, &eventVal)
		env := map[string]any{"event": eventVal}

		go func() {
			defer func() { recover() }() //nolint:errcheck
			runScript(t.Code, timeout, env)
		}()
	}
}

// extractString walks a map[string]any by keys and returns the string leaf, or "".
func extractString(m map[string]any, keys ...string) string {
	cur := m
	for i, k := range keys {
		v, ok := cur[k]
		if !ok {
			return ""
		}
		if i == len(keys)-1 {
			s, _ := v.(string)
			return s
		}
		cur, ok = v.(map[string]any)
		if !ok {
			return ""
		}
	}
	return ""
}

// extractMessageText tries common message text fields from the raw event payload.
func extractMessageText(m map[string]any) string {
	msg, ok := m["Message"].(map[string]any)
	if !ok {
		return ""
	}
	if s := extractString(msg, "conversation"); s != "" {
		return s
	}
	if s := extractString(msg, "extendedTextMessage", "text"); s != "" {
		return s
	}
	if s := extractString(msg, "imageMessage", "caption"); s != "" {
		return s
	}
	if s := extractString(msg, "videoMessage", "caption"); s != "" {
		return s
	}
	return ""
}

// ── CRUD handlers ─────────────────────────────────────────────────────────────

func getTriggers(e *core.RequestEvent) error {
	type triggerRow struct {
		ID          string `db:"id"           json:"id"`
		ScriptID    string `db:"script_id"    json:"script_id"`
		EventType   string `db:"event_type"   json:"event_type"`
		JIDFilter   string `db:"jid_filter"   json:"jid_filter"`
		TextPattern string `db:"text_pattern" json:"text_pattern"`
		Enabled     bool   `db:"enabled"      json:"enabled"`
		Created     string `db:"created"      json:"created"`
		Updated     string `db:"updated"      json:"updated"`
		ScriptName  string `db:"script_name"  json:"script_name"`
	}
	var rows []triggerRow
	err := pb.DB().NewQuery(`
		SELECT t.id, t.script_id, t.event_type, t.jid_filter, t.text_pattern, t.enabled,
		       t.created, t.updated, COALESCE(s.name, '') AS script_name
		FROM script_triggers t
		LEFT JOIN scripts s ON s.id = t.script_id
		ORDER BY t.event_type ASC, t.created ASC`).
		All(&rows)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []triggerRow{}
	}
	return e.JSON(http.StatusOK, map[string]any{"triggers": rows, "total": len(rows)})
}

func postTrigger(e *core.RequestEvent) error {
	var req struct {
		ScriptID    string `json:"script_id"`
		EventType   string `json:"event_type"`
		JIDFilter   string `json:"jid_filter"`
		TextPattern string `json:"text_pattern"`
		Enabled     bool   `json:"enabled"`
	}
	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return apis.NewBadRequestError("invalid body", err)
	}
	if req.ScriptID == "" || req.EventType == "" {
		return apis.NewBadRequestError("script_id and event_type are required", nil)
	}
	col, err := pb.FindCollectionByNameOrId("script_triggers")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": "script_triggers collection not found"})
	}
	record := core.NewRecord(col)
	record.Set("script_id", req.ScriptID)
	record.Set("event_type", req.EventType)
	record.Set("jid_filter", req.JIDFilter)
	record.Set("text_pattern", req.TextPattern)
	record.Set("enabled", req.Enabled)
	if err := pb.Save(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, triggerRecordToMap(record, ""))
}

func patchTrigger(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	record, err := pb.FindRecordById("script_triggers", id)
	if err != nil {
		return apis.NewNotFoundError("trigger not found", nil)
	}
	var req struct {
		ScriptID    *string `json:"script_id"`
		EventType   *string `json:"event_type"`
		JIDFilter   *string `json:"jid_filter"`
		TextPattern *string `json:"text_pattern"`
		Enabled     *bool   `json:"enabled"`
	}
	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return apis.NewBadRequestError("invalid body", err)
	}
	if req.ScriptID != nil {
		record.Set("script_id", *req.ScriptID)
	}
	if req.EventType != nil {
		record.Set("event_type", *req.EventType)
	}
	if req.JIDFilter != nil {
		record.Set("jid_filter", *req.JIDFilter)
	}
	if req.TextPattern != nil {
		record.Set("text_pattern", *req.TextPattern)
	}
	if req.Enabled != nil {
		record.Set("enabled", *req.Enabled)
	}
	if err := pb.Save(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, triggerRecordToMap(record, ""))
}

func deleteTrigger(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	record, err := pb.FindRecordById("script_triggers", id)
	if err != nil {
		return apis.NewNotFoundError("trigger not found", nil)
	}
	if err := pb.Delete(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "trigger deleted"})
}

func triggerRecordToMap(r *core.Record, scriptName string) map[string]any {
	return map[string]any{
		"id":           r.Id,
		"script_id":    r.GetString("script_id"),
		"event_type":   r.GetString("event_type"),
		"jid_filter":   r.GetString("jid_filter"),
		"text_pattern": r.GetString("text_pattern"),
		"enabled":      r.GetBool("enabled"),
		"script_name":  scriptName,
		"created":      r.GetDateTime("created").String(),
		"updated":      r.GetDateTime("updated").String(),
	}
}

// getScriptEventTypes returns common event type values for the trigger form dropdown.
func getScriptEventTypes(e *core.RequestEvent) error {
	type typeRow struct {
		Type  string `db:"type"  json:"type"`
		Count int    `db:"cnt"   json:"count"`
	}
	var rows []typeRow
	_ = pb.DB().Select("type", "COUNT(*) AS cnt").
		From("events").
		GroupBy("type").
		OrderBy("cnt DESC").
		Limit(30).
		All(&rows)
	if rows == nil {
		rows = []typeRow{}
	}
	// Prepend canonical types that may not have events yet
	canonical := []string{"Message", "SentMessage", "Receipt", "Presence", "HistorySync",
		"Connected", "Disconnected", "LoggedOut", "PairSuccess"}
	seen := make(map[string]bool, len(rows))
	for _, r := range rows {
		seen[r.Type] = true
	}
	var extra []typeRow
	for _, t := range canonical {
		if !seen[t] {
			extra = append(extra, typeRow{Type: t, Count: 0})
		}
	}
	return e.JSON(http.StatusOK, map[string]any{"types": append(extra, rows...)})
}

// Ensure fmt is used
var _ = fmt.Sprintf
