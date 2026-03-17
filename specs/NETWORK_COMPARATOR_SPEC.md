# NETWORK_COMPARATOR_SPEC — Multi-Session Comparator + Network Graph

## Overview

Two research-oriented visualisation tools shipped together on branch `feature/network-comparator`:

1. **Multi-Session Comparator** — select up to 6 Signal Protocol Double Ratchet sessions (or group SenderKey records) and compare their decoded properties side by side, with per-property difference highlighting.
2. **Network Graph** — interactive force-directed graph of the WhatsApp contact/group network derived from stored `Message` events, rendered on an HTML `<canvas>` element with a pure-JS physics simulation.

---

## 1. Multi-Session Comparator

### Design

The comparator is a pure-frontend feature — no new backend endpoints are required. It reuses the two existing signal session endpoints:

- `GET /zaplab/api/signal/sessions` → `{sessions: [{address, version, has_sender_chain, sender_counter, receiver_chains, previous_counter, remote_identity, local_identity, previous_states, raw_size_bytes, decode_error?}], total}`
- `GET /zaplab/api/signal/senderkeys` → `{sender_keys: [{chat_id, sender_id, key_id, iteration, signing_key, raw_size_bytes, decode_error?}], total}`

### Frontend — `pb_public/js/sections/sessioncomparator.js`

Factory `sessionComparatorSection()` merged via `Object.assign`.

#### State

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `mcLoading` | bool | false | Fetch in progress |
| `mcError` | string | '' | Last error |
| `mcSessions` | array | [] | All Double Ratchet sessions |
| `mcSenderKeys` | array | [] | All group SenderKey records |
| `mcSelected` | array | [] | Ordered list of selected IDs (max 6) |
| `mcTab` | string | `'sessions'` | Active tab: `sessions` \| `senderkeys` |
| `mcFilter` | string | '' | Client-side JID filter |

#### Session ID

For sessions: `session.address` string.
For sender keys: `k.chat_id + '|' + k.sender_id` composite string.

#### Key Methods

**`mcToggle(id)`** — toggles selection; enforces max-6 limit by blocking adds when full.

**`mcFiltered()`** — returns filtered list for the active tab.

**`mcSelectedSessions()`** — returns session objects in selection order.

**`mcSessionRows()`** — returns property descriptor array: `[{label, key, fmt}]` for 10 properties.

**`mcSenderKeyRows()`** — same for 5 SenderKey properties.

**`mcIsDiff(items, key, idx)`** — `true` if `String(items[idx][key]) !== String(items[0][key])`.

**`mcDiffCount(items, rows)`** — counts distinct property keys that differ across any item vs. reference.

#### Comparison Table

The comparison table has:
- **Rows** = properties (`mcSessionRows()` / `mcSenderKeyRows()`)
- **Columns** = selected sessions in selection order
- First column header is blue (reference); others are gray
- Cells where `mcIsDiff(sessions, row.key, idx)` is `true` receive amber background + bold text
- Diff count badge: green when 0, amber when > 0

#### Compared Session Properties

| Property | Description |
|----------|-------------|
| Version | Protocol version (v2 / v3 / v4) |
| Has Sender Chain | Whether the ratchet has a sender chain active |
| Sender Counter | Number of messages sent on current sender chain |
| Receiver Chains | Number of active receiver ratchet chains |
| Previous Counter | Sender counter when chain was last replaced |
| Previous States | Number of archived previous session states |
| Raw Size | Serialized blob size in bytes |
| Remote Identity | First 16 hex chars of remote public identity key |
| Local Identity | First 16 hex chars of local public identity key |
| Decode Error | Error message if decoding failed |

---

## 2. Network Graph

### Backend — `internal/api/network.go`

#### `GET /zaplab/api/network/graph?period=<days>&limit=<n>`

Scans `events WHERE type='Message'` in PocketBase's SQLite, parses each event's `raw` JSON column in Go, and aggregates a node/edge graph.

Query params:
- `period` int — days to look back (default 30, max 365; 0 = all time)
- `limit` int — max Message events to scan (default 2000, max 10000)

**Raw JSON parsing** — each message event `raw` column is unmarshalled into:
```go
type msgInfo struct {
    Info struct {
        Chat     string `json:"Chat"`     // types.JID → string via MarshalText
        Sender   string `json:"Sender"`
        IsFromMe bool   `json:"IsFromMe"`
        IsGroup  bool   `json:"IsGroup"`
    } `json:"Info"`
}
```

**Node type detection** by JID suffix:
- `@g.us` → `"group"`
- `@broadcast` → `"broadcast"`
- otherwise → `"contact"`

