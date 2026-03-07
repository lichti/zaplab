# Phase 2 — Location, Disappearing & Reply Support Spec

> Endpoints: `/sendlocation`, `/sendelivelocation`, `/setdisappearing`
> Cross-cutting: `reply_to` optional field on all send endpoints

---

## New Endpoints

### `POST /sendlocation`

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

**Go (`internal/whatsapp/send.go`):**
```go
func SendLocation(to types.JID, lat, lon float64, name, address string, reply *ReplyInfo) (*waE2E.Message, *whatsmeow.SendResponse, error)
```

Uses `waE2E.LocationMessage{DegreesLatitude, DegreesLongitude, Name, Address, ContextInfo}`.

---

### `POST /sendelivelocation`

Sends a live GPS location update. Call repeatedly with incrementing `sequence_number` to move the map pin on the receiver's device.

**Body:**
```json
{
  "to":                                    "5511999999999",
  "latitude":                              -23.5505,
  "longitude":                             -46.6333,
  "accuracy_in_meters":                    10,
  "speed_in_mps":                          1.4,
  "degrees_clockwise_from_magnetic_north": 90,
  "caption":                               "Heading to the office",
  "sequence_number":                       1,
  "time_offset":                           0
}
```

**Go (`internal/whatsapp/send.go`):**
```go
func SendLiveLocation(to types.JID, lat, lon float64, accuracyMeters uint32, speedMps float32, bearingDegrees uint32, caption string, sequenceNumber int64, timeOffset uint32, reply *ReplyInfo) (*waE2E.Message, *whatsmeow.SendResponse, error)
```

Uses `waE2E.LiveLocationMessage`.

---

### `POST /setdisappearing`

Sets the disappearing (auto-delete) timer for a chat or group.

**Body:**
```json
{ "to": "5511999999999", "timer": 86400 }
```

Allowed `timer` values (seconds):
| Value     | Meaning    |
|-----------|------------|
| `0`       | Off        |
| `86400`   | 24 hours   |
| `604800`  | 7 days     |
| `7776000` | 90 days    |

**Go (`internal/whatsapp/presence.go`):**
```go
func SetDisappearing(chat types.JID, timerSeconds uint32) error {
    return client.SetDisappearingTimer(context.Background(), chat, time.Duration(timerSeconds)*time.Second, time.Time{})
}
```

`settingTS` is passed as `time.Time{}` (zero) for DMs — whatsmeow auto-sets it to `time.Now()`.
For groups the server handles the timestamp.

**Response:** `{ "message": "Disappearing timer updated" }` — no whatsapp_message.

---

## Cross-cutting: Reply support (`reply_to`)

All send endpoints now accept an optional `reply_to` object to quote a previous message:

```json
{
  "reply_to": {
    "message_id":  "ABCD1234EFGH5678",
    "sender_jid":  "5511999999999@s.whatsapp.net",
    "quoted_text": "Original text shown in the bubble"
  }
}
```

**Affected endpoints:** `/sendmessage`, `/sendimage`, `/sendvideo`, `/sendaudio`, `/senddocument`, `/sendlocation`, `/sendelivelocation`

### Go implementation

**`internal/whatsapp/send.go`:**
```go
type ReplyInfo struct {
    MessageID string
    Sender    types.JID
    Text      string // optional: simplified preview shown in the bubble
}

func buildContextInfo(r *ReplyInfo) *waE2E.ContextInfo {
    if r == nil || r.MessageID == "" {
        return nil
    }
    return &waE2E.ContextInfo{
        StanzaID:    proto.String(r.MessageID),
        Participant: proto.String(r.Sender.String()),
        QuotedMessage: &waE2E.Message{
            Conversation: proto.String(r.Text),
        },
    }
}
```

**`internal/api/api.go`:**
```go
type replyToRequest struct {
    MessageID string `json:"message_id"`
    SenderJID string `json:"sender_jid"`
    Text      string `json:"quoted_text"`
}

func parseReplyTo(r *replyToRequest) *whatsapp.ReplyInfo {
    if r == nil || r.MessageID == "" { return nil }
    senderJID, ok := whatsapp.ParseJID(r.SenderJID)
    if !ok { return nil }
    return &whatsapp.ReplyInfo{MessageID: r.MessageID, Sender: senderJID, Text: r.Text}
}
```

### Text vs. ExtendedTextMessage

Plain text (`/sendmessage`) without reply → `Conversation` field (unchanged).
Plain text with `reply_to` → automatically switches to `ExtendedTextMessage{Text, ContextInfo}`.
Media messages always have `ContextInfo` set directly on the media proto struct.

---

## Frontend changes

### Send Message section

- Type selector: added `Location` and `Live Location` options
- Location fields (lat/lon) shown for both location types
- Static location: name + address fields
- Live location: accuracy / speed / bearing numeric inputs
- Reply To collapsible (checkbox trigger):
  - Message ID input
  - Sender JID input
  - Quoted text input (optional)
- `sendPayload()`, `sendCurlPayload()`, `sendEndpoint()`, `submitSend()` all updated for location types

### Message Control section

- New action: `Disappearing — set auto-delete timer`
- Timer select: Off / 24h / 7d / 90d
- `ctrlEndpoint`, `ctrlLabel`, `ctrlCurlPayload` updated

### New Alpine state fields (in `send`)

```js
// location
latitude:     '',
longitude:    '',
locName:      '',
locAddress:   '',
locAccuracy:  10,
locSpeed:     0,
locBearing:   0,
// reply_to
replyEnabled: false,
replyId:      '',
replySender:  '',
replyText:    '',
```

### New Alpine state fields (in `ctrl`)
```js
timer: '86400',  // for disappearing type
```

---

## Files changed

| File | Change |
|------|--------|
| `internal/whatsapp/send.go` | `ReplyInfo`, `buildContextInfo`; modified 5 existing send functions; added `SendLocation`, `SendLiveLocation` |
| `internal/whatsapp/presence.go` | Added `SetDisappearing` |
| `internal/api/api.go` | `replyToRequest`, `parseReplyTo`; updated 5 handlers; added `postSendLocation`, `postSendLiveLocation`, `postSetDisappearing`; registered 3 routes |
| `internal/whatsapp/events.go` | Updated `SendConversationMessage` calls → pass `nil` reply |
| `internal/whatsapp/spoof.go` | Updated `SendConversationMessage` calls → pass `nil` reply |
| `pb_public/index.html` | Location types, location fields, reply_to form, disappearing in ctrl |
| `specs/API_SPEC.md` | Documented 3 new endpoints + reply_to field |
| `README.md` | Added 3 new endpoints + reply_to section |
| `README.pt-BR.md` | Same in Portuguese |
