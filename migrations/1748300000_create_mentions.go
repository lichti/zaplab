package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const idMentions = "men1a2b3c4d5e6f7"

func init() {
	m.Register(upMentions, downMentions)
}

func upMentions(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idMentions); err == nil {
		return nil
	}
	col := core.NewBaseCollection("mentions", idMentions)
	col.Fields.Add(
		textField("men1msgid", "message_id", true, false),
		textField("men2chat", "chat_jid", true, false),
		textField("men3sender", "sender_jid", true, false),
		textField("men4mentioned", "mentioned_jid", true, false),
		&core.BoolField{Id: "men5isbot", Name: "is_bot"},
		textField("men6ctx", "context_text", false, false),
	)
	addAutodateFields(col)
	col.AddIndex("idx_men_mentioned", false, "mentioned_jid", "")
	col.AddIndex("idx_men_chat", false, "chat_jid", "")
	col.AddIndex("idx_men_isbot", false, "is_bot", "")
	col.AddIndex("idx_men_created", false, "created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	return app.Save(col)
}

func downMentions(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idMentions)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
