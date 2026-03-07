package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Increase the raw JSON field limit to 100 MB to accommodate large HistorySync payloads.
const rawMaxSize int64 = 104_857_600 // 100 MB

func init() {
	m.Register(upIncreaseRaw, downIncreaseRaw)
}

func upIncreaseRaw(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idEvents)
	if err != nil {
		return err
	}

	field, ok := col.Fields.GetById("nk6bu5ir").(*core.JSONField)
	if !ok || field.MaxSize == rawMaxSize {
		return nil // already updated or field not found
	}

	field.MaxSize = rawMaxSize
	return app.Save(col)
}

func downIncreaseRaw(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idEvents)
	if err != nil {
		return err
	}

	field, ok := col.Fields.GetById("nk6bu5ir").(*core.JSONField)
	if !ok {
		return nil
	}

	field.MaxSize = 2_000_000
	return app.Save(col)
}
