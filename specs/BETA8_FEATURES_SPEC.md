# Beta 8 — New Features Specification

Eight features implemented in the `feature/beta8` branch (session beginning 2026-03-17).

---

## 1. Connection Stability Dashboard

### Migration

File: `migrations/1747300000_create_conn_events.go`

Collection: `conn_events` (ID: `ce1a2b3c4d5e6f7g`)

Fields:
- `event_type` — TextField (`connected` | `disconnected`)
- `reason` — TextField (disconnect reason string, empty for connect events)
- `jid` — TextField (bot JID at event time)

### Event Recording

File: `internal/whatsapp/conntrack.go`

Function: `recordConnEvent(eventType, reason string)` — saves to `conn_events` via `pb.Save()`. Called asynchronously from `internal/whatsapp/events.go`:
- `Connected` case → `go recordConnEvent("connected", "")`
- `Disconnected` case → `go recordConnEvent("disconnected", fmt.Sprintf("%v", evt))`

### REST API

| Method | Path | Query Params | Description |
|--------|------|--------------|-------------|
| `GET` | `/zaplab/api/conn/events` | `limit`, `offset`, `since` (RFC3339) | Paginated event log |
| `GET` | `/zaplab/api/conn/stats` | — | Uptime % + counts for 24 h / 7 d / 30 d |

**File:** `internal/api/connstability.go`

**Stats response shape:**
```json
{
  "windows": [
    { "label": "24h",  "connected": 5, "disconnected": 2, "uptime_pct": 85.3 },
    { "label": "7d",   "connected": 12, "disconnected": 4, "uptime_pct": 90.1 },
    { "label": "30d",  "connected": 40, "disconnected": 9, "uptime_pct": 88.7 }
  ]
}
```

### Frontend

Section ID: `conn-stability` | JS file: `pb_public/js/sections/connstability.js` | State prefix: `cs`

Key state: `csEvents`, `csStats`, `csLoading`, `csError`

Key methods: `initConnStability()`, `loadConnData()`, `connEventBadge(type)`, `connStatCount(label, type)`, `connUptimePct(label)`

---

## 2. Script Import / Export

### REST API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/zaplab/api/scripts/export` | Returns all scripts as a JSON array |
| `POST` | `/zaplab/api/scripts/import` | Upserts scripts by name |

**File:** `internal/api/scriptsio.go`

**Export response:**
```json
[
  {
    "name": "ping",
    "description": "Replies pong",
    "code": "wa.sendText(wa.jid, 'pong')",
    "enabled": true,
    "timeout_secs": 30
  }
]
```

**Import body:** Same array format. For each item:
- If a script with the same `name` exists → update `description`, `code`, `timeout_secs`, `enabled`.
- Otherwise → create a new record.

**Import response:**
```json
{ "imported": 3, "updated": 1, "created": 2 }
```

### Frontend

Buttons added to the Scripting section header toolbar:
- **Export** → triggers `GET /zaplab/api/scripts/export` and downloads the JSON file.
- **Import** → `<input type="file" accept=".json">` → reads the file → `POST /zaplab/api/scripts/import`.

JS file: `pb_public/js/sections/scriptsio.js` | State prefix: `sio`

Key methods: `exportScripts()`, `importScripts(fileInput)`

---

## 3. WA Health Monitor

### REST API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/zaplab/api/wa/prekeys` | Pre-key supply metrics |
| `GET` | `/zaplab/api/wa/secrets` | `whatsmeow_message_secrets` rows |

**File:** `internal/api/wahealth.go`

Pre-key response:
```json
{
  "columns": ["key_id", "key_pair", "signature", "uploaded"],
  "rows": [ ... ],
  "total": 100,
  "uploaded": 75,
  "low": false
}
```

`low` is `true` when `uploaded < 20`.

Secrets response:
```json
{
  "columns": ["our_jid", "their_jid", "key"],
  "rows": [ ... ],
  "total": 42
}
```

Both endpoints use `dbeTableColumns(table)` + `dbeScanRows` from `dbexplorer.go`, then `rowsToMaps(cols, rows)` to produce JSON-serialisable maps.

### Frontend

Section ID: `wa-health` | JS file: `pb_public/js/sections/wahealth.js` | State prefix: `wah`

Tabs: `prekeys` | `secrets`

Key methods: `initWAHealth()`, `loadWAPrekeys()`, `loadWASecrets()`, `wahPrekeyBar()`

---

## 4. IQ Node Analyzer

### REST API

**Endpoint:** `GET /zaplab/api/frames/iq`

Query params: `level` (debug|info|warn|error), `iqtype` (get|set|result|error), `limit` (default 200)

**Filter logic:** `msg LIKE '%<iq%'` + optional `level = ?` + optional `msg LIKE '%type="<iqtype>"%'`

**File:** `internal/api/framesiq.go`

**Response:**
```json
{
  "frames": [
    {
      "id": "...",
      "module": "Client",
      "level": "debug",
      "seq": "42",
      "msg": "<iq type=\"get\" xmlns=\"urn:xmpp:ping\"/>",
      "created": "2026-03-17T12:00:00Z"
    }
  ],
  "total": 5
}
```

### Frontend

Tab within `frames-iq` section. State prefix: `fiq` (shared with Binary Node Inspector).

Key state: `fiq.iqLevel`, `fiq.iqType`, `fiq.iqFrames`

Key methods: `loadIQFrames()`, `fiqLevelBadge(level)`, `fiqToggleExpand(id)`

---

## 5. Binary Node Inspector

### REST API

**Endpoint:** `GET /zaplab/api/frames/binary`

