package migrations

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(upPatchRankingScript, nil)
}

// upPatchRankingScript fixes the existing group-ranking script that was seeded
// without an IIFE wrapper, causing "Illegal return statement" errors in goja.
// It also replaces the code with the current version from rankingScript.
func upPatchRankingScript(app core.App) error {
	type idRow struct {
		ID string `db:"id"`
	}
	var rows []idRow
	_ = app.DB().Select("id").From("scripts").
		Where(dbx.HashExp{"name": "group-ranking"}).
		Limit(1).All(&rows)
	if len(rows) == 0 {
		return nil // not seeded yet — seed migration will use the correct code
	}
	rec, err := app.FindRecordById("scripts", rows[0].ID)
	if err != nil {
		return nil
	}
	rec.Set("code", rankingScript)
	return app.Save(rec)
}
