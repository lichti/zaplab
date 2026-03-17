# Protocol Tools Specification

## Overview

Two complementary research sections for studying the WhatsApp Web protocol:

1. **Protocol Timeline** — chronological visual timeline of all protocol events
2. **Proto Schema Browser** — interactive browser of the full WhatsApp protobuf schema

---

## 1. Protocol Timeline

### Purpose

A dedicated, visual-first view of the `events` PocketBase collection. Where the existing "Live Events" and "Event Browser" sections are optimised for raw data inspection, the Protocol Timeline focuses on **temporal and categorical analysis** of the protocol flow: what happened, in what order, and of what type.

### Architecture

#### Data Source

Reads from the same PocketBase `events` collection used by the Live Events and Event Browser sections:

```
GET /api/collections/events/records
  sort=-created, perPage=200
```

Real-time updates come from the PocketBase realtime subscription:

```js
pb.collection('events').subscribe('*', handler)
```

#### Frontend state (`protocoltimeline.js`)

| Property | Type | Description |
|---|---|---|
| `ptEvents` | `array` | Loaded and live events (max `ptMaxItems = 200`) |
| `ptSelected` | `object \| null` | Currently expanded event |
| `ptFilter` | `string` | Free-text search filter |
| `ptTypeFilter` | `string` | Event type dropdown filter |
| `ptPaused` | `boolean` | When true, new live events are not appended |
| `ptConnStatus` | `string` | `connecting \| connected \| disconnected` |
| `ptSubscription` | `boolean` | Guard flag to prevent duplicate subscriptions |
| `ptMaxItems` | `number` | Maximum events kept in memory (default 200) |

#### Event type colour palette

| Event type | Colour | Notes |
|---|---|---|
| Message | Blue | Incoming/outgoing messages |
| Receipt | Green | Delivery/read receipts |
| Presence | Yellow | Online/offline/typing |
| HistorySync | Purple | Historical message sync |
| AppStateSyncComplete | Indigo | App state patch applied |
| Connected | Green | WebSocket connection established |
| Disconnected | Red | Connection dropped |
| LoggedOut | Red | Session invalidated |
| QR | Orange | QR code generated |
| PairSuccess | Green | Companion device paired |
| StreamReplaced | Orange | Session replaced by another client |
| KeepAliveTimeout | Yellow | Keepalive check failed |
| CallOffer | Pink | Incoming call |
| GroupInfo | Teal | Group metadata change |
| Contact | Cyan | Contact info update |
| NewsletterMessage | Violet | Newsletter (channel) message |
| *(other)* | Gray | Unrecognised event type |

#### Protocol summary extraction

For each event type, a concise human-readable summary is extracted from the payload:

| Type | Summary fields |
|---|---|
| Message | `Info.Sender` or `Info.Chat` + message body (first 60 chars) |
| Receipt | `SourceString` + `Type` |
| Presence | `From` + `State` |
| HistorySync | Conversation count + `syncType` |
| Connected | Static string |
| Disconnected | `Err` field or "stream closed" |
| LoggedOut | "on reconnect" or "during session" |
| CallOffer | `From` JID |
| GroupInfo | `JID` + `Type` |

### UI Layout

```
┌─ Header bar ──────────────────────────────────────────────────────────┐
│  "Protocol Timeline"   [status] [N/M events]                          │
│  [Type dropdown]  [Search…]  [Pause/Resume]  [Clear]                  │
└───────────────────────────────────────────────────────────────────────┘
┌─ Timeline list (full width or left half) ─┬─ Detail panel (right) ───┐
│                                            │  [Type badge] [time] [Copy]│
│  ● Message  "from +55…" — "Hello"  14:23  │  {                       │
│  │                                         │    "id": "...",          │
│  ● Receipt  read from +55…  14:22         │    "type": "Message",    │
│  │                                         │    "data": { ... }       │
│  ● Connected  WebSocket connected  14:20  │  }                       │
│  │                                         │                          │
└───────────────────────────────────────────┴──────────────────────────┘
```

- Vertical connector line runs through all dot markers
- Each dot pulses (`animate-ping`) for 2 seconds on new arrival
- Detail panel appears on the right on medium+ screens; replaces list on mobile
- Pause button freezes live appending without unsubscribing

---

## 2. Proto Schema Browser

### Purpose

Expose the complete WhatsApp protobuf schema embedded in the ZapLab binary for research use. Researchers can browse all message types, their field definitions, type references, oneof groups, and navigate between nested types interactively.

### Architecture

#### Schema registration

`internal/api/protoschema.go` blank-imports all 56 `go.mau.fi/whatsmeow/proto/*` packages. Each Go proto package registers its types in `protoregistry.GlobalTypes` via its `init()` function. This ensures 100% coverage of all WhatsApp proto definitions.

```go
import (
    _ "go.mau.fi/whatsmeow/proto/waE2E"
    _ "go.mau.fi/whatsmeow/proto/waHistorySync"
    // ... all 56 packages
)
```

#### Schema caching

Schema is built once using `sync.Once` on the first request and cached for the lifetime of the process. Building involves:

1. `protoregistry.GlobalTypes.RangeMessages()` — enumerate all message types
2. `protoregistry.GlobalTypes.RangeEnums()` — enumerate all enum types
3. For each message: extract fields, oneofs, nested message refs, nested enum refs
4. For each field: extract field number, name, kind, cardinality, and type ref (for message/enum kinds)
5. Sort all results alphabetically for deterministic output

#### API endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/zaplab/api/proto/schema` | Required | Full schema (all messages, enums, packages, stats) |
| `GET` | `/zaplab/api/proto/message?name=<FullName>` | Required | Single message descriptor by full name |

