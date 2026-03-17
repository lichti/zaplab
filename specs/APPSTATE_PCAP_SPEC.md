# APPSTATE_PCAP_SPEC — App State Inspector + PCAP Export

## Overview

Two complementary research tools shipped together on branch `feature/appstate-pcap`:

1. **App State Inspector** — read-only display of the three whatsmeow app state SQLite tables, allowing researchers to examine collection versions, decryption key metadata, and per-mutation HMAC integrity codes.
2. **PCAP Export** — export Frame Capture log entries as a standard libpcap (`.pcap`) file that can be opened directly in Wireshark or `tcpdump` for offline analysis.

---

## 1. App State Inspector

### Background

WhatsApp synchronises device state (contacts, settings, starred messages, labels, blocked contacts) via an encrypted, versioned protocol called "app state sync". The client maintains three SQLite tables:

| Table | Purpose |
|-------|---------|
| `whatsmeow_app_state_version` | Current version index + Merkle-tree hash per collection |
| `whatsmeow_app_state_sync_keys` | Symmetric AES-256 keys for decrypting patches from the server |
| `whatsmeow_app_state_mutation_macs` | Per-leaf HMAC integrity codes in the Merkle tree |

**Known collection names** (used as the `name` primary key):

| Name | Sensitivity | Description |
|------|-------------|-------------|
| `critical` | High | Privacy settings, PIN, security configuration |
| `regular` | Medium | Contacts, starred messages, chats, labels |
| `critical_unblock_to_primary` | High | Secondary critical settings synced to primary device |
| `critical_block` | Medium | Blocked contacts and spam reports |
| `regular_low` | Low | Low-priority preferences |

---

### Backend — `internal/api/appstate.go`

All three endpoints use the `waDB` read-only SQL connection (`*sql.DB`) opened by `initDBExplorer`. Returns HTTP 503 if `waDB == nil` (DB Explorer not initialised or non-SQLite dialect).

#### `GET /zaplab/api/appstate/collections`

Reads `whatsmeow_app_state_version`.

Response:
```json
{
  "collections": [
    {
      "jid":      "5511999999999@s.whatsapp.net",
      "name":     "critical",
      "version":  142,
      "hash_hex": "a1b2c3d4e5f6..."
    }
  ],
  "total": 5
}
```

SQL:
```sql
SELECT jid, name, version, hash FROM whatsmeow_app_state_version ORDER BY jid, name
```

The `hash` BLOB is hex-encoded before returning.

#### `GET /zaplab/api/appstate/synckeys`

Reads `whatsmeow_app_state_sync_keys`. Raw `key_data` bytes are withheld (only `data_len_bytes` is returned).

Response:
```json
{
  "sync_keys": [
    {
      "jid":             "5511999999999@s.whatsapp.net",
      "key_id_hex":      "01020304",
      "timestamp":       1700000000,
      "fingerprint_hex": "deadbeef...",
      "data_len_bytes":  32
    }
  ],
  "total": 3
}
```

SQL:
```sql
SELECT jid, key_id, key_data, timestamp, fingerprint FROM whatsmeow_app_state_sync_keys ORDER BY jid, key_id
```

#### `GET /zaplab/api/appstate/mutations?collection=<name>&limit=<n>`

Reads `whatsmeow_app_state_mutation_macs` for the specified collection, ordered by version descending.

Query params:
- `collection` string — required; validated/sanitised before SQL interpolation
- `limit` int — max rows (default 100, max 500)

Response:
```json
{
  "mutations": [
    {
      "jid":           "5511999999999@s.whatsapp.net",
      "name":          "regular",
      "version":       88,
      "index_mac_hex": "aabb...",
      "value_mac_hex": "ccdd..."
    }
  ],
  "collection": "regular",
  "total":      100,
  "limit":      100
}
```

SQL injection prevention: `sanitizeSQL()` (defined in `stats.go`) doubles single quotes before string interpolation. Limit is `%d`-formatted (integer only).

---

### Frontend — `pb_public/js/sections/appstate.js`

Factory `appStateSection()` merged into `zaplab()` via `Object.assign`.

#### State

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `asLoading` | bool | false | Collections + sync keys loading |
| `asError` | string | '' | Last error message |
| `asTab` | string | `'collections'` | Active tab: `collections` / `synckeys` / `mutations` |
| `asCollections` | array | [] | Collection version rows |
| `asColFilter` | string | '' | Client-side filter for collections |
| `asSyncKeys` | array | [] | Sync key rows |
| `asMutCollection` | string | `'critical'` | Selected collection for mutations tab |
| `asMutCollections` | array | 5 items | Dropdown options |
| `asMutations` | array | [] | Mutation MAC rows |
| `asMutLimit` | int | 100 | Selected row limit |
| `asMutLoading` | bool | false | Mutations fetch indicator |

#### Key Methods

**`asLoad()`** — parallel fetch of `/appstate/collections` and `/appstate/synckeys`.

**`asLoadMutations()`** — fetch `/appstate/mutations?collection=<asMutCollection>&limit=<asMutLimit>`.

**`asFilteredCollections()`** — client-side filter on `name` and `jid` fields.

**`asCollectionIcon(name)`** — returns emoji per collection name.

**`asCollectionDesc(name)`** — returns plain-English description.

**`asCollectionBadgeClass(name)`** — Tailwind color class per collection criticality.

**`asTsLabel(ts)`** — converts Unix timestamp (seconds) to `toLocaleString()`.

