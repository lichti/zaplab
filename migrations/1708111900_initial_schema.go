package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Collection IDs — kept stable so existing databases remain compatible.
const (
	idErrors       = "j2f9h8ebamqvcz3"
	idEvents       = "1im1g6nv2b2meaa"
	idCustomers    = "p57gurkc2595v8m"
	idPhoneNumbers = "fd7tbt5fyxi3coz"
	idCredits      = "fcedygo07u4m8c7"
	idHistory      = "m30rep1f1z5h85z"
)

func init() {
	m.Register(up, down)
}

// ─── Up ───────────────────────────────────────────────────────────────────────

func up(app core.App) error {
	for _, step := range []func(core.App) error{
		createErrors,
		createEvents,
		createCustomers,
		updateUsers,
		createPhoneNumbers,
		createCredits,
		createHistory,
	} {
		if err := step(app); err != nil {
			return err
		}
	}
	return nil
}

func createErrors(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idErrors); err == nil {
		return nil // already exists
	}
	col := core.NewBaseCollection("errors", idErrors)
	col.Fields.Add(
		textField("nu0ghabm", "type", false, false),
		// Field name matches persist.go record.Set("EvtError", ...)
		textField("pbhunrev", "EvtError", false, false),
		jsonField("xlhowioc", "raw", 2_000_000),
	)
	addAutodateFields(col)
	col.AddIndex("idx_errors_type", false, "type", "")
	col.AddIndex("idx_errors_created", false, "created", "")
	col.AddIndex("idx_errors_type_created", false, "type,created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	col.CreateRule = &empty
	return app.Save(col)
}

func createEvents(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idEvents); err == nil {
		return nil
	}
	col := core.NewBaseCollection("events", idEvents)
	col.Fields.Add(
		textField("4la5mfd6", "type", false, false),
		jsonField("nk6bu5ir", "raw", 2_000_000),
		jsonField("xsh3x4yc", "extra", 2_000_000),
		// maxSize matches the 50 MB limit enforced by the REST API
		fileField("pgiqkuyb", "file", 52_428_800),
		textField("x8tcak9t", "msgID", false, false),
	)
	addAutodateFields(col)
	col.AddIndex("idx_events_type", false, "type", "")
	col.AddIndex("idx_events_created", false, "created", "")
	col.AddIndex("idx_events_type_created", false, "type,created", "")
	col.AddIndex("idx_events_msgID", false, "msgID", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	return app.Save(col)
}

func createCustomers(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idCustomers); err == nil {
		return nil
	}
	col := core.NewBaseCollection("customers", idCustomers)
	col.Fields.Add(textField("adylfkfp", "stripe_id", false, false))
	addAutodateFields(col)
	return app.Save(col)
}

func updateUsers(app core.App) error {
	users, err := app.FindCollectionByNameOrId("_pb_users_auth_")
	if err != nil {
		return err
	}
	if users.Fields.GetById("fro7xuzm") != nil {
		return nil // already added
	}
	users.Fields.Add(relationField("fro7xuzm", "customer", idCustomers, false))
	return app.Save(users)
}

func createPhoneNumbers(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idPhoneNumbers); err == nil {
		return nil
	}
	col := core.NewBaseCollection("phone_numbers", idPhoneNumbers)
	col.Fields.Add(
		textField("zjkokrfk", "phone_number", true, true),
		relationField("ipaawifq", "customer", idCustomers, true),
	)
	addAutodateFields(col)
	return app.Save(col)
}

func createCredits(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idCredits); err == nil {
		return nil
	}
	minZero := 0.0
	col := core.NewBaseCollection("credits", idCredits)
	col.Fields.Add(
		relationField("aus86sog", "customer", idCustomers, true),
		&core.NumberField{
			Id:          "znp6sv2c",
			Name:        "qty",
			Presentable: true,
			Min:         &minZero,
			OnlyInt:     true,
		},
		&core.DateField{
			Id:          "ykwcogam",
			Name:        "expires",
			Required:    true,
			Presentable: true,
		},
	)
	addAutodateFields(col)
	return app.Save(col)
}

func createHistory(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idHistory); err == nil {
		return nil
	}
	col := core.NewBaseCollection("history", idHistory)
	col.Fields.Add(
		relationField("eymevtsj", "customer", idCustomers, true),
		textField("px5xjgcw", "phone_number", true, true),
		textField("wsprf5w3", "msgID", true, true),
		relationField("vkpboc8d", "event", idEvents, true),
	)
	addAutodateFields(col)
	return app.Save(col)
}

// ─── Down ─────────────────────────────────────────────────────────────────────

func down(app core.App) error {
	// Drop in reverse dependency order.
	for _, id := range []string{idHistory, idCredits, idPhoneNumbers, idCustomers, idEvents, idErrors} {
		col, err := app.FindCollectionByNameOrId(id)
		if err != nil {
			continue // already gone
		}
		if err := app.Delete(col); err != nil {
			return err
		}
	}

	// Remove customer field from users.
	if users, err := app.FindCollectionByNameOrId("_pb_users_auth_"); err == nil {
		users.Fields.RemoveById("fro7xuzm")
		_ = app.Save(users)
	}
	return nil
}

// ─── Field helpers ────────────────────────────────────────────────────────────

// addAutodateFields adds system created/updated autodate fields to a collection.
// In PocketBase v0.36 these are not added automatically and must be explicit.
func addAutodateFields(col *core.Collection) {
	col.Fields.Add(
		&core.AutodateField{
			Id:       "autodate_created",
			Name:     "created",
			System:   true,
			OnCreate: true,
		},
		&core.AutodateField{
			Id:       "autodate_updated",
			Name:     "updated",
			System:   true,
			OnCreate: true,
			OnUpdate: true,
		},
	)
}

func textField(id, name string, required, presentable bool) *core.TextField {
	return &core.TextField{
		Id: id, Name: name,
		Required: required, Presentable: presentable,
	}
}

func jsonField(id, name string, maxSize int64) *core.JSONField {
	return &core.JSONField{
		Id: id, Name: name, MaxSize: maxSize,
	}
}

func fileField(id, name string, maxSize int64) *core.FileField {
	return &core.FileField{
		Id: id, Name: name,
		MaxSelect: 1,
		MaxSize:   maxSize,
		Thumbs:    []string{"300x300"},
	}
}

func relationField(id, name, collectionId string, required bool) *core.RelationField {
	return &core.RelationField{
		Id: id, Name: name, CollectionId: collectionId,
		Required:    required,
		Presentable: required,
		MaxSelect:   1,
	}
}
