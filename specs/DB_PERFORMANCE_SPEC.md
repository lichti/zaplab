# Database Performance — Technical Specification

## Overview

SQLite is the embedded database used by PocketBase. As a single-writer database, query efficiency is critical both for read latency (API responses, UI) and write throughput (event ingestion at ~100 events/s sustained). This document describes the index strategy and SQLite tuning applied to the ZapLab database.

---

## SQLite Pragmas (migration 1748700)

Applied at migration time. Persist across restarts because they modify the database header page.

| Pragma | Value | Effect |
|---|---|---|
| `cache_size` | `-65536` (64 MB) | In-memory page cache. Default was ~8 MB. Reduces disk I/O for the 275 MB `data.db`. |
| `temp_store` | `MEMORY` | Temp tables and sort buffers go to RAM instead of a temp file. Eliminates temp-file I/O for ORDER BY and GROUP BY operations. |
| `mmap_size` | `268435456` (256 MB) | Memory-mapped I/O. Sequential page reads (large scans on `events`, `receipt_latency`) bypass the kernel page cache copy. |

PocketBase already enables WAL mode (`journal_mode=WAL`). `PRAGMA optimize` is run after each index migration to update the query planner's statistics.

---

## Index Design Principles

1. **Composites supersede single-column indexes** — if a query filters on column A and sorts/groups on column B, a `(A, B)` composite is used for both in a single B-tree scan. The single-column `A` index becomes redundant (SQLite can use the leftmost prefix of the composite).

2. **Partial indexes for sparse columns** — when a column is frequently empty/NULL, a `WHERE col != ''` partial index is 3–10× smaller and faster. Example: `events.msgID` is empty for ~72K of 106K rows.

3. **PRAGMA optimize** — SQLite's built-in statistics auto-analyzer. Run after bulk index changes to ensure the query planner uses the new indexes.

---

## Index Inventory (data.db)

### `events` (106K rows — largest table)

| Index | Columns | Notes |
|---|---|---|
| `idx_events_type` | `type` | Used as covering index for `COUNT(*) WHERE type = ?` |
| `idx_events_created` | `created` | Pagination / time-range scans |
| `idx_events_type_created` | `type, created` | Primary UI query: `WHERE type = ? ORDER BY created DESC` |
| `idx_events_msgid_nonempty` | `msgID WHERE msgID != ''` | **Partial**. Receipt latency lookup. Only 34K of 106K rows have a msgID. Replaced full `idx_events_msgID`. |

Dropped: `idx_events_msgID` (full, 106K entries) → replaced by partial (34K entries).

### `receipt_latency` (58K rows)

| Index | Columns | Notes |
|---|---|---|
| `idx_rl_msgid` | `msg_id` | Lookup for latency correlation |
| `idx_rl_chat` | `chat_jid` | Chat-specific queries |
| `idx_rl_created` | `created` | Time-range filter |
| `idx_rl_type_created` | `receipt_type, created` | **New.** Analytics: `WHERE receipt_type = ? AND created >= ?` |
| `idx_rl_chat_type` | `chat_jid, receipt_type` | **New.** Per-chat stats grouped by type |

Dropped: `idx_rl_type` (only 2 distinct values; full scan; superseded by composite).

### `frames` (5K rows)

| Index | Columns | Notes |
|---|---|---|
| `idx_frames_created` | `created` | Time-range + ORDER BY |
| `idx_frames_level_created` | `level, created` | Level-filtered timeline |
| `idx_frames_module_created` | `module, created` | **New.** Module-filtered timeline. Previous `idx_frames_module` was completely ignored by the planner — all module queries did a full scan on `idx_frames_created`. |

Dropped: `idx_frames_module` (single-column, ignored by planner), `idx_frames_level` (covered by `idx_frames_level_created` prefix).

### `auto_reply_rules`

| Index | Columns | Notes |
|---|---|---|
| `idx_arr_enabled_priority` | `enabled, priority ASC` | **New composite.** Evaluator query: `WHERE enabled=1 ORDER BY priority` — single B-tree scan |
| `idx_arr_created` | `created` | Retained for UI list ordering |

Dropped: `idx_arr_enabled`, `idx_arr_priority` (covered by composite).

### `scheduled_messages`

