# API Specification — ZapLab

Source: `internal/api/api.go` + `internal/whatsapp/commands.go`

---

## Authentication

All endpoints marked with **🔒 Auth** require the header:

```
X-API-Token: <token>
```

The token is read from the `API_TOKEN` environment variable at startup.
Alternatively, requests with a valid PocketBase auth session (JWT) are also accepted.

If `API_TOKEN` is not set, **X-API-Token authentication is disabled**, but PocketBase session auth still works.

Authentication failure response:
```
HTTP 401 Unauthorized
{ "message": "Invalid or missing API token" }
```

---

## Redirections

For convenience, the following root paths automatically redirect to the Web UI:

- `GET /` → `307 Temporary Redirect` → `/zaplab/tools/`
- `GET /zaplab` → `307 Temporary Redirect` → `/zaplab/tools/`

---

## Endpoints

### `GET /health`

Checks server status and WhatsApp connection. **Public** (no auth).

**Response 200 — WhatsApp connected:**
```json
{
  "pocketbase": "ok",
  "whatsapp": true
}
```

**Response 503 — WhatsApp disconnected:**
```json
{
  "pocketbase": "ok",
  "whatsapp": false
}
```

---

### `GET /wa/status`

Returns the current WhatsApp connection state and the paired phone's JID. **Public** (no auth).

**Response 200:**
```json
{ "status": "connected", "jid": "5511999999999@s.whatsapp.net" }
```

| `status` | Meaning |
|---|---|
| `connecting` | Connecting to WhatsApp servers |
| `qr` | Waiting for QR scan — fetch `/wa/qrcode` |
| `connected` | Paired and online |
| `disconnected` | Connection lost, reconnect in progress |
| `timeout` | QR expired, new one coming |
| `loggedout` | Session logged out |

---

### `GET /wa/qrcode`

Returns the current QR code as a PNG image (base64 data URI). Only available when `status` is `qr`. **Public** (no auth).

**Response 200:**
```json
{ "status": "qr", "image": "data:image/png;base64,..." }
```

**Response 404 — no QR available:**
```json
{ "message": "no QR code available" }
```

---

### `POST /wa/logout` 🔒

Logs out and clears the WhatsApp session on the server. Restart the server to re-pair.

**Response 200:**
```json
{ "message": "logged out" }
```

---

### `GET /wa/account` 🔒

Returns the connected account details. Includes profile picture URL (WhatsApp CDN, may expire), push name, about text, and platform.

**Response 200:**
```json
{
  "jid":           "5511999999999@s.whatsapp.net",
  "phone":         "5511999999999",
  "push_name":     "John Doe",
  "business_name": "",
  "platform":      "android",
  "status":        "Hey there! I am using WhatsApp.",
  "avatar_url":    "https://mmg.whatsapp.net/..."
}
```

| Field | Description |
|---|---|
| `jid` | Full JID including server suffix |
| `phone` | Phone number digits only |
| `push_name` | Display name set on the device |
| `business_name` | Verified business name, empty for regular accounts |
| `platform` | Device platform reported to WhatsApp (`android`, `ios`, etc.) |
| `status` | About/status text |
| `avatar_url` | Direct CDN URL for the profile picture (empty if not set or unauthorized) |

**Response 503 — not connected:**
```json
{ "message": "not connected" }
```

---

### `GET /ping` 🔒

Tests authentication and API availability.

**Response 200:**
```json
{ "message": "Pong!" }
```

---

### `POST /sendmessage` 🔒

Sends a plain text message.

**Body:**
```json
{
  "to":      "5511999999999",
  "message": "Hello!"
}
```

| Field     | Type   | Required | Description                                    |
|-----------|--------|----------|------------------------------------------------|
| `to`      | string | yes      | Number (e.g. `5511999999999`) or full JID      |
| `message` | string | yes      | Message text                                   |

The `to` field accepts:
- Plain number: `5511999999999` → converted to `5511999999999@s.whatsapp.net`
- Full JID: `5511999999999@s.whatsapp.net`
- Group JID: `123456789@g.us`
- With `+`: `+5511999999999` (the `+` is stripped automatically)

**Generated WhatsApp Message:**
```json
{ "conversation": "Hello!" }
```

**Response 200:**
```json
{
  "message": "Message sent",
  "whatsapp_message": {
    "conversation": "Hello!"
  },
  "send_response": {
    "Timestamp": "2024-01-01T12:00:00Z",
    "ID":        "ABCD1234EFGH5678",
    "ServerID":  "",
    "Sender":    "5511999999999@s.whatsapp.net",
    "DebugTimings": { ... }
  }
}
```

**Response 400 — invalid JID:**
```json
{ "message": "To field is not a valid" }
```

**Response 500 — send failure:**
```json
{ "message": "Error to send message" }
```

---

### `POST /sendimage` 🔒

Sends an image. Body limit: **50 MB**.

**Body:**
```json
{
  "to":      "5511999999999",
  "message": "Optional caption",
  "image":   "<base64>"
}
```

| Field     | Type   | Required | Description              |
|-----------|--------|----------|--------------------------|
| `to`      | string | yes      | Number or JID            |
| `message` | string | no       | Image caption            |
| `image`   | string | yes      | Image content in base64  |

**Generated WhatsApp Message:**
```json
{
  "imageMessage": {
    "caption":       "Optional caption",
    "url":           "<computed after upload>",
    "directPath":    "<computed after upload>",
    "mediaKey":      "<computed after upload>",
    "mimetype":      "<detected from bytes>",
    "fileEncSha256": "<computed after upload>",
    "fileSha256":    "<computed after upload>",
    "fileLength":    "<computed after upload>"
  }
}
```

