# Signal Session Visualizer & Event Annotations — Specification

## Overview

Two research tools that expose the internal cryptographic state of a live WhatsApp session and allow researchers to attach notes to protocol events.

---

## 1. Signal Session Visualizer

### Purpose

Decodes and displays the Double Ratchet session states (`whatsmeow_sessions`) and group SenderKey records (`whatsmeow_sender_keys`) stored in the whatsmeow SQLite database. Allows researchers to inspect chain counters, key IDs, and identity keys without leaving the ZapLab dashboard.

### Architecture

#### Backend (`internal/api/signalsessions.go`)

Uses the existing read-only `waDB` SQL connection from the DB Explorer to query raw blobs, then decodes them using:

- `go.mau.fi/libsignal/state/record.NewSessionFromBytes(raw, serializer, stateSerializer)` for individual sessions
- `go.mau.fi/libsignal/groups/state/record.NewSenderKeyFromBytes(raw, serializer, stateSerializer)` for group sender keys
- Both use `go.mau.fi/whatsmeow/store.SignalProtobufSerializer` (the same serializer the live whatsmeow stack uses)

Private key material is never present in these decoded structures (only public keys are stored in the session blobs).

#### API Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/zaplab/api/signal/sessions` | List all decoded Double Ratchet session states |
| `GET` | `/zaplab/api/signal/senderkeys` | List all decoded group SenderKey states |

Both require auth. Both return `503` if the DB Explorer SQL connection is not initialised (non-SQLite setups or DB not yet loaded).

#### `GET /zaplab/api/signal/sessions` response fields

| Field | Type | Description |
|---|---|---|
| `address` | string | `whatsmeow_sessions.their_id` — `"phone.0:device"` format |
| `version` | int | Session version (2 = legacy, 3 = current, 4 = multi-device) |
| `has_sender_chain` | bool | Whether this session has an active sender ratchet chain |
| `sender_counter` | uint32 | Messages sent in this ratchet chain (chain key index) |
| `receiver_chains` | int | Number of active receiver ratchet chains (max 5) |
| `previous_counter` | uint32 | Counter of the previous ratchet epoch |
| `remote_identity` | string | Remote party's identity public key (hex) |
| `local_identity` | string | Local device's identity public key (hex) |
| `previous_states` | int | Number of archived session states (max 40) |
| `raw_size_bytes` | int | Size of the serialized protobuf blob |
| `decode_error` | string | Set only when deserialization fails |

#### `GET /zaplab/api/signal/senderkeys` response fields

| Field | Type | Description |
|---|---|---|
| `chat_id` | string | Group JID (from `whatsmeow_sender_keys.chat_id`) |
| `sender_id` | string | Member JID (from `whatsmeow_sender_keys.sender_id`) |
| `key_id` | uint32 | SenderKey key ID (incremented on each new key distribution) |
| `iteration` | uint32 | Number of messages sent by this member in this group epoch |
| `signing_key` | string | ECDSA signing key public (hex) used to authenticate group messages |
| `raw_size_bytes` | int | Size of the serialized protobuf blob |
| `decode_error` | string | Set only when deserialization fails |

### Frontend (`pb_public/js/sections/signalsessions.js`)

Two-tab layout:

**Tab 1 — Individual Sessions**: Table view sorted by address. Health indicator (✓/⚠/✗), address, session version label, sender counter, receiver chain count, previous counter, archived states, remote identity key (truncated hex), raw blob size.

**Tab 2 — Group Sender Keys**: Table view sorted by chat ID then sender ID. Health indicator, chat ID, sender ID, key ID, chain iteration, signing key (truncated hex), raw blob size.

Both tabs share a JID filter input and a Refresh button.

---

## 2. Event Annotations

### Purpose

Provides a persistent, searchable notepad for attaching protocol research notes and tags to any WhatsApp event observed in ZapLab. Annotations can be created from the dedicated **Annotations** section or inline from the **Event Browser** (one click on the Annotate button pre-fills all context).

### Architecture

#### PocketBase Collection: `annotations`

| Field | Type | Notes |
|---|---|---|
| `event_id` | text | msgID or PocketBase record ID of the linked event |
| `event_type` | text | WhatsApp event type (e.g. "Message", "Receipt") |
| `jid` | text | JID context (sender, chat, or peer) |
| `note` | text | Free-form research note (required) |
| `tags` | JSON | Array of string tags |
| `created` | autodate | |
| `updated` | autodate | |

Indexes: `event_id`, `jid`, `created`.

All CRUD rules open to authenticated users (`col.ListRule = col.ViewRule = col.CreateRule = col.UpdateRule = col.DeleteRule = ""`).

#### API Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/zaplab/api/annotations` | List annotations (filterable by `event_id`, `jid`) |
| `POST` | `/zaplab/api/annotations` | Create a new annotation |
| `PATCH` | `/zaplab/api/annotations/{id}` | Update `note` and/or `tags` |
| `DELETE` | `/zaplab/api/annotations/{id}` | Delete annotation |

All require auth.

#### Frontend (`pb_public/js/sections/annotations.js`)

**Annotations section**: Paginated card list of all annotations. Each card shows:
- Event type badge, event ID (monospace), JID
- Multi-line note text
- Tag chips (color-coded by tag name hash)
- Created timestamp
- Edit (pencil) and delete (trash) action buttons

Filter bar: event_id and JID text inputs with server-side filtering on change.

Realtime updates via PocketBase `annotations` collection subscription (create/update/delete events are applied without full reload).

**Annotation modal editor** (`anEditOpen` state):
- Context fields (event ID, type, JID) — editable when creating, read-only display when editing
- Multi-line `<textarea>` for the note
- Tag input (comma-separated string, parsed into array on save)
- Error display, Save/Cancel buttons

**Event Browser integration**: The event detail header includes an **Annotate** button alongside Copy JSON. Clicking it opens the modal with `event_id`, `event_type`, and sender JID pre-filled from the selected event.
