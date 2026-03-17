package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(upScriptCron, downScriptCron)
}

func upScriptCron(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idScripts)
	if err != nil {
		return nil // scripts collection doesn't exist yet
	}
	if col.Fields.GetByName("cron_expression") != nil {
		return nil // already migrated
	}
	col.Fields.Add(
		textField("sc10cron", "cron_expression", false, false),
		textField("sc11next", "next_run_at", false, false),
	)
	return app.Save(col)
}

func downScriptCron(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idScripts)
	if err != nil {
		return nil
	}
	col.Fields.RemoveById("sc10cron")
	col.Fields.RemoveById("sc11next")
	return app.Save(col)
}