**Response 200:**
```json
{
  "message": "Image message sent",
  "whatsapp_message": {
    "imageMessage": {
      "caption": "Optional caption",
      "url": "https://mmg.whatsapp.net/...",
      "directPath": "/v/...",
      "mediaKey": "<base64>",
      "mimetype": "image/jpeg",
      "fileEncSha256": "<base64>",
      "fileSha256": "<base64>",
      "fileLength": 102400
    }
  },
  "send_response": {
    "Timestamp": "2024-01-01T12:00:00Z",
    "ID": "ABCD1234EFGH5678",
    "ServerID": "",
    "Sender": "5511999999999@s.whatsapp.net",
    "DebugTimings": { ... }
  }
}
```

**Response 400 — invalid base64:**
```json
{ "message": "Error to decode image" }
```

**Response 400 — invalid JID:**
```json
{ "message": "To field is not a valid" }
```

**Response 500 — upload or send failure:**
```json
{ "message": "Error to send image message" }
```

---

### `POST /sendvideo` 🔒

Sends a video. Body limit: **50 MB**.

**Body:**
```json
{
  "to":      "5511999999999",
  "message": "Optional caption",
  "video":   "<base64>"
}
```

| Field     | Type   | Required | Description              |
|-----------|--------|----------|--------------------------|
| `to`      | string | yes      | Number or JID            |
| `message` | string | no       | Video caption            |
| `video`   | string | yes      | Video content in base64  |

**Generated WhatsApp Message:**
```json
{
  "videoMessage": {
    "caption":       "Optional caption",
    "url":           "<computed after upload>",
    "directPath":    "<computed after upload>",
    "mediaKey":      "<computed after upload>",
    "mimetype":      "<detected from bytes>",
    "fileEncSha256": "<computed after upload>",
    "fileSha256":    "<computed after upload>",
    "fileLength":    "<computed after upload>"
  }
}
```

**Response 200:**
```json
{
  "message": "Video message sent",
  "whatsapp_message": { "videoMessage": { "caption": "...", "url": "...", "mimetype": "video/mp4", ... } },
  "send_response": { "Timestamp": "...", "ID": "...", "Sender": "...", ... }
}
```

**Errors:** analogous to `/sendimage`.

---

### `POST /sendaudio` 🔒

Sends audio. Body limit: **50 MB**.

**Body:**
```json
{
  "to":    "5511999999999",
  "audio": "<base64>",
  "ptt":   false
}
```

| Field   | Type    | Required | Description                                                     |
|---------|---------|----------|-----------------------------------------------------------------|
| `to`    | string  | yes      | Number or JID                                                   |
| `audio` | string  | yes      | Audio content in base64                                         |
| `ptt`   | boolean | no       | `true` = send as voice note (push-to-talk). Default: `false`    |

**Mimetype** detected via `mimetype.Detect(data)` + `"; codecs=opus"` (e.g. `audio/ogg; codecs=opus`).

**Generated WhatsApp Message:**
```json
{
  "audioMessage": {
    "url":               "<computed after upload>",
    "directPath":        "<computed after upload>",
    "mediaKey":          "<computed after upload>",
    "mimetype":          "<detected>; codecs=opus",
    "fileEncSha256":     "<computed after upload>",
    "fileSha256":        "<computed after upload>",
    "fileLength":        "<computed after upload>",
    "ptt":               false,
    "mediaKeyTimestamp": "<unix timestamp>"
  }
}
```

**Response 200:**
```json
{
  "message": "Audio message sent",
  "whatsapp_message": { "audioMessage": { "url": "...", "mimetype": "audio/ogg; codecs=opus", "ptt": false, ... } },
  "send_response": { "Timestamp": "...", "ID": "...", "Sender": "...", ... }
}
```

**Errors:** analogous to `/sendimage`.

---

### `POST /senddocument` 🔒

Sends a document/file. Body limit: **50 MB**.

**Body:**
```json
{
  "to":       "5511999999999",
  "message":  "Optional description",
  "document": "<base64>"
}
```

| Field      | Type   | Required | Description                    |
|------------|--------|----------|--------------------------------|
| `to`       | string | yes      | Number or JID                  |
| `message`  | string | no       | Document caption/description   |
| `document` | string | yes      | File content in base64         |

**Generated WhatsApp Message:**
```json
{
  "documentMessage": {
    "caption":       "Optional description",
    "url":           "<computed after upload>",
    "directPath":    "<computed after upload>",
    "mediaKey":      "<computed after upload>",
    "mimetype":      "<detected from bytes>",
    "fileEncSha256": "<computed after upload>",
    "fileSha256":    "<computed after upload>",
    "fileLength":    "<computed after upload>"
  }
}
```

**Response 200:**
```json
{
  "message": "Document message sent",
  "whatsapp_message": { "documentMessage": { "caption": "...", "url": "...", "mimetype": "application/pdf", ... } },
  "send_response": { "Timestamp": "...", "ID": "...", "Sender": "...", ... }
}
```

**Errors:** analogous to `/sendimage`.

---

### `POST /sendraw` 🔒

Sends an arbitrary `waE2E.Message` structure. No upload or encoding is performed — the message is sent exactly as provided.
Useful for protocol research and testing any message type defined in `WAWebProtobufsE2E.pb.go`.

