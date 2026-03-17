package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/pocketbase/pocketbase/core"
)

// ── Stats API ─────────────────────────────────────────────────────────────────
//
// All endpoints run raw SQL against PocketBase's own SQLite (pb.DB()) so they
// can use SQLite date functions not available in the PocketBase filter DSL.
// All require auth.

// getStatsHeatmap returns message-event counts grouped by (day-of-week, hour)
// for the requested period.  Used to render the activity heatmap.
//
// Query params:
//
//	period  int  days to look back (default 30, max 365; 0 = all time)
//	filter  string  "messages" (default) | "all" | "errors"
func getStatsHeatmap(e *core.RequestEvent) error {
	period, _ := strconv.Atoi(e.Request.URL.Query().Get("period"))
	if period < 0 || period > 365 {
		period = 30
	}

	type heatCell struct {
		DOW   string `db:"dow"   json:"dow"`
		Hour  string `db:"hour"  json:"hour"`
		Count int    `db:"cnt"   json:"count"`
	}

	periodClause := ""
	if period > 0 {
		periodClause = fmt.Sprintf("AND datetime(created) >= datetime('now', '-%d days')", period)
	}

	sql := fmt.Sprintf(`
		SELECT strftime('%%w', datetime(created)) AS dow,
		       strftime('%%H', datetime(created)) AS hour,
		       COUNT(*) AS cnt
		FROM events
		WHERE type LIKE '%%Message%%' %s
		GROUP BY dow, hour
		ORDER BY dow, hour`, periodClause)

	var cells []heatCell
	if err := pb.DB().NewQuery(sql).All(&cells); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if cells == nil {
		cells = []heatCell{}
	}
	return e.JSON(http.StatusOK, map[string]any{"cells": cells, "period": period})
}

// getStatsDaily returns daily message counts for the last N days.
// Includes the filter string (messages / all) for flexibility.
//
// Query params:
//
//	days   int    days to look back (default 30, max 365)
func getStatsDaily(e *core.RequestEvent) error {
	days, _ := strconv.Atoi(e.Request.URL.Query().Get("days"))
	if days < 1 || days > 365 {
		days = 30
	}

	type dayRow struct {
		Day   string `db:"day"  json:"day"`
		Count int    `db:"cnt"  json:"count"`
	}

	sql := fmt.Sprintf(`
		SELECT strftime('%%Y-%%m-%%d', datetime(created)) AS day,
		       COUNT(*) AS cnt
		FROM events
		WHERE type LIKE '%%Message%%'
		  AND datetime(created) >= datetime('now', '-%d days')
		GROUP BY day
		ORDER BY day ASC`, days)

	var rows []dayRow
	if err := pb.DB().NewQuery(sql).All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []dayRow{}
	}
	return e.JSON(http.StatusOK, map[string]any{"days": rows, "period": days})
}

// getStatsTypes returns event counts grouped by event type.
//
// Query params:
//
//	period  int  days to look back (default 30, max 365; 0 = all time)
//	limit   int  max types to return (default 20, max 50)
func getStatsTypes(e *core.RequestEvent) error {
	period, _ := strconv.Atoi(e.Request.URL.Query().Get("period"))
	if period < 0 || period > 365 {
		period = 30
	}
	limit, _ := strconv.Atoi(e.Request.URL.Query().Get("limit"))
	if limit < 1 || limit > 50 {
		limit = 20
	}

	type typeRow struct {
		Type  string `db:"type"  json:"type"`
		Count int    `db:"cnt"   json:"count"`
	}

	periodClause := ""
	if period > 0 {
		periodClause = fmt.Sprintf("WHERE datetime(created) >= datetime('now', '-%d days')", period)
	}

	sql := fmt.Sprintf(`
		SELECT type, COUNT(*) AS cnt
		FROM events
		%s
		GROUP BY type
		ORDER BY cnt DESC
		LIMIT %d`, periodClause, limit)

	var rows []typeRow
	if err := pb.DB().NewQuery(sql).All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []typeRow{}
	}
	return e.JSON(http.StatusOK, map[string]any{"types": rows, "period": period})
}