| Index | Columns | Notes |
|---|---|---|
| `idx_scm_status_scheduled` | `status, scheduled_at ASC` | **New composite.** Worker query: `WHERE status='pending' AND scheduled_at <= NOW()` |
| `idx_scm_chat` | `chat_jid` | Chat filter |
| `idx_scm_created` | `created` | UI ordering |

Dropped: `idx_scm_status`, `idx_scm_scheduled` (covered by composite).

### `webhook_deliveries`

| Index | Columns | Notes |
|---|---|---|
| `idx_wbd_status_created` | `status, created DESC` | **New composite.** Status-filtered list with recency sort |
| `idx_wbd_url` | `webhook_url` | URL filter |
| `idx_wbd_created` | `created` | Time-range filter |

Dropped: `idx_wbd_status` (covered by composite prefix).

### `message_reactions`

| Index | Columns | Notes |
|---|---|---|
| `idx_mre_chat_emoji` | `chat_jid, emoji` | **New composite.** Stats: top emojis per chat |
| `idx_mre_message` | `message_id` | Lookup by target message |
| `idx_mre_sender` | `sender_jid` | Sender filter |
| `idx_mre_created` | `created` | Timeline |

Dropped: `idx_mre_chat` (covered by composite prefix).

### `mentions`

| Index | Columns | Notes |
|---|---|---|
| `idx_men_bot_jid` | `is_bot, mentioned_jid` | **New composite.** Bot-mention stats; mention count per JID |
| `idx_men_chat` | `chat_jid` | Chat filter |
| `idx_men_created` | `created` | Timeline |

Dropped: `idx_men_isbot`, `idx_men_mentioned` (covered by composite).

### `conn_events` (2.9K rows)

| Index | Columns | Notes |
|---|---|---|
| `idx_ce_type_created` | `event_type, created DESC` | **New composite.** Connection status timeline |
| `idx_ce_created` | `created` | Retained |

Dropped: `idx_ce_type` (covered by composite prefix).

### `group_membership` (1.1K rows)

| Index | Columns | Notes |
|---|---|---|
| `idx_gm_group_created` | `group_jid, created` | **New composite.** Sorted group history (eliminates temp B-TREE) |
| `idx_gm_member_created` | `member_jid, created` | **New composite.** Sorted member history |
| `idx_gm_action` | `action` | Retained for action-type filter |
| `idx_gm_created` | `created` | Retained |

Dropped: `idx_gm_group`, `idx_gm_member` (covered by composites).

---

## Migration History

| Migration | Description |
|---|---|
| `1748700000_db_performance_indexes.go` | SQLite pragma tuning + composite/partial indexes via raw SQL |
| `1748800000_optimize_indexes.go` | Drops raw SQL indexes; re-registers all composites via PocketBase API (`col.AddIndex` / `app.Save`) so they appear in `_collections` metadata and survive collection re-saves from the admin UI. Also adds `receipt_latency` and `group_membership` composites. Runs `PRAGMA optimize`. |

> **Note:** Migration 1748700 created indexes outside PocketBase's `_collections` metadata. Migration 1748800 corrects this by dropping and re-creating them through the PocketBase collection API. This is necessary because `app.Save(col)` uses plain `CREATE INDEX` (not `IF NOT EXISTS`) — if a raw SQL index already exists, re-registration would fail without the prior DROP.

---

## Query Plan Improvements (confirmed on live DB)

| Query | Before | After |
|---|---|---|
| `frames WHERE module=? ORDER BY created DESC` | `SCAN frames USING INDEX idx_frames_created` (full scan) | `SEARCH frames USING INDEX idx_frames_module_created (module=?)` |
| `receipt_latency WHERE created>=? GROUP BY receipt_type` | `idx_rl_created` + temp B-TREE | `idx_rl_type_created` covers both |
| `group_membership WHERE group_jid=? ORDER BY created DESC` | `idx_gm_group` + temp B-TREE | `idx_gm_group_created` — no sort |
| `auto_reply_rules WHERE enabled=1 ORDER BY priority` | Two separate index scans | `idx_arr_enabled_priority` — single scan |
| `scheduled_messages WHERE status=? AND scheduled_at<=?` | Two separate index scans | `idx_scm_status_scheduled` — single scan |
| `events WHERE msgID=?` | 106K-entry full index | 34K-entry partial index |