See full spec: [`specs/SEND_RAW_SPEC.md`](./SEND_RAW_SPEC.md)

**Body:**
```json
{
  "to":      "5511999999999",
  "message": { "conversation": "Hello!" }
}
```

| Field     | Type   | Required | Description                                                    |
|-----------|--------|----------|----------------------------------------------------------------|
| `to`      | string | yes      | Number or JID                                                  |
| `message` | object | yes      | Protobuf-JSON encoded `waE2E.Message` (camelCase field names)  |

**Response 200:**
```json
{
  "message":          "Raw message sent",
  "whatsapp_message": { "conversation": "Hello!" },
  "send_response":    { "Timestamp": "...", "ID": "...", "Sender": "...", ... }
}
```

**Response 400 — protobuf unmarshal error:**
```json
{ "message": "invalid message JSON: ..." }
```

**Response 400 — invalid JID:**
```json
{ "message": "To field is not a valid" }
```

---

### `POST /sendlocation` 🔒

Sends a static GPS location pin.

**Body:**
```json
{
  "to":        "5511999999999",
  "latitude":  -23.5505,
  "longitude": -46.6333,
  "name":      "São Paulo",
  "address":   "Av. Paulista, 1000",
  "reply_to":  { "message_id": "...", "sender_jid": "...", "quoted_text": "..." }
}
```

| Field       | Type    | Required | Description                              |
|-------------|---------|----------|------------------------------------------|
| `to`        | string  | yes      | Number or JID                            |
| `latitude`  | float64 | yes      | Decimal degrees                          |
| `longitude` | float64 | yes      | Decimal degrees                          |
| `name`      | string  | no       | Location name shown on pin               |
| `address`   | string  | no       | Street address shown below the pin       |
| `reply_to`  | object  | no       | Quote a previous message (see below)     |

**Response 200:**
```json
{
  "message":          "Location sent",
  "whatsapp_message": { "locationMessage": { "degreesLatitude": -23.5505, "degreesLongitude": -46.6333, "name": "São Paulo", ... } },
  "send_response":    { "Timestamp": "...", "ID": "...", "Sender": "..." }
}
```

---

### `POST /sendelivelocation` 🔒

Sends a live GPS location update. Call repeatedly (incrementing `sequence_number`) to update the position on the receiver's map.

**Body:**
```json
{
  "to":                                      "5511999999999",
  "latitude":                                -23.5505,
  "longitude":                               -46.6333,
  "accuracy_in_meters":                      10,
  "speed_in_mps":                            1.4,
  "degrees_clockwise_from_magnetic_north":   90,
  "caption":                                 "Heading to the office",
  "sequence_number":                         1,
  "time_offset":                             0,
  "reply_to":                                { "message_id": "...", "sender_jid": "...", "quoted_text": "..." }
}
```

| Field                                     | Type    | Required | Description                          |
|-------------------------------------------|---------|----------|--------------------------------------|
| `to`                                      | string  | yes      | Number or JID                        |
| `latitude`                                | float64 | yes      | Current latitude                     |
| `longitude`                               | float64 | yes      | Current longitude                    |
| `accuracy_in_meters`                      | uint32  | no       | GPS accuracy (default 0)             |
| `speed_in_mps`                            | float32 | no       | Speed in metres per second           |
| `degrees_clockwise_from_magnetic_north`   | uint32  | no       | Heading / bearing                    |
| `caption`                                 | string  | no       | Text shown under the map             |
| `sequence_number`                         | int64   | no       | Increment each update (default 0)    |
| `time_offset`                             | uint32  | no       | Seconds since initial message        |
| `reply_to`                                | object  | no       | Quote a previous message             |

**Response 200:**
```json
{
  "message":          "Live location sent",
  "whatsapp_message": { "liveLocationMessage": { ... } },
  "send_response":    { "Timestamp": "...", "ID": "...", "Sender": "..." }
}
```

---

### `POST /setdisappearing` 🔒

Sets the disappearing (auto-delete) messages timer for a chat or group. WhatsApp only accepts the four official timer values.

**Body:**
```json
{
  "to":    "5511999999999",
  "timer": 86400
}
```

| Field   | Type   | Required | Description                                           |
|---------|--------|----------|-------------------------------------------------------|
| `to`    | string | yes      | Chat JID (number, full JID, or group JID)             |
| `timer` | uint32 | yes      | Seconds: `0` (off), `86400` (24h), `604800` (7d), `7776000` (90d) |

**Response 200:**
```json
{ "message": "Disappearing timer updated" }
```

**Response 400 — invalid timer value:**
```json
{ "message": "timer must be 0, 86400, 604800, or 7776000" }
```

---

### Reply support (`reply_to` field)

All send endpoints accept an optional `reply_to` field to quote a previous message:

```json
{
  "reply_to": {
    "message_id":  "ABCD1234EFGH5678",
    "sender_jid":  "5511999999999@s.whatsapp.net",
    "quoted_text": "Original message text shown in the bubble"
  }
}
```

| Field         | Type   | Required | Description                                         |
|---------------|--------|----------|-----------------------------------------------------|
| `message_id`  | string | yes      | ID of the message to quote                          |
| `sender_jid`  | string | yes      | JID of the original message author                  |
| `quoted_text` | string | no       | Text shown inside the reply bubble (simplified preview) |

When `reply_to` is present, text messages are sent as `ExtendedTextMessage` (with `ContextInfo`). Media messages have `ContextInfo` set on the media message directly.

---

