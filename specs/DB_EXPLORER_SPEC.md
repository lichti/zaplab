# DB Explorer — Technical Specification

## Overview

The DB Explorer exposes the internal whatsmeow SQLite database (`whatsapp.db`) for
protocol research and testing. It provides:

- **Read**: browse all 12 whatsmeow tables with pagination, full-text filter, and
  column-level protocol documentation.
- **Write**: edit individual cells and delete rows, with automatic backup before every
  mutation.
- **Backup & Restore**: create named snapshots (via `VACUUM INTO`) and restore any
  snapshot, which triggers a full whatsmeow stack reinitialisation.
- **Reconnect controls**: force a WebSocket reconnect or a full teardown/rebuild to
  observe how WhatsApp reacts to modified cryptographic state.

---

## Architecture

### Go backend

| File | Responsibility |
|------|----------------|
| `internal/api/dbexplorer.go` | All DB Explorer HTTP handlers, connection management, backup/restore logic |
| `internal/whatsapp/reconnect.go` | `ForceReconnect()` and `Reinitialize()` exported functions |
| `internal/whatsapp/deps.go` | `GetDBAddress()`, `GetDBDialect()` accessors; `storeContainer` package-level var |

### Database connections

Two long-lived `*sql.DB` connections are opened when the API initialises:

| Variable | Mode | DSN suffix | Purpose |
|----------|------|------------|---------|
| `waDB` | Read-only | `?mode=ro` | All `SELECT` queries (safe, never locks writes) |
| `waDBWrite` | Read-write | `?mode=rwc` | `UPDATE`, `DELETE`, `VACUUM INTO` |

Both connections have `SetMaxOpenConns(1)` to serialise access and avoid WAL contention
with the whatsmeow connection.

### Security

| Control | Implementation |
|---------|----------------|
| Table allowlist | `allowedTables` slice; any table name not in the list returns 404 |
| Column validation | All column names in `PATCH` bodies are validated against `PRAGMA table_info` before being used in SQL |
| Bound parameters | Filter values and row values are always passed as `?` placeholders, never concatenated |
| Read-only connection | `waDB` opened with `mode=ro`; cannot accidentally mutate the file |
| Auth | Both endpoints registered with `requireAuth()` (PocketBase JWT or `X-API-Token`) |

---

## Exposed Tables

| Table | Description |
|-------|-------------|
| `whatsmeow_device` | Device identity: JID, Noise key, Identity key, Signed pre-key, Registration ID, platform |
| `whatsmeow_identity_keys` | Signal Protocol identity keys per JID/device pair |
| `whatsmeow_pre_keys` | One-time pre-keys for X3DH session initiation |
| `whatsmeow_sessions` | Signal Double Ratchet session state per contact |
| `whatsmeow_sender_keys` | Group SenderKey Distribution records |
| `whatsmeow_app_state_sync_keys` | Symmetric keys for app state patch decryption |
| `whatsmeow_app_state_version` | Version index and Merkle root hash per app state collection |
| `whatsmeow_app_state_mutation_macs` | HMAC integrity codes for app state mutations |
| `whatsmeow_contacts` | Contacts with push names synced from the server |
| `whatsmeow_chat_settings` | Per-chat settings (mute, pin, archive) |
| `whatsmeow_message_secrets` | Per-message secrets for media/ephemeral decryption |
| `whatsmeow_privacy_tokens` | Privacy tokens used in contact availability checks |

---

## API Endpoints

