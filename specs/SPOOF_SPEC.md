# Spoof Messages Spec

> Endpoints: `/spoof/reply`, `/spoof/reply-private`, `/spoof/reply-img`, `/spoof/reply-location`, `/spoof/timed`, `/spoof/demo`

---

## Overview

WhatsApp's reply system uses a `ContextInfo` protobuf embedded in the message that describes the quoted message. This `ContextInfo` contains:

- `StanzaID` — the message ID of the quoted message
- `Participant` — the JID of the quoted message's author
- `QuotedMessage` — a partial copy of the quoted message content

WhatsApp clients display this context **as received**, without verifying that the referenced message actually exists or that `Participant` matches reality. This allows sending messages that appear to quote content that was never sent.

These endpoints implement spoofing for research and testing purposes.

---

## Protocol Details

### ExtendedTextMessage with spoofed ContextInfo

```
ExtendedTextMessage {
  Text: "<actual message text>"
  ContextInfo {
    StanzaID:      "<fake or real msg_id>"
    Participant:   "<spoofed sender JID>"
    QuotedMessage: {
      Conversation: "<fake quoted text>"
    }
  }
}
```

The receiver's app renders the quoted bubble as if it came from `Participant` with content from `QuotedMessage`, regardless of whether that message ever existed in the chat history.

### Image spoofing

For `reply-img`, the image is uploaded to the WhatsApp CDN (same as a normal image send) and embedded in the `QuotedMessage` as an `ImageMessage`. The recipient sees a spoofed image reply bubble attributed to the fake sender.

### Location spoofing

For `reply-location`, a `LocationMessage` with hardcoded coordinates (0.0, 0.0) is embedded in the `QuotedMessage`. The recipient sees a reply bubble showing a location pin attributed to the fake sender.

### Timed messages

A "timed" (self-destructing) message uses the `EphemeralMessage` wrapper. The message is shown to the recipient for a short period, then WhatsApp automatically deletes it.

### Demo sequence

The demo sends a pre-scripted sequence of messages (text + optional image) with delays between them, simulating a spoofed conversation. Four script variants: `boy/br`, `boy/en`, `girl/br`, `girl/en`. The endpoint returns immediately; the sequence runs in a background goroutine.

---

## Endpoints

### `POST /spoof/reply`

Sends a text message that appears to reply to a fake quoted message from a spoofed sender.

**Body:**
```json
{
  "to":          "5511999999999",
  "from_jid":    "5533777777777@s.whatsapp.net",
  "msg_id":      "",
  "quoted_text": "This is the fake quoted content",
  "text":        "My actual message"
}
```

| Field         | Required | Description                                               |
|---------------|----------|-----------------------------------------------------------|
| `to`          | yes      | Chat JID                                                  |
| `from_jid`    | yes      | Spoofed sender JID shown in the reply bubble              |
| `msg_id`      | no       | ID embedded in ContextInfo; auto-generated if empty       |
| `quoted_text` | no       | Text shown in the reply preview bubble (spoofed content)  |
| `text`        | no       | Actual message text the bot sends                         |

---

### `POST /spoof/reply-private`

Same as `/spoof/reply` but sends the message to the recipient's **private chat** (DM) even if `to` is a group JID. The spoofed context still references the group sender.

**Body:** identical to `/spoof/reply`.

---

### `POST /spoof/reply-img`

Sends a text message that appears to reply to a fake image from a spoofed sender. The image is uploaded to the WhatsApp CDN and embedded in the quote bubble. Body limit: **50 MB**.

**Body:**
```json
{
  "to":          "5511999999999",
  "from_jid":    "5533777777777@s.whatsapp.net",
  "msg_id":      "",
  "image":       "<raw base64 — no data: prefix>",
  "quoted_text": "Caption in the image bubble",
  "text":        "My reply text"
}
```

| Field         | Required | Description                                                    |
|---------------|----------|----------------------------------------------------------------|
| `to`          | yes      | Chat JID                                                       |
| `from_jid`    | yes      | Spoofed sender JID                                             |
| `msg_id`      | no       | Auto-generated if empty                                        |
| `image`       | yes      | Raw base64 image data                                          |
| `quoted_text` | no       | Caption shown inside the quoted image bubble                   |
| `text`        | no       | Actual text message the bot sends alongside the spoofed quote  |