**`asHexShort(hex)`** — truncates long hex strings to `first8…last8`.

---

### UI Layout

- **Tab bar**: Collections | Sync Keys | Mutations
- **Collections tab**: responsive 2–3 column card grid; each card shows icon, name badge, description, version counter, hash (truncated with full value in title tooltip), and JID
- **Sync Keys tab**: sortable table with key ID, timestamp, fingerprint (truncated), data size, JID
- **Mutations tab**: collection selector dropdown + limit selector; table with version, index MAC, value MAC, JID

---

## 2. PCAP Export

### Background

The `frames` PocketBase collection stores whatsmeow log entries (module, level, seq, msg, created). Exporting these as a PCAP file allows:

- Time-based filtering in Wireshark
- Side-by-side display with real Wireshark captures
- Scripted post-processing with `tshark`, `scapy`, or `pyshark`
- Correlation with packet-level traces from a parallel MitM setup

### File Format

Standard libpcap format (`pcap` not `pcapng`):

```
┌──────────────────────────────────────┐
│ Global Header (24 bytes)             │
│   magic     = 0xa1b2c3d4 (LE, µs)   │
│   ver_major = 2                      │
│   ver_minor = 4                      │
│   thiszone  = 0 (UTC)               │
│   sigfigs   = 0                      │
│   snaplen   = 65535                  │
│   network   = 1 (LINKTYPE_ETHERNET)  │
├──────────────────────────────────────┤
│ Per-packet Record (16 + N bytes)     │
│   ts_sec  = Unix seconds             │
│   ts_usec = microsecond fraction     │
│   incl_len = orig_len = N            │
│   [packet data]                      │
└──────────────────────────────────────┘
```

### Packet Structure (per frame entry)

Each packet is a complete Ethernet/IPv4/UDP frame:

```
Ethernet header (14 bytes)
  dst MAC  52:4c:41:42:00:02  (ASCII "RLAB\x00\x02")
  src MAC  52:4c:41:42:00:01
  EtherType 0x0800 (IPv4)

IPv4 header (20 bytes)
  version=4, IHL=5, TTL=64, protocol=17 (UDP)
  src IP  127.0.0.1
  dst IP  127.0.0.2
  (checksum disabled — Wireshark accepts)

UDP header (8 bytes)
  src port  443   (WhatsApp WebSocket)
  dst port  12345
  (checksum disabled)

UDP payload (variable)
  JSON: {"module":"...","level":"...","seq":"...","msg":"...","created":"..."}
```

The original `created` timestamp from PocketBase is used as the packet timestamp. Multiple timestamp formats are tried during parsing: `2006-01-02 15:04:05.999Z07:00`, `2006-01-02 15:04:05Z`, `time.RFC3339`, etc.

---

### Backend — `internal/api/pcap.go`

#### `GET /zaplab/api/frames/pcap?module=&level=&limit=`

Query params:
- `module` string — optional substring filter on `module` field
- `level` string — optional exact filter: `DEBUG`, `INFO`, `WARN`, `ERROR`
- `limit` int — max entries (default 1000, max 10000)

Response headers:
```
Content-Type: application/vnd.tcpdump.pcap
Content-Disposition: attachment; filename="zaplab_frames_20260316_142305.pcap"
Content-Length: <bytes>
```

SQL (on PocketBase's own SQLite via `pb.DB().NewQuery()`):
```sql
SELECT module, level, seq, msg, created FROM frames WHERE 1=1 [AND ...] ORDER BY created ASC LIMIT ?
```

Key functions:
- `writePCAPGlobalHeader(*bytes.Buffer)` — writes 24-byte global header
- `writePCAPPacket(*bytes.Buffer, time.Time, []byte)` — writes 16-byte record header + full Ethernet/IP/UDP frame
- `parsePCAPTime(string) time.Time` — tries multiple PocketBase timestamp formats

---

### Frontend — Frame Capture Toolbar

`fcExportPCAP()` added to `frameCaptureSection()`:

1. Builds query params from current `fcModuleFilter`, `fcLevelFilter`, `fcSearch`
2. `fetch()` with `apiHeaders()` (Bearer token)
3. Converts response to `Blob` → `createObjectURL`
4. Triggers download via a programmatically clicked `<a>` element
5. Revokes object URL after click

Button added to the Frame Capture toolbar (after Refresh), labelled **PCAP** with a download icon. Tooltip: "Export DB frames as PCAP (Wireshark)".

---

## API Routes Registered (`internal/api/api.go`)

```go
// App State Inspector
e.Router.GET("/zaplab/api/appstate/collections", getAppStateCollections).Bind(auth)
e.Router.GET("/zaplab/api/appstate/synckeys",    getAppStateSyncKeys).Bind(auth)
e.Router.GET("/zaplab/api/appstate/mutations",   getAppStateMutations).Bind(auth)

// PCAP export
e.Router.GET("/zaplab/api/frames/pcap", getFramesPCAP).Bind(auth)
```

---

## Security Notes

- App State Inspector is **read-only** — no write path exists.
- Raw `key_data` bytes from `whatsmeow_app_state_sync_keys` are withheld; only `data_len_bytes` is returned.
- `getAppStateMutations` user-supplied `collection` param is sanitised via `sanitizeSQL()` before string interpolation.
- All module/level params in `getFramesPCAP` are sanitised before interpolation; `limit` is `%d`-formatted.
- PCAP endpoint requires the standard `auth` middleware (PocketBase session or `X-API-Token` header).
