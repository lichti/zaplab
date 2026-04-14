package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/lichti/zaplab/internal/whatsapp"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// getMetrics returns a Prometheus-compatible text exposition of key ZapLab metrics.
// GET /metrics (public endpoint — typically protected by network policy, not auth).
func getMetrics(e *core.RequestEvent) error {
	var b strings.Builder

	b.WriteString("# HELP zaplab_connection_status WhatsApp connection state (1=connected, 0=other)\n")
	b.WriteString("# TYPE zaplab_connection_status gauge\n")
	status := string(whatsapp.GetConnectionStatus())
	connected := 0
	if status == "connected" {
		connected = 1
	}
	fmt.Fprintf(&b, "zaplab_connection_status{status=%q} %d\n", status, connected)

	b.WriteString("# HELP zaplab_event_queue_len Number of events waiting in the serialized worker queue\n")
	b.WriteString("# TYPE zaplab_event_queue_len gauge\n")
	fmt.Fprintf(&b, "zaplab_event_queue_len %d\n", whatsapp.EventQueueLen())

	b.WriteString("# HELP zaplab_activity_trackers_total Number of active device activity trackers\n")
	b.WriteString("# TYPE zaplab_activity_trackers_total gauge\n")
	fmt.Fprintf(&b, "zaplab_activity_trackers_total %d\n", len(whatsapp.GetTrackerStatus()))

	// Event counts by type (last 24h)
	b.WriteString("# HELP zaplab_events_24h Number of events persisted in the last 24 hours by type\n")
	b.WriteString("# TYPE zaplab_events_24h gauge\n")
	type evtCount struct {
		Type  string  `db:"type"`
		Count float64 `db:"cnt"`
	}
	var evtCounts []evtCount
	_ = pb.DB().NewQuery(
		"SELECT type, COUNT(*) as cnt FROM events WHERE created >= datetime('now','-1 day') GROUP BY type ORDER BY cnt DESC LIMIT 50",
	).All(&evtCounts)
	for _, ec := range evtCounts {
		fmt.Fprintf(&b, "zaplab_events_24h{type=%q} %.0f\n", ec.Type, ec.Count)
	}

	// Receipt latency percentiles (p50, p95) from last 24h
	b.WriteString("# HELP zaplab_receipt_latency_ms Receipt delivery latency in milliseconds\n")
	b.WriteString("# TYPE zaplab_receipt_latency_ms summary\n")
	type latRow struct {
		P50 float64 `db:"p50"`
		P95 float64 `db:"p95"`
	}
	var lat latRow
	_ = pb.DB().NewQuery(`
		SELECT
			PERCENTILE_DISC(0.5) WITHIN GROUP (ORDER BY latency_ms) as p50,
			PERCENTILE_DISC(0.95) WITHIN GROUP (ORDER BY latency_ms) as p95
		FROM receipt_latency
		WHERE receipt_type='delivered' AND created >= datetime('now','-1 day')
	`).One(&lat)
	if lat.P50 > 0 {
		fmt.Fprintf(&b, "zaplab_receipt_latency_ms{quantile=\"0.5\",type=\"delivered\"} %.0f\n", lat.P50)
		fmt.Fprintf(&b, "zaplab_receipt_latency_ms{quantile=\"0.95\",type=\"delivered\"} %.0f\n", lat.P95)
	}

	// Webhook delivery stats
	b.WriteString("# HELP zaplab_webhook_deliveries_total Webhook delivery attempts by status\n")
	b.WriteString("# TYPE zaplab_webhook_deliveries_total counter\n")
	type wbRow struct {
		Status string  `db:"status"`
		Count  float64 `db:"cnt"`
	}
	var wbCounts []wbRow
	_ = pb.DB().NewQuery(
		"SELECT status, COUNT(*) as cnt FROM webhook_deliveries WHERE created >= datetime('now','-1 day') GROUP BY status",
	).Bind(dbx.Params{}).All(&wbCounts)
	for _, wc := range wbCounts {
		fmt.Fprintf(&b, "zaplab_webhook_deliveries_total{status=%q} %.0f\n", wc.Status, wc.Count)
	}

	// Scheduled messages by status
	b.WriteString("# HELP zaplab_scheduled_messages Number of scheduled messages by status\n")
	b.WriteString("# TYPE zaplab_scheduled_messages gauge\n")
	type smRow struct {
		Status string  `db:"status"`
		Count  float64 `db:"cnt"`
	}
	var smCounts []smRow
	_ = pb.DB().NewQuery(
		"SELECT status, COUNT(*) as cnt FROM scheduled_messages GROUP BY status",
	).Bind(dbx.Params{}).All(&smCounts)
	for _, sc := range smCounts {
		fmt.Fprintf(&b, "zaplab_scheduled_messages{status=%q} %.0f\n", sc.Status, sc.Count)
	}

	e.Response.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	return e.String(http.StatusOK, b.String())
}