---

### `POST /spoof/reply-location`

Sends a text message that appears to reply to a fake location from a spoofed sender.

**Body:**
```json
{
  "to":       "5511999999999",
  "from_jid": "5533777777777@s.whatsapp.net",
  "msg_id":   "",
  "text":     "Reply to this location"
}
```

---

### `POST /spoof/timed`

Sends a self-destructing (ephemeral) text message. Does not require `from_jid`.

**Body:**
```json
{
  "to":   "5511999999999",
  "text": "This message will disappear"
}
```

---

### `POST /spoof/demo`

Starts a pre-scripted spoofed conversation in the background. Body limit: **50 MB**.

**Body:**
```json
{
  "to":       "5511999999999",
  "from_jid": "5533777777777@s.whatsapp.net",
  "gender":   "boy",
  "language": "br",
  "image":    "<base64 image — optional>"
}
```

| Field      | Required | Description                                            |
|------------|----------|--------------------------------------------------------|
| `to`       | yes      | Chat JID                                               |
| `from_jid` | yes      | Spoofed conversation partner JID                       |
| `gender`   | yes      | `"boy"` or `"girl"` — script variant                   |
| `language` | yes      | `"br"` (Portuguese) or `"en"` (English)                |
| `image`    | no       | Optional base64 image embedded in the demo sequence    |

Returns `{ "message": "Demo started (boy/br)" }` immediately; sequences run in a goroutine.

---

## Go Implementation

**File:** `internal/whatsapp/spoof.go`

| Exported Function | Description |
|---|---|
| `SpoofReply(chatJID, fromJID types.JID, msgID, quotedText, text string)` | Spoofed text reply |
| `SpoofReplyPrivate(chatJID, fromJID types.JID, msgID, quotedText, text string)` | Spoofed reply in private chat |
| `SpoofReplyImg(chatJID, fromJID types.JID, msgID string, imgData []byte, quotedText, text string)` | Spoofed image reply |
| `SpoofReplyLocation(chatJID, fromJID types.JID, msgID, text string)` | Spoofed location reply |
| `SendTimedMessage(chatJID types.JID, text string)` | Self-destructing message |
| `SpoofDemo(chatJID, spoofedJID types.JID, gender, language string, imgData []byte)` | Pre-scripted demo sequence (blocking — must call in goroutine) |

**File:** `internal/api/api.go`

Helper functions:
- `parseSpoofBase(e, to, fromJID)` — validates and parses both JIDs, returns early with JSON error if invalid
- `resolveMsgID(msgID)` — returns `msgID` if non-empty, or calls `client.GenerateMessageID()` to create a random ID

---

## Frontend

Section: **Spoof Messages** (`activeSection === 'spoof'`)

File: `pb_public/js/sections/spoof.js` — `spoofSection()` factory

State object: `spoof.{type, to, fromJid, msgId, quotedText, text, image, imageName, gender, language, loading, toast, result}`

Supported types and their required fields:

| Type | `from_jid` | `text` | `quoted_text` | `image` | `gender`/`language` |
|---|:---:|:---:|:---:|:---:|:---:|
| `reply` | ✓ | ✓ | ✓ | — | — |
| `reply-private` | ✓ | ✓ | ✓ | — | — |
| `reply-img` | ✓ | ✓ | ✓ | ✓ | — |
| `reply-location` | ✓ | ✓ | — | — | — |
| `timed` | — | ✓ | — | — | — |
| `demo` | ✓ | — | — | optional | ✓ |

---

## Files Changed

| File | Change |
|---|---|
| `internal/whatsapp/spoof.go` | Added exported wrapper functions (`SpoofReply`, `SpoofReplyPrivate`, `SpoofReplyImg`, `SpoofReplyLocation`, `SendTimedMessage`, `SpoofDemo`) |
| `internal/api/api.go` | Added 6 spoof routes + `parseSpoofBase` + `resolveMsgID` helpers |
| `pb_public/js/sections/spoof.js` | New — `spoofSection()` factory |
| `pb_public/js/zaplab.js` | Added `spoofSection()` to `Object.assign`, `this.initSpoof()` to `init()` |
| `pb_public/index.html` | Added spoof nav button, spoof section HTML, `<script src>` tag |
| `specs/API_SPEC.md` | Documented 6 new endpoints and updated body limits table |