### `POST /sendreaction` 🔒

Adds or removes an emoji reaction to a message.

**Body:**
```json
{
  "to":          "5511999999999",
  "message_id":  "ABCD1234EFGH5678",
  "sender_jid":  "5511999999999@s.whatsapp.net",
  "emoji":       "❤️"
}
```

| Field        | Type   | Required | Description                                              |
|--------------|--------|----------|----------------------------------------------------------|
| `to`         | string | yes      | Chat JID (number or full JID)                            |
| `message_id` | string | yes      | ID of the message to react to                            |
| `sender_jid` | string | yes      | JID of the original message author                       |
| `emoji`      | string | no       | Emoji string. Empty string removes the existing reaction |

**Response 200:**
```json
{
  "message":          "Reaction sent",
  "whatsapp_message": { "reactionMessage": { "key": { ... }, "text": "❤️", "senderTimestampMs": "..." } },
  "send_response":    { "Timestamp": "...", "ID": "...", "Sender": "..." }
}
```

**Response 400 — missing message_id / invalid JID:**
```json
{ "message": "message_id is required" }
```

---

### `POST /editmessage` 🔒

Edits a previously sent text message. Only messages sent by the bot itself can be edited.

> Note: WhatsApp allows editing within ~20 minutes of the original send (`EditWindow = 20 * time.Minute`).

**Body:**
```json
{
  "to":         "5511999999999",
  "message_id": "ABCD1234EFGH5678",
  "new_text":   "Updated message text"
}
```

| Field        | Type   | Required | Description                      |
|--------------|--------|----------|----------------------------------|
| `to`         | string | yes      | Chat JID (number or full JID)    |
| `message_id` | string | yes      | ID of the message to edit        |
| `new_text`   | string | yes      | Replacement text                 |

**Response 200:**
```json
{
  "message":          "Message edited",
  "whatsapp_message": { "editedMessage": { "message": { "protocolMessage": { ... } } } },
  "send_response":    { "Timestamp": "...", "ID": "...", "Sender": "..." }
}
```

---

### `POST /revokemessage` 🔒

Deletes a message for everyone (revoke). Group admins can revoke other members' messages.

**Body:**
```json
{
  "to":         "5511999999999",
  "message_id": "ABCD1234EFGH5678",
  "sender_jid": "5511999999999@s.whatsapp.net"
}
```

| Field        | Type   | Required | Description                                                              |
|--------------|--------|----------|--------------------------------------------------------------------------|
| `to`         | string | yes      | Chat JID (number or full JID)                                            |
| `message_id` | string | yes      | ID of the message to delete                                              |
| `sender_jid` | string | yes      | JID of the original message author. Use bot's own JID for own messages   |

**Response 200:**
```json
{
  "message":          "Message revoked",
  "whatsapp_message": { "protocolMessage": { "type": "REVOKE", "key": { ... } } },
  "send_response":    { "Timestamp": "...", "ID": "...", "Sender": "..." }
}
```

---

### `POST /settyping` 🔒

Sends a typing or voice-recording presence indicator to a chat. Call with `state: "paused"` to stop.

**Body:**
```json
{
  "to":    "5511999999999",
  "state": "composing",
  "media": "text"
}
```

| Field   | Type   | Required | Description                                                                      |
|---------|--------|----------|----------------------------------------------------------------------------------|
| `to`    | string | yes      | Chat JID (number or full JID)                                                    |
| `state` | string | yes      | `"composing"` (show indicator) or `"paused"` (stop indicator)                   |
| `media` | string | no       | `"text"` (typing, default) or `"audio"` (recording). Only applies to composing  |

**Response 200:**
```json
{ "message": "Typing state updated" }
```

**Response 400:**
```json
{ "message": "state must be 'composing' or 'paused'" }
```

---

### `POST /sendcontact` 🔒

Sends a single vCard contact.

