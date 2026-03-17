package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const idScriptTriggers = "tr1g9x0k2mzpw5q"

func init() {
	m.Register(upScriptTriggers, downScriptTriggers)
}

func upScriptTriggers(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idScriptTriggers); err == nil {
		return nil // already exists
	}
	col := core.NewBaseCollection("script_triggers", idScriptTriggers)
	col.Fields.Add(
		textField("tr1scriptid", "script_id", true, false),
		textField("tr2evttype", "event_type", true, false),
		textField("tr3jidfilter", "jid_filter", false, false),
		textField("tr4txtpat", "text_pattern", false, false),
		&core.BoolField{Id: "tr5enabled", Name: "enabled"},
	)
	addAutodateFields(col)
	col.AddIndex("idx_st_event_type", false, "event_type", "")
	col.AddIndex("idx_st_enabled", false, "enabled", "")
	col.AddIndex("idx_st_created", false, "created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	col.CreateRule = &empty
	col.UpdateRule = &empty
	col.DeleteRule = &empty
	return app.Save(col)
}

func downScriptTriggers(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idScriptTriggers)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
