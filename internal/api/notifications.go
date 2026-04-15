package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// getNotifications returns a paginated, filterable list of notifications.
// Query params:
//
//	status  = "unread" | "read" | "all" (default "all")
//	type    = notification type filter (optional)
//	limit   = 1–500 (default 100)
//	offset  = (default 0)
func getNotifications(e *core.RequestEvent) error {
	status := e.Request.URL.Query().Get("status") // "unread" | "read" | "all"
	typeFilter := e.Request.URL.Query().Get("type")

	limit := 100
	if n, err := strconv.Atoi(e.Request.URL.Query().Get("limit")); err == nil && n > 0 && n <= 500 {
		limit = n
	}
	offset := 0
	if n, err := strconv.Atoi(e.Request.URL.Query().Get("offset")); err == nil && n >= 0 {
		offset = n
	}

	type row struct {
		ID        string `db:"id"         json:"id"`
		Type      string `db:"type"       json:"type"`
		Title     string `db:"title"      json:"title"`
		Body      string `db:"body"       json:"body"`
		EntityID  string `db:"entity_id"  json:"entity_id"`
		EntityJID string `db:"entity_jid" json:"entity_jid"`
		ReadAt    string `db:"read_at"    json:"read_at"`
		Created   string `db:"created"    json:"created"`
	}

	q := pb.DB().
		Select("id", "type", "title", "body", "entity_id", "entity_jid", "read_at", "created").
		From("notifications").
		OrderBy("created DESC").
		Limit(int64(limit)).
		Offset(int64(offset))

	switch status {
	case "unread":
		q = q.AndWhere(dbx.Or(dbx.HashExp{"read_at": ""}, dbx.HashExp{"read_at": nil}))
	case "read":
		q = q.AndWhere(dbx.Not(dbx.Or(dbx.HashExp{"read_at": ""}, dbx.HashExp{"read_at": nil})))
	}
	if typeFilter != "" {
		q = q.AndWhere(dbx.HashExp{"type": typeFilter})
	}

	var rows []row
	if err := q.All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []row{}
	}

	// Unread count (always returned regardless of status filter)
	var unread int
	_ = pb.DB().NewQuery("SELECT COUNT(*) FROM notifications WHERE read_at IS NULL OR read_at = ''").Row(&unread)

	return e.JSON(http.StatusOK, map[string]any{
		"notifications": rows,
		"total":         len(rows),
		"unread_count":  unread,
	})
}

// putNotificationRead marks a single notification as read.
func putNotificationRead(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if id == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "missing id"})
	}
	rec, err := pb.FindRecordById("notifications", id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	rec.Set("read_at", time.Now().UTC().Format(time.RFC3339))
	if err := pb.Save(rec); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"ok": true})
}

// postNotificationsReadAll marks all unread notifications as read.
func postNotificationsReadAll(e *core.RequestEvent) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := pb.DB().NewQuery(
		"UPDATE notifications SET read_at = {:now}, updated = {:now} WHERE read_at IS NULL OR read_at = ''",
	).Bind(dbx.Params{"now": now}).Execute()
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"ok": true})
}

// deleteNotification removes a single notification.
func deleteNotification(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if id == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "missing id"})
	}
	rec, err := pb.FindRecordById("notifications", id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	if err := pb.Delete(rec); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"ok": true})
}

// postNotificationsPurge deletes all read notifications.
func postNotificationsPurge(e *core.RequestEvent) error {
	_, err := pb.DB().NewQuery(
		"DELETE FROM notifications WHERE read_at IS NOT NULL AND read_at != ''",
	).Execute()
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"ok": true})
}
