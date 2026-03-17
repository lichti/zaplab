package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const idAnnotations = "an7k2p9x3qrbm5w"

func init() {
	m.Register(upAnnotations, downAnnotations)
}

func upAnnotations(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idAnnotations); err == nil {
		return nil // already exists
	}
	col := core.NewBaseCollection("annotations", idAnnotations)
	col.Fields.Add(
		textField("an1evtid", "event_id", false, false),
		textField("an2evttype", "event_type", false, false),
		textField("an3jid", "jid", false, false),
		textField("an4note", "note", false, false),
		jsonField("an5tags", "tags", 4096),
	)
	addAutodateFields(col)
	col.AddIndex("idx_annotations_event_id", false, "event_id", "")
	col.AddIndex("idx_annotations_jid", false, "jid", "")
	col.AddIndex("idx_annotations_created", false, "created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	col.CreateRule = &empty
	col.UpdateRule = &empty
	col.DeleteRule = &empty
	return app.Save(col)
}

func downAnnotations(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idAnnotations)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
