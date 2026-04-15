package whatsapp

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
)

// CreateNotification persists a notification record and broadcasts it over SSE.
// notifType must be one of: "mention", "tracker_state", "webhook_failure".
// entityID and entityJID are optional references to the originating record/JID.
// data is arbitrary payload stored as JSON.
func CreateNotification(notifType, title, body, entityID, entityJID string, data any) {
	if shuttingDown.Load() || pb == nil || pb.DB() == nil {
		return
	}
	col, err := pb.FindCollectionByNameOrId("notifications")
	if err != nil {
		return
	}

	raw, _ := json.Marshal(data)

	r := core.NewRecord(col)
	r.Set("type", notifType)
	r.Set("title", title)
	r.Set("body", body)
	r.Set("entity_id", entityID)
	r.Set("entity_jid", entityJID)
	r.Set("read_at", "")
	r.Set("data", raw)
	if err := pb.Save(r); err != nil && !shuttingDown.Load() {
		logger.Warnf("CreateNotification: save failed: %v", err)
		return
	}

	ssePublish("Notification", map[string]any{
		"id":         r.Id,
		"type":       notifType,
		"title":      title,
		"body":       body,
		"entity_id":  entityID,
		"entity_jid": entityJID,
		"data":       data,
		"created":    r.GetDateTime("created").String(),
	})
}
