package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const idWebhookDeliveries = "wbd1a2b3c4d5e6f7"

func init() {
	m.Register(upWebhookDeliveries, downWebhookDeliveries)
}

func upWebhookDeliveries(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idWebhookDeliveries); err == nil {
		return nil
	}
	col := core.NewBaseCollection("webhook_deliveries", idWebhookDeliveries)
	col.Fields.Add(
		textField("wbd1url", "webhook_url", true, false),
		textField("wbd2type", "event_type", false, false),
		textField("wbd3status", "status", false, false), // delivered/failed
		&core.NumberField{Id: "wbd4attempt", Name: "attempt"},
		&core.NumberField{Id: "wbd5http", Name: "http_status"},
		textField("wbd6error", "error_msg", false, false),
		textField("wbd7at", "delivered_at", false, false),
	)
	addAutodateFields(col)
	col.AddIndex("idx_wbd_status", false, "status", "")
	col.AddIndex("idx_wbd_url", false, "webhook_url", "")
	col.AddIndex("idx_wbd_created", false, "created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	return app.Save(col)
}

func downWebhookDeliveries(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idWebhookDeliveries)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
