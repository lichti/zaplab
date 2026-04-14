package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const idMessageReactions = "mre1a2b3c4d5e6f7"

func init() {
	m.Register(upMessageReactions, downMessageReactions)
}

func upMessageReactions(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idMessageReactions); err == nil {
		return nil
	}
	col := core.NewBaseCollection("message_reactions", idMessageReactions)
	col.Fields.Add(
		textField("mre1msgid", "message_id", true, false),  // target message that was reacted to
		textField("mre2chat", "chat_jid", true, false),
		textField("mre3sender", "sender_jid", true, false),
		textField("mre4emoji", "emoji", false, false),
		&core.BoolField{Id: "mre5removed", Name: "removed"},
		textField("mre6ts", "react_at", false, false),
	)
	addAutodateFields(col)
	col.AddIndex("idx_mre_message", false, "message_id", "")
	col.AddIndex("idx_mre_chat", false, "chat_jid", "")
	col.AddIndex("idx_mre_sender", false, "sender_jid", "")
	col.AddIndex("idx_mre_created", false, "created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	return app.Save(col)
}

func downMessageReactions(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idMessageReactions)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
