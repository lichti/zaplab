package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/pocketbase/pocketbase/core"
)

// getReceiptLatency returns receipt latency stats.
// Query params:
//
//	jid    string  filter by chat JID (optional)
//	type   string  "delivered" | "read" (optional)
//	days   int     days to look back (default 30, 0 = all time)
//	limit  int     max rows (default 500, max 5000)
func getReceiptLatency(e *core.RequestEvent) error {
	jid := e.Request.URL.Query().Get("jid")
	rtype := e.Request.URL.Query().Get("type")
	days, _ := strconv.Atoi(e.Request.URL.Query().Get("days"))
	if days < 0 || days > 365 {
		days = 30
	}
	limit, _ := strconv.Atoi(e.Request.URL.Query().Get("limit"))
	if limit < 1 || limit > 5000 {
		limit = 500
	}

	where := "WHERE 1=1"
	if days > 0 {
		where += fmt.Sprintf(" AND datetime(created) >= datetime('now', '-%d days')", days)
	}
	if jid != "" {
		where += fmt.Sprintf(" AND chat_jid = '%s'", sanitizeSQL(jid))
	}
	if rtype != "" {
		where += fmt.Sprintf(" AND receipt_type = '%s'", sanitizeSQL(rtype))
	}

	type latRow struct {
		MsgID       string `db:"msg_id"       json:"msg_id"`
		ChatJID     string `db:"chat_jid"     json:"chat_jid"`
		ReceiptType string `db:"receipt_type" json:"receipt_type"`
		LatencyMs   int64  `db:"latency_ms"   json:"latency_ms"`
		SentAt      string `db:"sent_at"      json:"sent_at"`
		ReceiptAt   string `db:"receipt_at"   json:"receipt_at"`
		Created     string `db:"created"      json:"created"`
	}

	sql := fmt.Sprintf(`
        SELECT msg_id, chat_jid, receipt_type, latency_ms, sent_at, receipt_at, created
        FROM receipt_latency
        %s
        ORDER BY created DESC
        LIMIT %d`, where, limit)

	var rows []latRow
	if err := pb.DB().NewQuery(sql).All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []latRow{}
	}

	// Summary stats
	type statsRow struct {
		AvgMs float64 `db:"avg_ms"  json:"avg_ms"`
		MinMs int64   `db:"min_ms"  json:"min_ms"`
		MaxMs int64   `db:"max_ms"  json:"max_ms"`
		Count int     `db:"cnt"     json:"count"`
		P50   float64 `db:"p50"     json:"p50"`
	}
	statSQL := fmt.Sprintf(`
        SELECT
          AVG(latency_ms)    AS avg_ms,
          MIN(latency_ms)    AS min_ms,
          MAX(latency_ms)    AS max_ms,
          COUNT(*)           AS cnt,
          0.0                AS p50
        FROM receipt_latency
        %s AND latency_ms > 0`, where)
	var stats statsRow
	_ = pb.DB().NewQuery(statSQL).One(&stats)

	return e.JSON(http.StatusOK, map[string]any{
		"rows":  rows,
		"stats": stats,
		"days":  days,
	})
}
