package whatsapp

import (
	"github.com/lichti/zaplab/internal/webhook"
	"github.com/pocketbase/pocketbase/core"
)

// InitWebhookDeliveryLogger wires the webhook package's OnDelivery callback to
// persist delivery records into the webhook_deliveries PocketBase collection.
// Called once from api.Init (after pb is set via whatsapp.Init).
func InitWebhookDeliveryLogger() {
	wh.OnDelivery = func(rec webhook.DeliveryRecord) {
		if shuttingDown.Load() || pb == nil || pb.DB() == nil {
			return
		}
		col, err := pb.FindCollectionByNameOrId("webhook_deliveries")
		if err != nil {
			return
		}
		r := core.NewRecord(col)
		r.Set("webhook_url", rec.WebhookURL)
		r.Set("event_type", rec.EventType)
		r.Set("status", rec.Status)
		r.Set("attempt", rec.Attempt)
		r.Set("http_status", rec.HTTPStatus)
		r.Set("error_msg", rec.ErrorMsg)
		r.Set("delivered_at", rec.DeliveredAt)
		_ = pb.Save(r)
	}
}
