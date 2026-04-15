package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const idScheduledMessages = "scm1a2b3c4d5e6f7"

func init() {
	m.Register(upScheduledMessages, downScheduledMessages)
}

func upScheduledMessages(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idScheduledMessages); err == nil {
		return nil
	}
	col := core.NewBaseCollection("scheduled_messages", idScheduledMessages)
	col.Fields.Add(
		textField("scm1jid", "chat_jid", true, false),
		textField("scm2text", "message_text", false, false),
		// msg_type: text | image | video | audio | document
		textField("scm3type", "msg_type", false, false),
		textField("scm4at", "scheduled_at", true, false), // ISO-8601 UTC
		textField("scm5sent", "sent_at", false, false),
		// status: pending | sent | failed | cancelled
		textField("scm6status", "status", false, false),
		textField("scm7err", "error", false, false),
		textField("scm8replyid", "reply_to_msg_id", false, false),
		textField("scm9media", "media_url", false, false),
	)
	addAutodateFields(col)
	col.AddIndex("idx_scm_status", false, "status", "")
	col.AddIndex("idx_scm_scheduled", false, "scheduled_at", "")
	col.AddIndex("idx_scm_chat", false, "chat_jid", "")
	col.AddIndex("idx_scm_created", false, "created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	col.CreateRule = &empty
	col.UpdateRule = &empty
	col.DeleteRule = &empty
	return app.Save(col)
}

func downScheduledMessages(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idScheduledMessages)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
