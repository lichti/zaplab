package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// upDBPerformance adds composite indexes and tunes SQLite pragmas for better
// read/write throughput across the most-queried collections.
func init() {
	m.Register(upDBPerformance, downDBPerformance)
}

func upDBPerformance(app core.App) error {
	db := app.DB()

	// ── SQLite tuning ────────────────────────────────────────────────────────────
	// Increase in-memory page cache to 64 MB (default is ~2 MB).
	if _, err := db.NewQuery("PRAGMA cache_size = -65536").Execute(); err != nil {
		return err
	}
	// Store temp tables/indices in memory instead of on-disk.
	if _, err := db.NewQuery("PRAGMA temp_store = MEMORY").Execute(); err != nil {
		return err
	}
	// Increase memory-mapped I/O limit to 256 MB for sequential scans of events.
	if _, err := db.NewQuery("PRAGMA mmap_size = 268435456").Execute(); err != nil {
		return err
	}

	// ── Composite indexes ────────────────────────────────────────────────────────

	// auto_reply_rules: evaluator queries WHERE enabled=1 ORDER BY priority ASC
	// A composite (enabled, priority) lets SQLite both filter and sort with one scan.
	_, _ = db.NewQuery(`CREATE INDEX IF NOT EXISTS idx_arr_enabled_priority
		ON auto_reply_rules(enabled, priority ASC)`).Execute()

	// scheduled_messages: worker queries WHERE status='pending' AND scheduled_at <= NOW()
	// Composite (status, scheduled_at) avoids a full-table scan every minute.
	_, _ = db.NewQuery(`CREATE INDEX IF NOT EXISTS idx_scm_status_scheduled
		ON scheduled_messages(status, scheduled_at ASC)`).Execute()

	// webhook_deliveries: API commonly filters by status then orders by created DESC
	_, _ = db.NewQuery(`CREATE INDEX IF NOT EXISTS idx_wbd_status_created
		ON webhook_deliveries(status, created DESC)`).Execute()

	// message_reactions: stats group by chat_jid + emoji; log filters by chat_jid
	_, _ = db.NewQuery(`CREATE INDEX IF NOT EXISTS idx_mre_chat_emoji
		ON message_reactions(chat_jid, emoji)`).Execute()

	// mentions: stats group by mentioned_jid; frequently filtered by is_bot
	_, _ = db.NewQuery(`CREATE INDEX IF NOT EXISTS idx_men_bot_jid
		ON mentions(is_bot, mentioned_jid)`).Execute()

	// events: receipt latency looks up WHERE msgID=? — already indexed.
	// Add a partial index covering only rows where msgID is non-empty (avoids
	// indexing the majority of events that have no msgID).
	_, _ = db.NewQuery(`CREATE INDEX IF NOT EXISTS idx_events_msgid_nonempty
		ON events(msgID) WHERE msgID != ''`).Execute()

	// conn_events: often queried for latest connection status (ORDER BY created DESC)
	_, _ = db.NewQuery(`CREATE INDEX IF NOT EXISTS idx_ce_type_created
		ON conn_events(event_type, created DESC)`).Execute()

	return nil
}

func downDBPerformance(app core.App) error {
	db := app.DB()
	for _, idx := range []string{
		"idx_arr_enabled_priority",
		"idx_scm_status_scheduled",
		"idx_wbd_status_created",
		"idx_mre_chat_emoji",
		"idx_men_bot_jid",
		"idx_events_msgid_nonempty",
		"idx_ce_type_created",
	} {
		_, _ = db.NewQuery("DROP INDEX IF EXISTS " + idx).Execute()
	}
	return nil
}