// getStatsSummary returns aggregate message statistics across multiple time windows.
func getStatsSummary(e *core.RequestEvent) error {
	type summaryRow struct {
		Total   int    `db:"total"    json:"total"`
		Last24h int    `db:"last24h"  json:"last_24h"`
		Last7d  int    `db:"last7d"   json:"last_7d"`
		Last30d int    `db:"last30d"  json:"last_30d"`
		LastEvt string `db:"last_evt" json:"last_event"`
	}

	sql := `
		SELECT
		  COUNT(*)                                                                              AS total,
		  SUM(CASE WHEN datetime(created) >= datetime('now', '-1 day')   THEN 1 ELSE 0 END)    AS last24h,
		  SUM(CASE WHEN datetime(created) >= datetime('now', '-7 days')  THEN 1 ELSE 0 END)    AS last7d,
		  SUM(CASE WHEN datetime(created) >= datetime('now', '-30 days') THEN 1 ELSE 0 END)    AS last30d,
		  COALESCE(MAX(datetime(created)), '')                                                  AS last_evt
		FROM events
		WHERE type LIKE '%Message%'`

	var row summaryRow
	if err := pb.DB().NewQuery(sql).One(&row); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}

	// Also fetch edit/delete counts (all time)
	type editRow struct {
		Edited  int `db:"edited"  json:"edited"`
		Deleted int `db:"deleted" json:"deleted"`
	}
	editSQL := `
		SELECT
		  SUM(CASE WHEN raw LIKE '%"Edit":"1"%' THEN 1 ELSE 0 END) AS edited,
		  SUM(CASE WHEN raw LIKE '%"Edit":"7"%' OR raw LIKE '%"Edit":"8"%' THEN 1 ELSE 0 END) AS deleted
		FROM events
		WHERE type = 'Message'`
	var er editRow
	_ = pb.DB().NewQuery(editSQL).One(&er)

	return e.JSON(http.StatusOK, map[string]any{
		"total":      row.Total,
		"last_24h":   row.Last24h,
		"last_7d":    row.Last7d,
		"last_30d":   row.Last30d,
		"last_event": row.LastEvt,
		"edited":     er.Edited,
		"deleted":    er.Deleted,
	})
}

// getStatsEditChain returns all versions of a message (identified by its msgID):
// the original event plus all subsequent edit events, sorted chronologically.
// Used to render the edit timeline in the Message Diff view.
//
// Query param: msgid (required)
func getStatsEditChain(e *core.RequestEvent) error {
	msgID := e.Request.URL.Query().Get("msgid")
	if msgID == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "msgid is required"})
	}

	type chainRow struct {
		ID      string `db:"id"      json:"id"`
		Type    string `db:"type"    json:"type"`
		MsgID   string `db:"msgID"   json:"msgID"`
		Raw     string `db:"raw"     json:"raw"`
		Created string `db:"created" json:"created"`
	}

	// Collect: (a) original message with this msgID, (b) edit/delete events targeting it
	sql := fmt.Sprintf(`
		SELECT id, type, msgID, raw, created
		FROM events
		WHERE msgID = '%s'
		ORDER BY created ASC
		LIMIT 50`, sanitizeSQL(msgID))

	var rows []chainRow
	if err := pb.DB().NewQuery(sql).All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []chainRow{}
	}
	return e.JSON(http.StatusOK, map[string]any{"chain": rows, "msgid": msgID, "total": len(rows)})
}

// sanitizeSQL replaces single quotes in user input to prevent SQL injection
// in the basic string interpolation used above.
func sanitizeSQL(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'', '\'') // SQL escape: '' for a literal '
		} else {
			out = append(out, s[i])
		}
	}
	return string(out)
}