**Self JID** — queried from `whatsmeow_device LIMIT 1` via `waDB`; falls back to string `"self"` if `waDB == nil`.

**Label enrichment** — `whatsmeow_contacts` is queried for `push_name` / `full_name`; fallback is the user part of the JID (before `@`).

**Edges**:
- `self ↔ chat` — one edge per message (weight = message count after aggregation)
- `sender ↔ group` — one edge per group message from a non-self sender (captures membership)

**Top-N trimming** — nodes sorted by `msg_count` descending (self always first), capped at 100. Edges where either endpoint is not in the kept set are discarded.

**Response**:
```json
{
  "nodes": [
    {"id": "5511...@s.whatsapp.net", "label": "Alice", "node_type": "contact", "msg_count": 42},
    ...
  ],
  "edges": [
    {"source": "self", "target": "5511...@s.whatsapp.net", "weight": 42},
    ...
  ],
  "period": 30,
  "total_messages": 1200,
  "total_nodes": 37,
  "total_edges": 55
}
```

---

### Frontend — `pb_public/js/sections/networkgraph.js`

Factory `networkGraphSection()` merged via `Object.assign`.

#### State

| Field | Default | Description |
|-------|---------|-------------|
| `ngNodes` | [] | Node array with position + velocity state appended |
| `ngEdges` | [] | Edge array |
| `ngNodeMap` | {} | `id → node` lookup for O(1) edge resolution |
| `ngPeriod` | 30 | Active period (days) |
| `ngSelected` | null | Last clicked node (for detail panel) |
| `ngHovered` | null | Node under mouse cursor |
| `ngDragging` | null | Node being dragged |
| `ngAnimating` | bool | Simulation running |
| `ngW / ngH` | 800/550 | Canvas logical dimensions |

#### Physics Simulation

Each tick (`_ngTick()`):

1. **Repulsion** (Coulomb-like, all N² pairs):
   ```
   F = k_rep / d²  applied along unit vector
   k_rep = 9000
   ```

2. **Spring attraction** (along edges):
   ```
   F = k_spring × (d − rest_len)
   k_spring = 0.025, rest_len = 130px
   ```

3. **Centre gravity** (weak pull to canvas centre):
   ```
   F = k_grav × (centre − pos)
   k_grav = 0.012
   ```

4. **Verlet integration** with damping:
   ```
   vx' = (vx + fx) × damping
   damping = 0.82
   ```

5. **Boundary clamping** — nodes stay within canvas bounds padded by their radius.

Simulation runs at `requestAnimationFrame` speed for the first 200 ticks, then throttles to ~10 fps (80 ms interval) to reduce CPU usage while still responding to drag interactions.

#### Node Sizing

```
radius(self)    = 14
radius(group)   = clamp(7, 20, 7 + √msg_count × 0.9)
radius(contact) = clamp(5, 16, 5 + √msg_count × 0.6)
```

#### Node Colours (dark / light)

| Type | Dark | Light |
|------|------|-------|
| self | `#58a6ff` | `#0550ae` |
| contact | `#ffa657` | `#953800` |
| group | `#3fb950` | `#116329` |
| broadcast | `#bc8cff` | `#6639ba` |

#### Edge Rendering

```
alpha = clamp(0.08, 0.70, 0.08 + weight / 60)
lineWidth = clamp(0.4, 4.0, 0.4 + weight / 15)
```

#### Label Display

Labels are rendered below each node (10px system font). Only shown when:
- Node type is `self` (always)
- Node is hovered or selected
- `msg_count >= 5`

#### Mouse Events

| Event | Behaviour |
|-------|-----------|
| `mousedown` on node | Pin node, set as `ngSelected`, start drag |
| `mousedown` on background | Deselect |
| `mousemove` dragging | Update node position, re-render |
| `mousemove` not dragging | Update `ngHovered`, re-render on change |
| `mouseup` | Unpin dragged node (except self), clear drag |
| `mouseleave` | Clear drag and hover, re-render |

#### Detail Panel

Right-side panel shows the selected node's:
- Type badge with colour dot
- Label and full JID
- Message count
- All connected edges with peer label and weight

---

## API Routes Registered (`internal/api/api.go`)

```go
// Network graph
e.Router.GET("/zaplab/api/network/graph", getNetworkGraph).Bind(auth)
```

---

## Security Notes

- `getNetworkGraph` uses `%d`-formatted `period` and `limit` (integer only) in the SQL string.
- No user-supplied string data is interpolated into SQL.
- All data is read-only; the endpoint only queries the PocketBase `events` table and (optionally) `whatsmeow_contacts` via `waDB`.
- `waDB` contact name query uses no user input — unconditional `SELECT` with no parameters.
