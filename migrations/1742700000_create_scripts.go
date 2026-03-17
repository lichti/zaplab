package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const idScripts = "sc9p4r2k7mxnb1q"

func init() {
	m.Register(upScripts, downScripts)
}

func upScripts(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idScripts); err == nil {
		return nil // already exists
	}
	col := core.NewBaseCollection("scripts", idScripts)
	col.Fields.Add(
		textField("sc1name", "name", true, false),
		textField("sc2desc", "description", false, false),
		textField("sc3code", "code", false, false),
		&core.BoolField{Id: "sc4enabled", Name: "enabled"},
		&core.NumberField{Id: "sc5timeout", Name: "timeout_secs"},
		textField("sc6status", "last_run_status", false, false),
		textField("sc7output", "last_run_output", false, false),
		&core.NumberField{Id: "sc8dur", Name: "last_run_duration_ms"},
		textField("sc9error", "last_run_error", false, false),
	)
	addAutodateFields(col)
	col.AddIndex("idx_scripts_name", true, "name", "")
	col.AddIndex("idx_scripts_created", false, "created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	col.CreateRule = &empty
	col.UpdateRule = &empty
	col.DeleteRule = &empty
	return app.Save(col)
}

func downScripts(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idScripts)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
