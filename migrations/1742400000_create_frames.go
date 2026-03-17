package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const idFrames = "pn3d8q1a7fxcm2v"

func init() {
	m.Register(upFrames, downFrames)
}

func upFrames(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idFrames); err == nil {
		return nil // already exists
	}
	col := core.NewBaseCollection("frames", idFrames)
	col.Fields.Add(
		textField("fr1module", "module", false, false),
		textField("fr2level", "level", false, false),
		textField("fr3seq", "seq", false, false),
		textField("fr4msg", "msg", false, false),
	)
	addAutodateFields(col)
	col.AddIndex("idx_frames_module", false, "module", "")
	col.AddIndex("idx_frames_level", false, "level", "")
	col.AddIndex("idx_frames_created", false, "created", "")
	col.AddIndex("idx_frames_level_created", false, "level,created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	return app.Save(col)
}

func downFrames(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idFrames)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
