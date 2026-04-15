package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const idNotifications = "ntf1a2b3c4d5e6f7"

func init() {
	m.Register(upNotifications, downNotifications)
}

func upNotifications(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idNotifications); err == nil {
		return nil
	}
	col := core.NewBaseCollection("notifications", idNotifications)
	col.Fields.Add(
		textField("ntf1type", "type", true, false),          // "mention" | "tracker_state" | "webhook_failure"
		textField("ntf2title", "title", true, false),        // short human-readable label
		textField("ntf3body", "body", false, false),         // detail description
		textField("ntf4entid", "entity_id", false, false),   // ID of the related record
		textField("ntf5entjid", "entity_jid", false, false), // related JID when applicable
		textField("ntf6readat", "read_at", false, false),    // ISO timestamp; empty = unread
		jsonField("ntf7data", "data", 8192),                 // raw payload for context
	)
	addAutodateFields(col)
	col.AddIndex("idx_ntf_type", false, "type", "")
	col.AddIndex("idx_ntf_readat", false, "read_at", "")
	col.AddIndex("idx_ntf_created", false, "created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	col.UpdateRule = &empty
	col.DeleteRule = &empty
	return app.Save(col)
}

func downNotifications(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idNotifications)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