#### Response format — `GET /zaplab/api/proto/schema`

```json
{
  "messages": [
    {
      "full_name": "waE2E.Message",
      "package": "waE2E",
      "fields": [
        {
          "number": 1,
          "name": "conversation",
          "type": "string",
          "label": "optional",
          "type_ref": "",
          "oneof": ""
        },
        {
          "number": 2,
          "name": "senderKeyDistributionMessage",
          "type": "message",
          "label": "optional",
          "type_ref": "waE2E.SenderKeyDistributionMessage",
          "oneof": "message"
        }
      ],
      "oneofs": ["message"],
      "nested": ["waE2E.Message.DeviceSentMessage", "..."],
      "enums": ["waE2E.Message.MediaType"]
    }
  ],
  "enums": [
    {
      "full_name": "waE2E.Message.MediaType",
      "package": "waE2E",
      "values": [
        { "name": "UNKNOWN_MEDIA", "number": 0 },
        { "name": "IMAGE", "number": 1 }
      ]
    }
  ],
  "packages": ["armadilloutil", "waAdv", "waCommon", "waE2E", "waHistorySync", "..."],
  "stats": {
    "messages": 412,
    "enums": 289,
    "packages": 56
  }
}
```

#### Package extraction

Package name is derived from the first `.`-delimited component of the proto `FullName`:

```
"waE2E.Message.SubType" → package "waE2E"
```

Note: WhatsApp protos use Go package names (e.g., `waE2E`) as proto package names rather than the standard reverse-DNS convention.

### Frontend state (`protoschema.js`)

| Property | Type | Description |
|---|---|---|
| `psSchema` | `object \| null` | Cached schema from API |
| `psLoading` | `boolean` | Loading indicator |
| `psError` | `string` | Error message |
| `psPackageFilter` | `string` | Active package filter |
| `psSearch` | `string` | Free-text search |
| `psSelected` | `object \| null` | Currently viewed message/enum descriptor |
| `psSelectedKind` | `string` | `'message'` or `'enum'` |
| `psNavStack` | `array` | Navigation breadcrumb `[{kind, name}]` |
| `psView` | `string` | `'list'` or `'detail'` |

#### Navigation

- Clicking a message in the left list → `psSelectMessage(msg)` → pushes to `psNavStack`
- Clicking a clickable type reference in the fields table → `psNavigateTo(typeRef, kind)`:
  - Looks up the type in the local schema cache first
  - Falls back to `GET /zaplab/api/proto/message?name=<ref>` for nested types not in the top-level list
  - Pushes to `psNavStack`
- "Back" button → `psNavBack()` → pops `psNavStack` and restores the previous view

#### Type colour coding

| Kind | Colour |
|---|---|
| `string` | Green |
| `bytes` | Yellow |
| `bool` | Orange |
| `int*` / `uint*` / `fixed*` / `sint*` | Blue |
| `float` / `double` | Cyan |
| `message` / `group` | Purple (clickable) |
| `enum` | Pink (clickable) |

### UI Layout

```
┌─ Header bar ──────────────────────────────────────────────────────────┐
│  "Proto Schema Browser"   412 messages  289 enums  56 packages        │
│  [Reload]                                                             │
└───────────────────────────────────────────────────────────────────────┘
┌─ Left sidebar (w-72) ──────────┬─ Right detail panel ────────────────┐
│ [Search types…]                │  waE2E  message  Message             │
│ [Package ▼]                    │  3 fields  1 oneofs  4 nested  2 enums│
│                                │                                      │
│  MESSAGES (45)                 │  Fields                              │
│  ├ AudioMessage           (7f) │  ┌──┬──────────────────┬──────┬────┐ │
│  ├ ButtonsMessage         (23f)│  │ #│ Name             │ Type │ … │ │
│  ├ ExtendedTextMessage    (27f)│  ├──┼──────────────────┼──────┼────┤ │
│  ├ ImageMessage           (15f)│  │ 1│ conversation     │string│   │ │
│  └ Message               (80f)│  │ 2│ senderKeyDistri… │SenderK│  │ │
│                                │  └──┴──────────────────┴──────┴────┘ │
│  ENUMS (12)                    │                                      │
│  ├ Message.MediaType      (8v) │  Nested Messages                     │
│  └ Message.SubType        (3v) │  [DeviceSentMessage] […]             │
│                                │  Enums                               │
│                                │  [Message.MediaType] […]             │
└────────────────────────────────┴──────────────────────────────────────┘
```

---

## Security

Both endpoints require authentication (PocketBase JWT or `X-API-Token`). The schema endpoint is read-only and carries no sensitive runtime data — it exposes static type definitions compiled into the binary.

---

## Protocol Research Use Cases

### Protocol Timeline

- Observe the exact sequence of events during connection establishment (Noise handshake → `Connected` → `AppStateSyncComplete`)
- Identify timing patterns in receipt propagation (`Message` → `Receipt` with `delivered` → `Receipt` with `read`)
- Visualise session replacement (`StreamReplaced`) and reconnection events
- Monitor presence state transitions per JID

### Proto Schema Browser

- Understand the full `waE2E.Message` structure (80+ fields, 10+ oneof groups) without reading .proto files
- Navigate `waHistorySync.Conversation` and `waHistorySync.HistorySync` for history sync research
- Inspect `waCompanionReg.DeviceProps` to understand companion registration payload structure
- Browse `waSyncAction.*` types for app state patch decoding
- Cross-reference field numbers between captured binary frames and the schema for manual protobuf decoding
