package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/lichti/zaplab/internal/whatsapp"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// getPresenceTimeline returns presence events for a given JID over time.
// Presence events are stored in the events table as type "Presence.Online",
// "Presence.Offline", "Presence.OfflineLastSeen", and "ChatPresence.*".
//
// Query params:
//
//	jid    string  filter by JID (optional; returns all if empty)
//	days   int     look-back period (default 7, 0 = all time)
//	limit  int     max rows (default 200, max 2000)
func getPresenceTimeline(e *core.RequestEvent) error {
	jid := e.Request.URL.Query().Get("jid")
	days, _ := strconv.Atoi(e.Request.URL.Query().Get("days"))
	if days < 0 || days > 365 {
		days = 7
	}
	limit, _ := strconv.Atoi(e.Request.URL.Query().Get("limit"))
	if limit < 1 || limit > 2000 {
		limit = 200
	}

	where := "WHERE (type LIKE 'Presence.%' OR type LIKE 'ChatPresence.%')"
	if days > 0 {
		where += fmt.Sprintf(" AND datetime(created) >= datetime('now', '-%d days')", days)
	}
	if jid != "" {
		where += fmt.Sprintf(" AND (raw LIKE '%%%s%%' OR raw LIKE '%%%s%%')", sanitizeSQL(jid), sanitizeSQL(jid))
	}

	type presRow struct {
		ID      string `db:"id"      json:"id"`
		Type    string `db:"type"    json:"type"`
		Raw     string `db:"raw"     json:"raw"`
		Created string `db:"created" json:"created"`
	}

	sql := fmt.Sprintf(`
        SELECT id, type, raw, created
        FROM events
        %s
        ORDER BY created DESC
        LIMIT %d`, where, limit)

	var rows []presRow
	if err := pb.DB().NewQuery(sql).All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []presRow{}
	}

	// Summary: count online/offline events per JID
	type jidSummary struct {
		JID     string `db:"jid"     json:"jid"`
		Online  int    `db:"online"  json:"online"`
		Offline int    `db:"offline" json:"offline"`
	}
	summaryWhere := where
	sumSQL := fmt.Sprintf(`
        SELECT
          json_extract(raw, '$.From') AS jid,
          SUM(CASE WHEN type = 'Presence.Online' THEN 1 ELSE 0 END) AS online,
          SUM(CASE WHEN type LIKE 'Presence.Offline%%' THEN 1 ELSE 0 END) AS offline
        FROM events
        %s AND type LIKE 'Presence.%%'
        GROUP BY jid
        ORDER BY (online + offline) DESC
        LIMIT 50`, summaryWhere)
	var summary []jidSummary
	_ = pb.DB().NewQuery(sumSQL).All(&summary)
	if summary == nil {
		summary = []jidSummary{}
	}

	return e.JSON(http.StatusOK, map[string]any{
		"events":  rows,
		"summary": summary,
		"days":    days,
		"total":   len(rows),
	})
}

// postSubscribePresence subscribes to presence updates for a given JID.
// Incoming events are handled automatically by the event loop and persisted
// as "Presence.Online", "Presence.Offline", or "Presence.OfflineLastSeen".
//
// Note: WhatsApp only delivers presence for contacts that have you saved.
// Non-contacts are silently ignored server-side.
func postSubscribePresence(e *core.RequestEvent) error {
	var req struct {
		JID string `json:"jid"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.JID == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "jid is required"})
	}
	jid, ok := whatsapp.ParseJID(req.JID)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "jid is not a valid JID"})
	}
	if err := whatsapp.SubscribePresence(jid); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "Failed to subscribe to presence", "error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message": "Subscribed to presence updates",
		"jid":     jid.String(),
	})
}
