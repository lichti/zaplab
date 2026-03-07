# Phase 1 — Message Control Spec

> Endpoints: `/sendreaction`, `/editmessage`, `/revokemessage`, `/settyping`
> whatsmeow helpers used: `BuildReaction`, `BuildEdit`, `BuildRevoke`, `SendChatPresence`

---

## Endpoints

### `POST /sendreaction`

Adds or removes an emoji reaction on a message.

**Body:**
```json
{
  "to":         "5511999999999",
  "message_id": "ABCD1234EFGH5678",
  "sender_jid": "5511999999999@s.whatsapp.net",
  "emoji":      "❤️"
}
```

| Field        | Type   | Required | Notes                                          |
|--------------|--------|----------|------------------------------------------------|
| `to`         | string | yes      | Chat JID (number or full JID)                  |
| `message_id` | string | yes      | ID of the target message                       |
| `sender_jid` | string | yes      | JID of the original message author             |
| `emoji`      | string | no       | Emoji string. Empty string removes the reaction|

**Go implementation (`internal/whatsapp/send.go`):**
```go
func SendReaction(chat, sender types.JID, messageID, emoji string) (*waE2E.Message, *whatsmeow.SendResponse, error) {
    msg := client.BuildReaction(chat, sender, messageID, emoji)
    return sendMessage(chat, msg)
}
```

**whatsapp_message produced:**
```json
{
  "reactionMessage": {
    "key": { "remoteJid": "...", "fromMe": false, "id": "ABCD1234EFGH5678", "participant": "..." },
    "text": "❤️",
    "senderTimestampMs": "1700000000000"
  }
}
```

---

### `POST /editmessage`