See [API_SPEC.md — DB Explorer section](API_SPEC.md#db-explorer) for the full HTTP
reference.

Quick summary:

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/zaplab/api/db/tables` | 🔒 | List tables with descriptions and row counts |
| `GET` | `/zaplab/api/db/tables/{table}` | 🔒 | Paginated rows; includes `_rowid_` as first column |
| `PATCH` | `/zaplab/api/db/tables/{table}/{rowid}` | 🔒 | Update columns; auto-backup before write |
| `DELETE` | `/zaplab/api/db/tables/{table}/{rowid}` | 🔒 | Delete row; auto-backup before write |
| `POST` | `/zaplab/api/db/reconnect` | 🔒 | Reconnect (`full: false`) or full reinit (`full: true`) |
| `POST` | `/zaplab/api/db/backup` | 🔒 | Create manual backup |
| `GET` | `/zaplab/api/db/backups` | 🔒 | List backups |
| `POST` | `/zaplab/api/db/restore` | 🔒 | Restore backup + reinitialise whatsmeow |
| `DELETE` | `/zaplab/api/db/backups/{name}` | 🔒 | Delete backup |

---

## Backup System

### Creation

Backups are created using SQLite's `VACUUM INTO 'path/to/file.db'`.

- Produces a clean, WAL-checkpointed copy with no journal files.
- Does not block ongoing reads on `waDB`.
- Stored in `pb_data/db_backups/` with the filename pattern `whatsapp_YYYYMMDD_HHMMSS.db`.
- **Auto-backup**: triggered automatically before every `PATCH` or `DELETE` operation.
- **Manual backup**: also available via `POST /zaplab/api/db/backup`.

### Restore flow

```
POST /zaplab/api/db/restore { "name": "whatsapp_20260316_143022.db" }
  1. Close waDB and waDBWrite connections
  2. io.Copy(backup → whatsapp.db) + Sync
  3. os.Remove(whatsapp.db-wal), os.Remove(whatsapp.db-shm)
  4. Reopen waDB (mode=ro) and waDBWrite (mode=rwc)
  5. whatsapp.Reinitialize():
     a. client.Disconnect()
     b. storeContainer.Close()
     c. sqlstore.New(...) → new container
     d. container.GetFirstDevice() → device
     e. whatsmeow.NewClient(device) → new client
     f. client.AddEventHandler(handler)
     g. client.Connect()
```

After a restore, the entire in-memory whatsmeow state (session keys, identity, device info)
is discarded and rebuilt from the restored file.

---

## Frontend

### Section: `dbexplorer`

Implemented in `pb_public/js/sections/dbexplorer.js` and rendered in
`pb_public/index.html`.

**Layout:**

```
┌─ Header bar ──────────────────────────────────────────────────┐
│ DB Explorer  [Reconnect] [Full Reinit] [Backups] [↺]          │
├─ Backups panel (collapsible) ─────────────────────────────────┤
│ [+ New backup]  whatsapp_20260316_143022.db (200KB) [Restore] │
├──────────────────────────────────────────────────────────────┤
│ Left panel (table list) │ Right panel                         │
│ device             1    │ Toolbar: filter, Search, Copy       │
│ identity_keys      5    ├──────────────────────────────────── │
│ pre_keys          100   │ Table (data columns, _rowid_ hidden) │
│ sessions          12    │                                      │
│ ...                     ├──────────────────────────────────── │
│                         │ Pagination footer                    │
│                         │                         │ Detail     │
│                         │  (row selected)         │ panel      │
│                         │                         │ View/Edit  │
└─────────────────────────┴─────────────────────────┴───────────┘
```

**Row detail panel — View mode:**
- Lists each column with its name, protocol documentation tooltip, and formatted value.
- BLOB values shown in full hex with a `hex blob` badge.
- "Edit" button switches to edit mode.
- "Delete this row" button with two-step confirmation.

**Row detail panel — Edit mode:**
- Each column rendered as a `<textarea>` pre-filled with the current value.
- BLOB fields show a `hex blob` badge; the user types hex; the backend decodes before storing.
- "Reconnect WhatsApp after save" checkbox (default: on).
- "Save & backup" button → `PATCH` → success toast shows backup filename.
- "Cancel" returns to view mode without saving.

**Backups panel:**
- Collapsible strip below the header bar.
- Lists all `.db` files in `pb_data/db_backups/` with name, size, Restore button, Delete button.
- "New backup" button creates a manual snapshot immediately.

**State object (`dbe`):**
```javascript
{
  tables, loadingTables,           // table list
  table, description, columns, types, rows, page, limit, total, pages, filter, loading, error,
  selectedRow, copied,             // table view
  editing, editValues, saving, saveError, saveResult, autoReconnect,
  deleting, confirmDelete,         // edit / delete
  showBackups, backups, backupsLoading, backupCreating, backupError, restoring,
  reconnecting
}
```

---

## Protocol Research Use Cases

| Scenario | Steps |
|----------|-------|
| Observe session reset | Edit `whatsmeow_sessions.session` (corrupt one byte) → Save → Reconnect → Watch Live Events for `SessionReset` or re-keying messages |
| Test pre-key exhaustion | Delete all rows in `whatsmeow_pre_keys` → Full Reinit → Observe server requesting new pre-keys |
| Push name spoofing | Edit `whatsmeow_device.push_name` → Reconnect → Send a message; the server broadcasts the new push name |
| App state tampering | Edit `whatsmeow_app_state_version.hash` → Full Reinit → Observe full app state re-sync |
| Noise key replay | Save known `noise_key` value → Reconnect → Observe if the server accepts or rejects the old key |
| Restore known state | Backup before exploration → Run experiments → Restore original backup to return to clean state |
