package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const idContactCache = "cca1a2b3c4d5e6f7"

func init() {
	m.Register(upContactCache, downContactCache)
}

func upContactCache(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idContactCache); err == nil {
		return nil
	}
	col := core.NewBaseCollection("contact_cache", idContactCache)
	col.Fields.Add(
		textField("cca1jid", "jid", true, false),
		textField("cca2name", "name", false, false),
		textField("cca3phone", "phone", false, false),
		textField("cca4about", "about", false, false),
		textField("cca5avatar", "avatar_url", false, false),
		textField("cca6lastseen", "last_seen", false, false),
		textField("cca7updat", "cache_updated_at", false, false),
	)
	addAutodateFields(col)
	col.AddIndex("idx_cca_jid", true, "jid", "") // unique
	col.AddIndex("idx_cca_updated", false, "cache_updated_at", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	col.CreateRule = &empty
	col.UpdateRule = &empty
	return app.Save(col)
}

func downContactCache(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idContactCache)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
