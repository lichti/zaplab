package migrations

import (
	"strings"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(upOptimizeIndexes, downOptimizeIndexes)
}

type idxChange struct {
	remove []string
	add    []struct{ name, cols, where string }
}

// optimizeChanges describes the index changes per collection.
// Redundant single-column indexes are replaced by composite ones that cover
// both filtering and ordering in a single B-tree scan.
var optimizeChanges = map[string]idxChange{
	// evaluator: WHERE enabled=1 ORDER BY priority — composite covers both
	"auto_reply_rules": {
		remove: []string{"idx_arr_enabled", "idx_arr_priority"},
		add:    []struct{ name, cols, where string }{{"idx_arr_enabled_priority", "enabled, priority", ""}},
	},
	// status timeline: already composite; drop single-column prefix
	"conn_events": {
		remove: []string{"idx_ce_type"},
		add:    []struct{ name, cols, where string }{{"idx_ce_type_created", "event_type, created DESC", ""}},
	},
	// reaction log: composite (chat_jid, emoji) covers chat-only queries too
	"message_reactions": {
		remove: []string{"idx_mre_chat"},
		add:    []struct{ name, cols, where string }{{"idx_mre_chat_emoji", "chat_jid, emoji", ""}},
	},
	// mention stats: (is_bot, mentioned_jid) composite supersedes both singles
	"mentions": {
		remove: []string{"idx_men_isbot", "idx_men_mentioned"},
		add:    []struct{ name, cols, where string }{{"idx_men_bot_jid", "is_bot, mentioned_jid", ""}},
	},
	// delivery log: (status, created DESC) covers status-only queries too
	"webhook_deliveries": {
		remove: []string{"idx_wbd_status"},
		add:    []struct{ name, cols, where string }{{"idx_wbd_status_created", "status, created DESC", ""}},
	},
	// scheduler worker: (status, scheduled_at) covers both columns in one scan
	"scheduled_messages": {
		remove: []string{"idx_scm_status", "idx_scm_scheduled"},
		add:    []struct{ name, cols, where string }{{"idx_scm_status_scheduled", "status, scheduled_at", ""}},
	},

	// events: replace full msgID index with partial (msgID != '') — ~34K vs 106K rows indexed.
	// ~68% of events have no msgID; the partial index is 3x smaller with the same selectivity.
	"events": {
		remove: []string{"idx_events_msgID"},
		add:    []struct{ name, cols, where string }{{"idx_events_msgid_nonempty", "msgID", "msgID != ''"}},
	},

	// frames: the single-column idx_frames_module is UNUSED by the query planner
	// (SQLite prefers idx_frames_created to avoid ORDER BY sort). The composite
	// (module, created) eliminates the full-index scan for every module-filtered query.
	// Also drop idx_frames_level — covered by the existing idx_frames_level_created.
	"frames": {
		remove: []string{"idx_frames_module", "idx_frames_level"},
		add:    []struct{ name, cols, where string }{{"idx_frames_module_created", "module, created", ""}},
	},

	// receipt_latency (58K rows): add composites for analytics and chat-specific stats.
	// Remove idx_rl_type — only 2 distinct values; the new composite supersedes it.
	"receipt_latency": {
		remove: []string{"idx_rl_type"},
		add: []struct{ name, cols, where string }{
			{"idx_rl_type_created", "receipt_type, created", ""},
			{"idx_rl_chat_type", "chat_jid, receipt_type", ""},
		},
	},

	// group_membership: sorted lookups by group/member need composite to avoid temp B-TREE.
	"group_membership": {
		remove: []string{"idx_gm_group", "idx_gm_member"},
		add: []struct{ name, cols, where string }{
			{"idx_gm_group_created", "group_jid, created", ""},
			{"idx_gm_member_created", "member_jid, created", ""},
		},
	},
}

