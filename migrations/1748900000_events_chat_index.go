package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// upEventsChatIndex adds an expression index on json_extract(raw,'$.Info.Chat')
// combined with created to speed up the group-overview / ranking queries.
// Without this index every query on events filters 100K+ rows via json_extract
// on every row (full table scan).  With the index SQLite jumps directly to the
// relevant rows for a given chat JID and time window.
func init() {
	m.Register(upEventsChatIndex, downEventsChatIndex)
}

func upEventsChatIndex(app core.App) error {
	db := app.DB()

	// Primary index: chat_jid + created — covers the most common filter pattern:
	//   WHERE json_extract(raw,'$.Info.Chat') = ? AND created >= ?
	// The compound key lets SQLite filter by chat first, then range-scan by date.
	_, err := db.NewQuery(`CREATE INDEX IF NOT EXISTS idx_events_chat_created
		ON events (json_extract(raw,'$.Info.Chat'), created DESC)`).Execute()
	if err != nil {
		return err
	}

	// Secondary index: sender_jid — accelerates sender-stats GROUP BY queries.
	_, err = db.NewQuery(`CREATE INDEX IF NOT EXISTS idx_events_sender_created
		ON events (json_extract(raw,'$.Info.Sender'), created DESC)`).Execute()
	if err != nil {
		return err
	}

	// Refresh query-planner statistics after adding new indexes.
	_, _ = db.NewQuery("PRAGMA optimize").Execute()
	return nil
}

func downEventsChatIndex(app core.App) error {
	db := app.DB()
	_, _ = db.NewQuery("DROP INDEX IF EXISTS idx_events_chat_created").Execute()
	_, _ = db.NewQuery("DROP INDEX IF EXISTS idx_events_sender_created").Execute()
	return nil
}
