package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const idConnEvents = "ce1a2b3c4d5e6f7g"

func init() {
	m.Register(upConnEvents, downConnEvents)
}

func upConnEvents(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idConnEvents); err == nil {
		return nil
	}
	col := core.NewBaseCollection("conn_events", idConnEvents)
	col.Fields.Add(
		textField("ce1type", "event_type", true, false),
		textField("ce2reason", "reason", false, false),
		textField("ce3jid", "jid", false, false),
	)
	addAutodateFields(col)
	col.AddIndex("idx_ce_type", false, "event_type", "")
	col.AddIndex("idx_ce_created", false, "created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	col.CreateRule = &empty
	return app.Save(col)
}

func downConnEvents(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idConnEvents)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
