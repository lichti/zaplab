package api

import (
	"net/http"
	"strconv"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// getConnEvents returns connection/disconnection events.
// GET /zaplab/api/conn/events?days=7&limit=200
func getConnEvents(e *core.RequestEvent) error {
	days := 7
	limit := 200
	if v := e.Request.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}
	if v := e.Request.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	type connRow struct {
		ID        string `db:"id"         json:"id"`
		EventType string `db:"event_type" json:"event_type"`
		Reason    string `db:"reason"     json:"reason"`
		JID       string `db:"jid"        json:"jid"`
		Created   string `db:"created"    json:"created"`
	}

	var rows []connRow
	err := pb.DB().Select("id", "event_type", "reason", "jid", "created").
		From("conn_events").
		Where(dbx.NewExp("created >= datetime('now', {:d})", dbx.Params{"d": "-" + strconv.Itoa(days) + " days"})).
		OrderBy("created DESC").
		Limit(int64(limit)).
		All(&rows)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []connRow{}
	}
	return e.JSON(http.StatusOK, map[string]any{"events": rows, "total": len(rows)})
}

// getConnStats returns aggregated connection stability statistics.
// GET /zaplab/api/conn/stats?days=7
func getConnStats(e *core.RequestEvent) error {
	days := 7
	if v := e.Request.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}

	daysStr := strconv.Itoa(days)

	type countRow struct {
		EventType string `db:"event_type" json:"event_type"`
		Count     int    `db:"cnt"        json:"count"`
	}
	var counts []countRow
	_ = pb.DB().NewQuery(`
		SELECT event_type, COUNT(*) as cnt
		FROM conn_events
		WHERE created >= datetime('now', '-` + daysStr + ` days')
		GROUP BY event_type
		ORDER BY cnt DESC
	`).All(&counts)

	type dailyRow struct {
		Date  string `db:"day" json:"date"`
		Count int    `db:"cnt" json:"count"`
	}
	var daily []dailyRow
	_ = pb.DB().NewQuery(`
		SELECT strftime('%Y-%m-%d', created) as day, COUNT(*) as cnt
		FROM conn_events
		WHERE created >= datetime('now', '-` + daysStr + ` days')
		  AND event_type = 'disconnected'
		GROUP BY day
		ORDER BY day ASC
	`).All(&daily)

	if counts == nil {
		counts = []countRow{}
	}
	if daily == nil {
		daily = []dailyRow{}
	}
	return e.JSON(http.StatusOK, map[string]any{
		"by_type":           counts,
		"daily_disconnects": daily,
		"days":              days,
	})
}