Query params: `level`, `module` (Noise|Socket), `limit` (default 200)

**Filter logic:** `module IN ('Noise','Socket','noise','socket')` + optional level/module filters. Selects `COALESCE(LENGTH(msg),0) as size`.

**File:** `internal/api/framesiq.go`

**Response:**
```json
{
  "frames": [
    {
      "id": "...",
      "module": "Noise",
      "level": "info",
      "seq": "10",
      "msg": "binary frame hex or text",
      "size": 128,
      "created": "2026-03-17T12:00:00Z"
    }
  ],
  "total": 3
}
```

### Frontend

Second tab within the `frames-iq` section.

Key state: `fiq.binLevel`, `fiq.binModule`, `fiq.binFrames`

Key method: `loadBinaryFrames()`

---

## 6. Group Membership Tracker

### Migration

File: `migrations/1747310000_create_group_membership.go`

Collection: `group_membership` (ID: `gm1a2b3c4d5e6f7g`)

Fields:
- `group_jid` — TextField
- `group_name` — TextField
- `member_jid` — TextField
- `action` — TextField (`join` | `leave` | `promote` | `demote`)
- `actor_jid` — TextField (JID of the actor, may be empty)

### Event Recording

File: `internal/whatsapp/conntrack.go`

Function: `recordGroupMembership(evt *events.GroupInfo)` — iterates `evt.Join`, `evt.Leave`, `evt.Promote`, `evt.Demote` slices, saves one `group_membership` record per affected member. Called from `internal/whatsapp/events.go` as `go recordGroupMembership(evt)`.

### REST API

| Method | Path | Query Params | Description |
|--------|------|--------------|-------------|
| `GET` | `/zaplab/api/groups/{jid}/history` | `limit`, `offset` | History for one group |
| `GET` | `/zaplab/api/groups/membership` | `action`, `jid`, `limit`, `offset` | All membership events |

**File:** `internal/api/groupmembership.go`

### Frontend

Section ID: `group-membership` | JS file: `pb_public/js/sections/groupmembership.js` | State prefix: `gmt`

Key state: `gmtRows`, `gmtTotal`, `gmtActionFilter`, `gmtJIDFilter`, `gmtHistoryJID`, `gmtHistoryRows`

Key methods: `initGroupMembership()`, `loadGroupMembership()`, `loadGroupHistory(jid)`, `gmtFiltered()`, `gmtActionBadge(action)`

---

## 7. Message Secret Inspector

Implemented as the **Secrets** tab within the WA Health Monitor section. See §3 above for API and frontend details.

---

## 8. Audit Log

### Migration

File: `migrations/1747320000_create_audit_log.go`

Collection: `audit_log` (ID: `au1a2b3c4d5e6f7g`)

Fields:
- `method` — TextField
- `path` — TextField
- `status_code` — TextField
- `remote_ip` — TextField
- `request_body` — TextField (up to 64 KB)

### Middleware

File: `internal/api/auditlog.go`

`auditMiddleware()` returns a `hook.Handler[*core.RequestEvent]` that:
1. Reads and buffers the request body (cap 64 KB).
2. Restores the body with `io.NopCloser(bytes.NewReader(buf))` so downstream handlers can still read it.
3. After `e.Next()`, saves an `audit_log` record asynchronously.

Applied in `internal/api/api.go` to:
- `POST /sendmessage`
- `POST /zaplab/api/scripts/{id}/run`
- `POST /zaplab/api/scripts/run`
- `POST /zaplab/api/scripts/import`

### REST API

**Endpoint:** `GET /zaplab/api/audit`

Query params: `method`, `path`, `limit` (default 100), `offset`

**File:** `internal/api/auditlog.go`

**Response:**
```json
{
  "entries": [
    {
      "id": "...",
      "method": "POST",
      "path": "/sendmessage",
      "status_code": "",
      "remote_ip": "127.0.0.1",
      "request_body": "{\"to\":\"...\",\"message\":\"hello\"}",
      "created": "2026-03-17T12:00:00Z"
    }
  ],
  "total": 12
}
```

### Frontend

Section ID: `audit-log` | JS file: `pb_public/js/sections/auditlog.js` | State prefix: `alog`

Key state: `alogEntries`, `alogTotal`, `alogMethodFilter`, `alogPathFilter`, `alogLoading`

Key methods: `initAuditLog()`, `loadAuditLog()`, `alogMethodBadge(method)`, `alogPathShort(path)`

---

## Bug Fixes in This Session

### Frame Analyzers — `no such column: direction`

**Root cause:** `framesiq.go` was filtering on a `direction` column that does not exist in the `frames` PocketBase collection (actual columns: `id`, `module`, `level`, `seq`, `msg`, `created`, `updated`).

**Fix:** Removed all references to `direction`; replaced with `level` filter using `dbx.HashExp{"level": ...}`. IQ filter now uses `msg LIKE '%<iq%'`. Frontend dropdowns and column references updated to use `fiq.iqLevel` / `fiq.binLevel`.

### DB Sandbox — duplicate `LIMIT` syntax error

**Root cause:** `postDBQuery` in `internal/api/dbexplorer.go` was appending ` LIMIT 1000` unconditionally, even when the user's query already had a `LIMIT` clause.

**Fix:** Check `strings.Contains(strings.ToUpper(stmt), " LIMIT ")` before appending.

### DB Sandbox — `no such column: version`

**Root cause:** The Sessions quick-access example query referenced a `version` column in `whatsmeow_sessions`. That column does not exist; the table has only `our_jid`, `their_id`, `session`.

**Fix:** Removed `version` from the example query in `pb_public/js/sections/dbsandbox.js`.