**Body:**
```json
{
  "to":           "5511999999999",
  "display_name": "John Doe",
  "vcard":        "BEGIN:VCARD\nVERSION:3.0\nFN:John Doe\nTEL;TYPE=CELL:+5511999999999\nEND:VCARD",
  "reply_to":     { "message_id": "...", "sender_jid": "...", "quoted_text": "..." }
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `to` | string | yes | Recipient phone number or JID |
| `display_name` | string | no | Name shown in the chat bubble |
| `vcard` | string | yes | RFC 2426 vCard string |
| `reply_to` | object | no | Optional reply context (see [Reply support](#reply_to)) |

**Response 200:**
```json
{
  "message": "Contact sent",
  "whatsapp_message": { ... },
  "send_response": { ... }
}
```

---

### `POST /sendcontacts` 🔒

Sends multiple vCard contacts in a single message bubble.

**Body:**
```json
{
  "to":           "5511999999999",
  "display_name": "2 contacts",
  "contacts": [
    { "name": "Alice", "vcard": "BEGIN:VCARD\nVERSION:3.0\nFN:Alice\nTEL:+5511111111111\nEND:VCARD" },
    { "name": "Bob",   "vcard": "BEGIN:VCARD\nVERSION:3.0\nFN:Bob\nTEL:+5522222222222\nEND:VCARD" }
  ],
  "reply_to": { "message_id": "...", "sender_jid": "...", "quoted_text": "..." }
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `to` | string | yes | Recipient phone number or JID |
| `display_name` | string | no | Label for the contact group bubble |
| `contacts` | array | yes | Array of `{ name, vcard }` objects (min 1) |
| `reply_to` | object | no | Optional reply context |

**Response 200:**
```json
{
  "message": "Contacts sent",
  "whatsapp_message": { ... },
  "send_response": { ... }
}
```

---

### `POST /createpoll` 🔒

Creates a WhatsApp poll. The poll encryption key is handled internally by whatsmeow.

**Body:**
```json
{
  "to":               "5511999999999",
  "question":         "Favourite colour?",
  "options":          ["Blue", "Green", "Red"],
  "selectable_count": 1
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `to` | string | yes | Recipient phone number or JID |
| `question` | string | yes | Poll question text |
| `options` | array | yes | Option name strings (min 2, max 12) |
| `selectable_count` | int | no | Max options a voter can pick; `0` = unlimited (default: `1`) |

**Response 200:**
```json
{
  "message": "Poll created",
  "whatsapp_message": { ... },
  "send_response": { ... }
}
```

---

### `POST /votepoll` 🔒

Casts a vote on an existing poll. The `poll_message_id` and `poll_sender_jid` must match the original poll exactly for the vote to be accepted.

**Body:**
```json
{
  "to":               "5511999999999",
  "poll_message_id":  "ABCD1234EFGH5678",
  "poll_sender_jid":  "5511999999999@s.whatsapp.net",
  "selected_options": ["Blue", "Green"]
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `to` | string | yes | Chat JID (same conversation as the poll) |
| `poll_message_id` | string | yes | Message ID of the original poll |
| `poll_sender_jid` | string | yes | Full JID of whoever created the poll |
| `selected_options` | array | yes | Option name strings to vote for (must match exactly) |

**Response 200:**
```json
{
  "message": "Vote cast",
  "whatsapp_message": { ... },
  "send_response": { ... }
}
```

---

### `GET /groups` 🔒

Returns all groups the bot is currently a member of.

**Response 200:**
```json
{ "groups": [ { "JID": "123456789-000@g.us", "Name": "...", "Participants": [...], ... } ] }
```

---

### `GET /groups/{jid}` 🔒

Returns detailed info for a specific group. The `{jid}` segment must be URL-encoded.

**Response 200:** `types.GroupInfo` object

---

### `POST /groups` 🔒

Creates a new WhatsApp group.

**Body:**
```json
{
  "name": "My Group",
  "participants": ["5511999999999", "5511888888888"]
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Group name (max 25 characters) |
| `participants` | array | yes | Phone numbers or full JIDs |

**Response 200:**
```json
{ "message": "Group created", "group": { ... } }
```

---

### `POST /groups/{jid}/participants` 🔒

Adds, removes, promotes, or demotes participants.

**Body:**
```json
{ "action": "add", "participants": ["5511999999999"] }
```

`action`: `"add"` | `"remove"` | `"promote"` | `"demote"`

**Response 200:**
```json
{ "message": "Participants updated", "results": [...] }
```

---

### `PATCH /groups/{jid}` 🔒

Updates group settings. Include only the fields to change.

**Body:**
```json
{ "name": "New Name", "topic": "New desc", "announce": true, "locked": false }
```

| Field | Type | Description |
|---|---|---|
| `name` | string | New group name (max 25 chars) |
| `topic` | string | New group description |
| `announce` | bool | `true` = only admins can send |
| `locked` | bool | `true` = only admins can edit group info |

**Response 200:**
```json
{ "message": "Group updated" }
```

---

### `POST /groups/{jid}/leave` 🔒

Makes the bot leave the group.

**Response 200:**
```json
{ "message": "Left group" }
```

---

### `GET /groups/{jid}/invitelink` 🔒

Returns the invite link for the group. Add `?reset=true` to revoke and regenerate.

**Response 200:**
```json
{ "link": "https://chat.whatsapp.com/AbCdEf123456" }
```

---

### `POST /groups/join` 🔒

Joins a group using an invite link or code.

**Body:**
```json
{ "link": "https://chat.whatsapp.com/AbCdEf123456" }
```

Full URL or just the code suffix are both accepted.

**Response 200:**
```json
{ "message": "Joined group", "jid": "123456789-000@g.us" }
```

---

### `GET /groups/{jid}/participants` 🔒

Returns only the participant list for a group without full metadata (lighter than `GET /groups/{jid}`).

**Response 200:**
```json
{
  "jid": "123456789-000@g.us",
  "participants": [
    {
      "jid":            "5511999999999@s.whatsapp.net",
      "phone":          "5511999999999",
      "is_admin":       true,
      "is_super_admin": false
    }
  ]
}
```

---

### `POST /wa/qrtext` 🔒

Generates a QR Code PNG for any text string (e.g. an invite link).

**Body:**
```json
{ "text": "https://chat.whatsapp.com/AbCdEf123456" }
```

**Response 200:**
```json
{ "image": "data:image/png;base64,..." }
```

---

### `POST /cmd` 🔒

Dispatches internal bot commands. Returns the command output as a string.

**Body:**
```json
{
  "cmd":  "<command>",
  "args": "<arguments>"
}
```

| Field  | Type   | Required | Description                               |
|--------|--------|----------|-------------------------------------------|
| `cmd`  | string | yes      | Command name (see table below)            |
| `args` | string | yes      | Command arguments as a single string      |

**Response 200:**
```json
{ "message": "<command output>" }
```

---

#### Available commands

##### Webhook

| Command                     | Args                     | Description                                              |
|-----------------------------|--------------------------|----------------------------------------------------------|
| `set-default-webhook`       | `<url>`                  | Set the default webhook URL (all events)                 |
| `set-error-webhook`         | `<url>`                  | Set the error webhook URL                                |
| `add-cmd-webhook`           | `<cmd>\|<url>`           | Associate a bot command with a specific webhook          |
| `rm-cmd-webhook`            | `<cmd>`                  | Remove the webhook association for a bot command         |
| `print-cmd-webhooks-config` | —                        | Print the current webhook configuration to the log       |

Examples:
```bash
# Set default webhook
curl -X POST http://localhost:8090/cmd \
  -H "X-API-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"cmd":"set-default-webhook","args":"https://n8n.example.com/webhook/abc"}'

# Associate !hello command with a webhook
curl -X POST http://localhost:8090/cmd \
  -H "X-API-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"cmd":"add-cmd-webhook","args":"!hello|https://n8n.example.com/webhook/hello"}'
```

##### Groups

| Command      | Args      | Description                                       |
|--------------|-----------|---------------------------------------------------|
| `getgroup`   | `<jid>`   | Returns information about a group by JID          |
| `listgroups` | —         | Lists all groups the bot participates in          |

> Group JID format: `123456789-1234567890@g.us`

##### Advanced messages (spoofed)

| Command                              | Args                                                                            | Description                                               |
|--------------------------------------|---------------------------------------------------------------------------------|-----------------------------------------------------------|
| `send-spoofed-reply`                 | `<chat_jid> <msgID:\!│#ID> <spoofed_jid> <spoofed_text>\|<text>`               | Send reply spoofing sender and quoted text                |
| `sendSpoofedReplyMessageInPrivate`   | `<chat_jid> <msgID:\!│#ID> <spoofed_jid> <spoofed_text>\|<text>`               | Same, but in the recipient's private chat                 |
| `send-spoofed-img-reply`             | `<chat_jid> <msgID:\!│#ID> <spoofed_jid> <spoofed_file> <spoofed_text>\|<text>`| Spoofed reply with image                                  |
| `send-spoofed-demo`                  | `<boy\|girl> <br\|en> <chat_jid> <spoofed_jid>`                                 | Send spoofed conversation demo (text)                     |
| `send-spoofed-demo-img`              | `<boy\|girl> <br\|en> <chat_jid> <spoofed_jid> <spoofed_img>`                   | Send spoofed conversation demo (with image)               |
| `spoofed-reply-this`                 | `<chat_jid> <msgID:\!│#ID> <spoofed_jid> <text>`                                | Spoofed reply of the quoted message (requires reply)      |
| `SendTimedMsg`                       | `<chat_jid> <text>`                                                             | Send self-destructing message                             |
| `removeOldMsg`                       | `<chat_jid> <msgID>`                                                            | Delete a previously sent message                          |
| `editOldMsg`                         | `<chat_jid> <msgID> <newMSG>`                                                   | Edit a previously sent message                            |

> `msgID`: use `!` to generate a new ID automatically, or `#<ID>` to use a specific ID.

---

### `POST /media/download` 🔒

Downloads and decrypts a WhatsApp media file. Body limit: **50 MB**.

Returns the raw decrypted binary as a file download (not JSON).

**Body:**
```json
{
  "url":        "https://mmg.whatsapp.net/...",
  "media_key":  "<base64-encoded media key>",
  "media_type": "image"
}
```

| Field        | Type   | Required | Description                                                                            |
|--------------|--------|----------|----------------------------------------------------------------------------------------|
| `url`        | string | yes      | WhatsApp CDN URL (from `url` or `directPath` field of the media message)              |
| `media_key`  | string | yes      | Base64-encoded media key from the WhatsApp message protobuf                           |
| `media_type` | string | yes      | One of: `image`, `video`, `audio`, `document`, `sticker`                              |

**Response 200:**

Raw binary file with headers:
```
Content-Type: image/jpeg  (detected from decrypted bytes)
Content-Disposition: attachment; filename="media.jpg"
```

**Response 400 — missing field:**
```json
{ "message": "url is required" }
{ "message": "media_key is required" }
{ "message": "media_type is required (image, video, audio, document, sticker)" }
```

**Response 400 — decrypt error:**
```json
{ "message": "..." }
```

---

### `GET /contacts` 🔒

Returns all contacts stored in the local WhatsApp device store.

**Response 200:**
```json
{
  "contacts": [
    {
      "JID":          "5511999999999@s.whatsapp.net",
      "Found":        true,
      "FirstName":    "John",
      "FullName":     "John Doe",
      "PushName":     "Johnny",
      "BusinessName": ""
    }
  ],
  "count": 1
}
```

**Response 503 — not connected:**
```json
{ "message": "..." }
```

---

### `POST /contacts/check` 🔒

Checks whether a list of phone numbers are registered on WhatsApp.

**Body:**
```json
{
  "phones": ["5511999999999", "5522888888888"]
}
```

| Field    | Type     | Required | Description                               |
|----------|----------|----------|-------------------------------------------|
| `phones` | string[] | yes      | List of phone numbers (digits only or `+` prefix) |

**Response 200:**
```json
{
  "results": [
    { "Query": "5511999999999", "JID": "5511999999999@s.whatsapp.net", "IsIn": true, "VerifiedName": "" }
  ],
  "count": 1
}
```

**Response 400:**
```json
{ "message": "phones array is required" }
```

---

### `GET /contacts/{jid}` 🔒

Returns stored info for a specific contact. `{jid}` must be URL-encoded.

**Response 200:**
```json
{
  "JID":          "5511999999999@s.whatsapp.net",
  "Found":        true,
  "FirstName":    "John",
  "FullName":     "John Doe",
  "PushName":     "Johnny",
  "BusinessName": ""
}
```

**Response 400 — invalid JID:**
```json
{ "message": "Invalid JID" }
```

---

### `POST /spoof/reply` 🔒

Sends a spoofed reply — the quoted message shows a fake sender (`from_jid`) and fake content (`quoted_text`), but the outer message is sent by the bot.

**Body:**
```json
{
  "to":          "5511999999999",
  "from_jid":    "5533777777777@s.whatsapp.net",
  "msg_id":      "",
  "quoted_text": "Original message that never existed",
  "text":        "My reply to it"
}
```

| Field         | Type   | Required | Description                                                   |
|---------------|--------|----------|---------------------------------------------------------------|
| `to`          | string | yes      | Chat JID (number, full JID, or group JID)                     |
| `from_jid`    | string | yes      | JID that appears as the quoted message author (spoofed)       |
| `msg_id`      | string | no       | Message ID to embed in the `ContextInfo`; auto-generated if empty |
| `quoted_text` | string | no       | Text shown in the reply bubble preview (spoofed content)      |
| `text`        | string | no       | Actual message text sent by the bot                           |

**Response 200:**
```json
{
  "message":          "Spoofed reply sent",
  "whatsapp_message": { "extendedTextMessage": { ... } },
  "send_response":    { "Timestamp": "...", "ID": "...", "Sender": "..." }
}
```

---

### `POST /spoof/reply-private` 🔒

Same as `/spoof/reply` but the message is sent to the recipient's **private chat** (DM), even if the spoofed context references a group message. Useful for out-of-context spoofing in DMs.

**Body:** identical to `/spoof/reply`.

**Response 200:**
```json
{ "message": "Spoofed private reply sent", "whatsapp_message": { ... }, "send_response": { ... } }
```

---

### `POST /spoof/reply-img` 🔒

Sends a spoofed reply where the quoted bubble shows an image (uploaded to WhatsApp CDN) attributed to the fake sender. Body limit: **50 MB**.

**Body:**
```json
{
  "to":          "5511999999999",
  "from_jid":    "5533777777777@s.whatsapp.net",
  "msg_id":      "",
  "image":       "<base64-encoded image>",
  "quoted_text": "Caption shown inside the quoted bubble",
  "text":        "My reply text"
}
```

| Field         | Type   | Required | Description                                                    |
|---------------|--------|----------|----------------------------------------------------------------|
| `to`          | string | yes      | Chat JID                                                       |
| `from_jid`    | string | yes      | Spoofed sender JID                                             |
| `msg_id`      | string | no       | Embedded message ID; auto-generated if empty                   |
| `image`       | string | yes      | Raw base64 image data (no `data:` prefix)                      |
| `quoted_text` | string | no       | Caption shown inside the image bubble in the reply preview     |
| `text`        | string | no       | Actual text message the bot sends alongside the spoofed quote  |

**Response 200:**
```json
{ "message": "Spoofed image reply sent", "whatsapp_message": { ... }, "send_response": { ... } }
```

**Response 400:**
```json
{ "message": "image (base64) is required" }
{ "message": "Error decoding image" }
```

---

### `POST /spoof/reply-location` 🔒

Sends a spoofed reply where the quoted bubble shows a location pin attributed to the fake sender.

**Body:**
```json
{
  "to":       "5511999999999",
  "from_jid": "5533777777777@s.whatsapp.net",
  "msg_id":   "",
  "text":     "My reply to this location"
}
```

| Field      | Type   | Required | Description                              |
|------------|--------|----------|------------------------------------------|
| `to`       | string | yes      | Chat JID                                 |
| `from_jid` | string | yes      | Spoofed sender JID                       |
| `msg_id`   | string | no       | Embedded message ID; auto-generated if empty |
| `text`     | string | no       | Actual text message sent by the bot      |

**Response 200:**
```json
{ "message": "Spoofed location reply sent", "whatsapp_message": { ... }, "send_response": { ... } }
```

---

### `POST /spoof/timed` 🔒

Sends a self-destructing (timed) message. The message disappears from the recipient's screen after a short time.

**Body:**
```json
{
  "to":   "5511999999999",
  "text": "This message will self-destruct"
}
```

| Field  | Type   | Required | Description           |
|--------|--------|----------|-----------------------|
| `to`   | string | yes      | Chat JID              |
| `text` | string | yes      | Message text to send  |

**Response 200:**
```json
{ "message": "Timed message sent", "whatsapp_message": { ... }, "send_response": { ... } }
```

---

### `POST /spoof/demo` 🔒

Runs a pre-scripted spoofed conversation demo — sends a sequence of messages with delays. Returns immediately; the demo runs in the background. Body limit: **50 MB**.

**Body:**
```json
{
  "to":       "5511999999999",
  "from_jid": "5533777777777@s.whatsapp.net",
  "gender":   "boy",
  "language": "br",
  "image":    "<base64-encoded image (optional)>"
}
```

| Field      | Type   | Required | Description                                           |
|------------|--------|----------|-------------------------------------------------------|
| `to`       | string | yes      | Chat JID                                              |
| `from_jid` | string | yes      | Spoofed conversation partner JID                      |
| `gender`   | string | yes      | `"boy"` or `"girl"` — selects the demo script variant|
| `language` | string | yes      | `"br"` (Portuguese) or `"en"` (English)               |
| `image`    | string | no       | Optional base64 image to embed in the demo sequence   |

**Response 200** (returned immediately, before demo completes):
```json
{ "message": "Demo started (boy/br)" }
```

**Response 400:**
```json
{ "message": "gender must be 'boy' or 'girl'" }
{ "message": "language must be 'br' or 'en'" }
{ "message": "Error decoding image" }
```

---

### `GET /tools/{path...}`

Serves static files from the `pb_public/` directory. No authentication required.

Used to serve the frontend (`index.html`) directly from the bot.

---

## Technical details

### Body limits

| Endpoint              | Limit              |
|-----------------------|--------------------|
| `/sendimage`          | 50 MB              |
| `/sendvideo`          | 50 MB              |
| `/sendaudio`          | 50 MB              |
| `/senddocument`       | 50 MB              |
| `/media/download`     | 50 MB              |
| `/spoof/reply-img`    | 50 MB              |
| `/spoof/demo`         | 50 MB              |
| Others                | PocketBase default |

### Mimetype detection for media sends

- **image, video, document**: `http.DetectContentType(data)` (detects from first bytes)
- **audio**: `mimetype.Detect(data).String()` + `"; codecs=opus"` (library `gabriel-vasile/mimetype`)

### JID format

The `to` field in all send endpoints is processed by `ParseJID()`:

| Input                            | Resulting JID                      |
|----------------------------------|------------------------------------|
| `5511999999999`                  | `5511999999999@s.whatsapp.net`     |
| `+5511999999999`                 | `5511999999999@s.whatsapp.net`     |
| `5511999999999@s.whatsapp.net`   | `5511999999999@s.whatsapp.net`     |
| `123456789-000@g.us`             | `123456789-000@g.us` (group)       |

### Webhook payload (outbound)

When the bot fires a webhook (WhatsApp events → external destination), the sent payload is:

```json
[
  {
    "type":  "<event type>",
    "raw":   { ... },
    "extra": { ... }
  }
]
```

The webhook expects an `HTTP 200` response. Timeout: **10 seconds**.

---

## DB Explorer

Read-only and read-write access to the internal whatsmeow SQLite tables (`whatsapp.db`).
All endpoints require **🔒 Auth**. Backup files are stored in `pb_data/db_backups/`.

### Table list

```
GET /zaplab/api/db/tables
```

Response:
```json
{
  "tables": [
    { "name": "whatsmeow_device", "description": "...", "count": 1 },
    { "name": "whatsmeow_pre_keys", "description": "...", "count": 100 }
  ]
}
```

### Table rows (paginated)

```
GET /zaplab/api/db/tables/{table}?page=1&limit=50&filter=
```

| Param | Default | Max | Description |
|-------|---------|-----|-------------|
| `page` | 1 | — | 1-based page number |
| `limit` | 50 | 200 | Rows per page |
| `filter` | — | — | Free-text search applied across all columns (`CAST AS TEXT LIKE ?`) |

The first column is always `_rowid_` (SQLite internal row identifier used for write operations).
Binary BLOB columns are returned as lowercase hex strings.

Response:
```json
{
  "table":   "whatsmeow_device",
  "columns": ["_rowid_", "jid", "noise_key", "..."],
  "types":   ["INTEGER", "TEXT", "BLOB", "..."],
  "rows":    [[1, "5511...@s.whatsapp.net", "0abc...", "..."]],
  "page": 1, "limit": 50, "total": 1, "pages": 1
}
```

### Update row

```
PATCH /zaplab/api/db/tables/{table}/{rowid}
```

Body:
```json
{
  "values": { "push_name": "test", "noise_key": "0a1b2c..." },
  "reconnect": true
}
```

- `values`: map of column name → new value. All column names are validated against `PRAGMA table_info`.
- For BLOB columns, provide a hex string; it will be decoded to bytes before storage.
- `reconnect`: if `true`, triggers a WhatsApp disconnect+connect after the update.
- **An automatic backup is created before every write.**

Response:
```json
{ "message": "row updated", "backup": "whatsapp_20260316_143022.db", "reconnect": true }
```

### Delete row

```
DELETE /zaplab/api/db/tables/{table}/{rowid}
```

Body (optional):
```json
{ "reconnect": false }
```

**An automatic backup is created before deleting.**

### Reconnect

```
POST /zaplab/api/db/reconnect
```

Body:
```json
{ "full": false }
```

| `full` | Behaviour |
|--------|-----------|
| `false` | Disconnect + connect (WebSocket level; fast) |
| `true` | Full reinitialise: close client + `sqlstore.Container`, reopen from DSN, reconnect |

### Create backup

```
POST /zaplab/api/db/backup
```

Creates a clean snapshot using SQLite `VACUUM INTO`. Returns the backup filename and size.

### List backups

```
GET /zaplab/api/db/backups
```

Response:
```json
{
  "backups": [
    { "name": "whatsapp_20260316_143022.db", "size": 204800, "created": "2026-03-16T14:30:22Z" }
  ]
}
```

### Restore backup

```
POST /zaplab/api/db/restore
```

Body:
```json
{ "name": "whatsapp_20260316_143022.db" }
```

1. Closes both DB connections.
2. Copies the backup file over `whatsapp.db`; removes WAL/SHM sidecars.
3. Reopens connections.
4. Calls `whatsapp.Reinitialize()` (full stack rebuild + reconnect).

### Delete backup

```
DELETE /zaplab/api/db/backups/{name}
```