Edits a previously sent text message (bot's own messages only).

> whatsmeow constant: `EditWindow = 20 * time.Minute` — edits after this window may be rejected by WhatsApp.

**Body:**
```json
{
  "to":         "5511999999999",
  "message_id": "ABCD1234EFGH5678",
  "new_text":   "Updated message text"
}
```

| Field        | Type   | Required | Notes                          |
|--------------|--------|----------|--------------------------------|
| `to`         | string | yes      | Chat JID (number or full JID)  |
| `message_id` | string | yes      | ID of the message to edit      |
| `new_text`   | string | yes      | Replacement text content       |

**Go implementation (`internal/whatsapp/send.go`):**
```go
func EditMessage(chat types.JID, messageID, newText string) (*waE2E.Message, *whatsmeow.SendResponse, error) {
    newContent := &waE2E.Message{Conversation: proto.String(newText)}
    msg := client.BuildEdit(chat, messageID, newContent)
    return sendMessage(chat, msg)
}
```

**whatsapp_message produced:**
```json
{
  "editedMessage": {
    "message": {
      "protocolMessage": {
        "key": { "fromMe": true, "id": "ABCD1234EFGH5678", "remoteJid": "..." },
        "type": "MESSAGE_EDIT",
        "editedMessage": { "conversation": "Updated message text" },
        "timestampMs": "1700000000000"
      }
    }
  }
}
```

---

### `POST /revokemessage`

Deletes a message for everyone (revoke). Group admins can revoke other members' messages by passing their JID as `sender_jid`.

**Body:**
```json
{
  "to":         "5511999999999",
  "message_id": "ABCD1234EFGH5678",
  "sender_jid": "5511999999999@s.whatsapp.net"
}
```

| Field        | Type   | Required | Notes                                                       |
|--------------|--------|----------|-------------------------------------------------------------|
| `to`         | string | yes      | Chat JID (number or full JID)                               |
| `message_id` | string | yes      | ID of the message to delete                                 |
| `sender_jid` | string | yes      | JID of the message author. Use bot's own JID for own messages|

**Go implementation (`internal/whatsapp/send.go`):**
```go
func RevokeMessage(chat, sender types.JID, messageID string) (*waE2E.Message, *whatsmeow.SendResponse, error) {
    msg := client.BuildRevoke(chat, sender, messageID)
    return sendMessage(chat, msg)
}
```

**whatsapp_message produced:**
```json
{
  "protocolMessage": {
    "key": { "remoteJid": "...", "fromMe": false, "id": "ABCD1234EFGH5678", "participant": "..." },
    "type": "REVOKE"
  }
}
```

---

### `POST /settyping`

Sends a typing or voice-recording presence indicator. Does not produce a WhatsApp message — it is a signaling-only operation.

**Body:**
```json
{
  "to":    "5511999999999",
  "state": "composing",
  "media": "text"
}
```

| Field   | Type   | Required | Values                                       |
|---------|--------|----------|----------------------------------------------|
| `to`    | string | yes      | Chat JID (number or full JID)                |
| `state` | string | yes      | `"composing"` or `"paused"`                  |
| `media` | string | no       | `"text"` (default) or `"audio"`              |

- `state: "composing"` + `media: "text"` → shows typing indicator
- `state: "composing"` + `media: "audio"` → shows voice recording indicator
- `state: "paused"` → stops any active indicator (`media` is ignored)

**Go implementation (`internal/whatsapp/presence.go`):**
```go
func SetTyping(chat types.JID, state, media string) error {
    presenceState := types.ChatPresencePaused
    if state == "composing" {
        presenceState = types.ChatPresenceComposing
    }
    presenceMedia := types.ChatPresenceMediaText
    if media == "audio" {
        presenceMedia = types.ChatPresenceMediaAudio
    }
    return client.SendChatPresence(context.Background(), chat, presenceState, presenceMedia)
}
```

**Response:** `{ "message": "Typing state updated" }` — no `whatsapp_message` or `send_response`.

---

## Frontend: Message Control section

Located in `pb_public/index.html`, shown when `activeSection === 'ctrl'`.

### Layout

```
┌─────────────────────────────────────────────────────────────────┐
│  Message Control          react · edit · delete · typing        │
├───────────────────────────┬─────────────────────────────────────┤
│                           │                                     │
│  API Token                │  ┌── Preview ──────────────────┐   │
│  Action selector          │  │  [cURL]  [Response]    Copy  │   │
│  To                       │  │  /sendreaction          ...  │   │
│                           │  │  <reactive content>         │   │
│  ── react ──              │  └─────────────────────────────┘   │
│  Message ID               │                                     │
│  Sender JID               │                                     │
│  Emoji (quick-pick + text)│                                     │
│                           │                                     │
│  ── edit ──               │                                     │
│  Message ID               │                                     │
│  New Text (textarea)      │                                     │
│                           │                                     │
│  ── delete ──             │                                     │
│  Message ID               │                                     │
│  Sender JID               │                                     │
│                           │                                     │
│  ── typing ──             │                                     │
│  State (composing/paused) │                                     │
│  Media (text/audio)       │                                     │
│                           │                                     │
│  [Send Reaction]          │                                     │
│                           │                                     │
└───────────────────────────┴─────────────────────────────────────┘
```

### Alpine state

```js
ctrl: {
  type:      'react',    // 'react' | 'edit' | 'delete' | 'typing'
  to:        '',
  messageId: '',
  senderJid: '',
  emoji:     '❤️',
  newText:   '',
  state:     'composing',
  media:     'text',
  loading:   false,
  toast:     null,
  result:    null,
},
ctrlPreviewTab:    localStorage.getItem('zaplab-ctrl-preview-tab') || 'curl',
ctrlPreviewCopied: false,
```

### Alpine methods

| Method               | Description                                              |
|----------------------|----------------------------------------------------------|
| `ctrlEndpoint()`     | Returns `/sendreaction`, `/editmessage`, etc.            |
| `ctrlLabel()`        | Button label: "Send Reaction", "Edit Message", etc.      |
| `ctrlCurlPayload()`  | Returns the request body object for the current type     |
| `ctrlCurlPreview()`  | Builds the `curl` command string                         |
| `ctrlResultPreview()`| Renders the API response with syntax highlight           |
| `ctrlPreviewContent()`| Dispatches to cURL or Response tab                      |
| `copyCtrlPreview()`  | Copies active tab content to clipboard                   |
| `submitCtrl()`       | POSTs payload to the appropriate endpoint                |

### localStorage keys

| Key                        | Default  | Description              |
|----------------------------|----------|--------------------------|
| `zaplab-ctrl-preview-tab`  | `'curl'` | Active preview tab       |

### Emoji quick-pick

The React form shows 8 preset emoji buttons (`❤️ 👍 😂 😮 😢 🙏 🔥 🎉`) that set `ctrl.emoji` on click. A free-text input below allows entering any emoji. The active emoji button is highlighted with `border-blue-500`.

---

## Files changed

| File | Change |
|------|--------|
| `internal/whatsapp/send.go` | Added `SendReaction`, `RevokeMessage`, `EditMessage` |
| `internal/whatsapp/presence.go` | New file — `SetTyping` |
| `internal/api/api.go` | Registered 4 routes; added 4 handlers |
| `pb_public/index.html` | Sidebar button, section HTML, Alpine state + methods |
| `specs/API_SPEC.md` | Documented 4 new endpoints |
| `README.md` | Added 4 endpoints to REST API section |
| `README.pt-BR.md` | Added 4 endpoints to REST API section (PT) |