// restoreChanges is the inverse of optimizeChanges (for down migration).
var restoreChanges = map[string]idxChange{
	"auto_reply_rules": {
		remove: []string{"idx_arr_enabled_priority"},
		add: []struct{ name, cols, where string }{
			{"idx_arr_enabled", "enabled", ""},
			{"idx_arr_priority", "priority", ""},
		},
	},
	"conn_events": {
		remove: []string{"idx_ce_type_created"},
		add:    []struct{ name, cols, where string }{{"idx_ce_type", "event_type", ""}},
	},
	"message_reactions": {
		remove: []string{"idx_mre_chat_emoji"},
		add:    []struct{ name, cols, where string }{{"idx_mre_chat", "chat_jid", ""}},
	},
	"mentions": {
		remove: []string{"idx_men_bot_jid"},
		add: []struct{ name, cols, where string }{
			{"idx_men_isbot", "is_bot", ""},
			{"idx_men_mentioned", "mentioned_jid", ""},
		},
	},
	"webhook_deliveries": {
		remove: []string{"idx_wbd_status_created"},
		add:    []struct{ name, cols, where string }{{"idx_wbd_status", "status", ""}},
	},
	"scheduled_messages": {
		remove: []string{"idx_scm_status_scheduled"},
		add: []struct{ name, cols, where string }{
			{"idx_scm_status", "status", ""},
			{"idx_scm_scheduled", "scheduled_at", ""},
		},
	},
	"events": {
		remove: []string{"idx_events_msgid_nonempty"},
		add:    []struct{ name, cols, where string }{{"idx_events_msgID", "msgID", ""}},
	},
	"frames": {
		remove: []string{"idx_frames_module_created"},
		add: []struct{ name, cols, where string }{
			{"idx_frames_module", "module", ""},
			{"idx_frames_level", "level", ""},
		},
	},
	"receipt_latency": {
		remove: []string{"idx_rl_type_created", "idx_rl_chat_type"},
		add:    []struct{ name, cols, where string }{{"idx_rl_type", "receipt_type", ""}},
	},
	"group_membership": {
		remove: []string{"idx_gm_group_created", "idx_gm_member_created"},
		add: []struct{ name, cols, where string }{
			{"idx_gm_group", "group_jid", ""},
			{"idx_gm_member", "member_jid", ""},
		},
	},
}

// rawIndexesFromMigration1748700 are indexes that were created via raw SQL in
// the previous migration and are NOT yet registered in PocketBase's _collections
// metadata. They must be dropped before app.Save(col) tries to CREATE them
// (PocketBase uses CREATE INDEX, not CREATE INDEX IF NOT EXISTS).
var rawIndexesFromMigration1748700 = []string{
	"idx_arr_enabled_priority",
	"idx_scm_status_scheduled",
	"idx_wbd_status_created",
	"idx_mre_chat_emoji",
	"idx_men_bot_jid",
	"idx_events_msgid_nonempty",
	"idx_ce_type_created",
}

func applyIdxChanges(app core.App, changes map[string]idxChange) error {
	// Drop raw SQL indexes from migration 1748700 so PocketBase can recreate
	// them as properly managed indexes (registered in _collections metadata).
	for _, idx := range rawIndexesFromMigration1748700 {
		_, _ = app.DB().NewQuery("DROP INDEX IF EXISTS " + idx).Execute()
	}

	for colName, change := range changes {
		col, err := app.FindCollectionByNameOrId(colName)
		if err != nil {
			continue // collection doesn't exist yet; skip gracefully
		}
		for _, name := range change.remove {
			col.RemoveIndex(name)
		}
		for _, a := range change.add {
			// Skip if already registered (idempotent)
			already := false
			for _, existing := range col.Indexes {
				if strings.Contains(strings.ToLower(existing), strings.ToLower(a.name)) {
					already = true
					break
				}
			}
			if !already {
				col.AddIndex(a.name, false, a.cols, a.where)
			}
		}
		if err := app.Save(col); err != nil {
			return err
		}
	}
	// Refresh query planner statistics after all index changes.
	_, _ = app.DB().NewQuery("PRAGMA optimize").Execute()
	return nil
}

func upOptimizeIndexes(app core.App) error {
	return applyIdxChanges(app, optimizeChanges)
}

func downOptimizeIndexes(app core.App) error {
	return applyIdxChanges(app, restoreChanges)
}
