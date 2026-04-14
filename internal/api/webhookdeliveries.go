package api

import (
	"net/http"
	"strconv"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// getWebhookDeliveries returns the webhook delivery log.
// Query params: status (delivered|failed), url (partial match), limit (default 100, max 500)
func getWebhookDeliveries(e *core.RequestEvent) error {
	limit := 100
	if v := e.Request.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	q := pb.DB().
		Select("id", "webhook_url", "event_type", "status", "attempt", "http_status", "error_msg", "delivered_at", "created").
		From("webhook_deliveries").
		OrderBy("created DESC").
		Limit(int64(limit))

	if status := e.Request.URL.Query().Get("status"); status != "" {
		q = q.AndWhere(dbx.HashExp{"status": status})
	}
	if urlFilter := e.Request.URL.Query().Get("url"); urlFilter != "" {
		q = q.AndWhere(dbx.NewExp("webhook_url LIKE {:u}", dbx.Params{"u": "%" + urlFilter + "%"}))
	}

	type row struct {
		ID          string  `db:"id"          json:"id"`
		WebhookURL  string  `db:"webhook_url"  json:"webhook_url"`
		EventType   string  `db:"event_type"   json:"event_type"`
		Status      string  `db:"status"       json:"status"`
		Attempt     float64 `db:"attempt"      json:"attempt"`
		HTTPStatus  float64 `db:"http_status"  json:"http_status"`
		ErrorMsg    string  `db:"error_msg"    json:"error_msg"`
		DeliveredAt string  `db:"delivered_at" json:"delivered_at"`
		Created     string  `db:"created"      json:"created"`
	}
	var rows []row
	if err := q.All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []row{}
	}
	return e.JSON(http.StatusOK, map[string]any{"deliveries": rows, "total": len(rows)})
}

// deleteWebhookDeliveries purges delivery log records older than N days.
// Query param: days (default 7)
func deleteWebhookDeliveries(e *core.RequestEvent) error {
	days := 7
	if v := e.Request.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}
	res, err := pb.DB().NewQuery(
		"DELETE FROM webhook_deliveries WHERE created < datetime('now', {:d})",
	).Bind(dbx.Params{"d": "-" + strconv.Itoa(days) + " days"}).Execute()
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	affected, _ := res.RowsAffected()
	return e.JSON(http.StatusOK, map[string]any{"deleted": affected})
}
