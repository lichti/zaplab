package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const idAuditLog = "au1a2b3c4d5e6f7g"

func init() {
	m.Register(upAuditLog, downAuditLog)
}

func upAuditLog(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idAuditLog); err == nil {
		return nil
	}
	col := core.NewBaseCollection("audit_log", idAuditLog)
	col.Fields.Add(
		textField("au1method", "method", false, false),
		textField("au2path", "path", false, false),
		textField("au3status", "status_code", false, false),
		textField("au4ip", "remote_ip", false, false),
		textField("au5body", "request_body", false, false),
	)
	addAutodateFields(col)
	col.AddIndex("idx_al_method", false, "method", "")
	col.AddIndex("idx_al_path", false, "path", "")
	col.AddIndex("idx_al_created", false, "created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	col.CreateRule = &empty
	return app.Save(col)
}

func downAuditLog(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idAuditLog)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
