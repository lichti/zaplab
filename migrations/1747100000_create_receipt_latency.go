package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const idReceiptLatency = "rl1a2b3c4d5e6f7g"

func init() {
	m.Register(upReceiptLatency, downReceiptLatency)
}

func upReceiptLatency(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idReceiptLatency); err == nil {
		return nil
	}
	col := core.NewBaseCollection("receipt_latency", idReceiptLatency)
	col.Fields.Add(
		textField("rl1msgid", "msg_id", true, false),
		textField("rl2chat", "chat_jid", false, false),
		textField("rl3type", "receipt_type", false, false),
		&core.NumberField{Id: "rl4lat", Name: "latency_ms"},
		textField("rl5sent", "sent_at", false, false),
		textField("rl6rcvd", "receipt_at", false, false),
	)
	addAutodateFields(col)
	col.AddIndex("idx_rl_msgid", false, "msg_id", "")
	col.AddIndex("idx_rl_chat", false, "chat_jid", "")
	col.AddIndex("idx_rl_type", false, "receipt_type", "")
	col.AddIndex("idx_rl_created", false, "created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	col.CreateRule = &empty
	return app.Save(col)
}

func downReceiptLatency(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idReceiptLatency)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
