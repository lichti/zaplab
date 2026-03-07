# Whatsmeow Analysis — ZapLab

> Analysis of the whatsmeow library to identify available features and propose new API endpoints.
> Generated: 2026-03-06

---

## Summary

| Metric | Value |
|--------|-------|
| Total message types in `waE2E.Message` | 99+ |
| Currently implemented endpoints | 6 |
| Implementation coverage | ~6% |
| Client methods found | 40+ |
| Proposed new endpoints | 24 |

**Whatsmeow library path analyzed:**
`/Users/lichti/go/pkg/mod/go.mau.fi/whatsmeow@v0.0.0-20260227112304-c9652e4448a2/`

---

## Current Endpoints

| Endpoint | Description |
|----------|-------------|
| `POST /sendmessage` | Plain text |
| `POST /sendimage` | Image with caption |
| `POST /sendvideo` | Video with caption |
| `POST /sendaudio` | Audio / voice note |
| `POST /senddocument` | File/document |
| `POST /sendraw` | Arbitrary `waE2E.Message` JSON |

---

## Message Types Available in `waE2E.Message`

### Implemented
- `conversation` — plain text
- `imageMessage` — image
- `videoMessage` — video
- `audioMessage` — audio / voice note
- `documentMessage` — file

### High-Value (not yet implemented)

| Field | Description |
|-------|-------------|
| `extendedTextMessage` | Formatted text with URL previews, mentions (`@user`) |
| `locationMessage` | Static GPS location with name/address |
| `liveLocationMessage` | Streaming live GPS location |
| `contactMessage` | Single vCard contact |
| `contactsArrayMessage` | Multiple vCard contacts |
| `reactionMessage` | Emoji reaction to a message |
| `stickerMessage` | Animated/static sticker (WebP) |
| `pollCreationMessage` | Create a poll |
| `pollUpdateMessage` | Vote on a poll |
| `protocolMessage` | Edit or revoke (delete) a sent message |

### Other Types (lower priority)

| Field | Description |
|-------|-------------|
| `interactiveMessage` | Buttons, lists, carousels (business) |
| `listMessage` | Selectable list |
| `buttonsMessage` | Button-based UI |
| `groupInviteMessage` | Group invite link |
| `orderMessage` | Product order (business) |
| `invoiceMessage` | Invoice (business) |
| `productMessage` | Product catalog item |
| `templateMessage` | Business template |
| `highlyStructuredMessage` | Business structured messages |

---

## ContextInfo Features

`ContextInfo` is a field embedded in most message types that adds metadata:

| Feature | Field | Description |
|---------|-------|-------------|
| Replies | `quotedMessage` + `stanzaId` + `participant` | Quote/reply to a previous message |
| Mentions | `mentionedJid` | @-tag users in a message |
| Ephemeral | `expiration` | Auto-delete timer (seconds) |
| ViewOnce | `viewOnce` on media message | Media viewable only once |
| Forwarding | `isForwarded` + `forwardingScore` | Mark as forwarded |
| Thread | `forwardedNewsletterMessageInfo` | Newsletter/channel threading |

---

## Whatsmeow Client Methods

### Message Control
| Method | Description |
|--------|-------------|
| `SendMessage()` | Generic send (already used) |
| `BuildRevoke()` | Construct a revoke (delete) `ProtocolMessage` |
| `RevokeMessage()` | Higher-level delete helper |
| `BuildEdit()` | Construct an edit `ProtocolMessage` |
| `BuildReaction()` | Construct a reaction message |

### Media
| Method | Description |
|--------|-------------|
| `Upload()` | Upload media to WhatsApp CDN (already used) |
| `Download()` | Download media from CDN |
| `DownloadAny()` | Download any media message |

### Presence & Typing
| Method | Description |
|--------|-------------|
| `SendPresence()` | Broadcast online/offline status |
| `SubscribePresence()` | Subscribe to contact presence updates |
| `SendChatPresence()` | Send typing / recording indicator |

### Disappearing Messages
| Method | Description |
|--------|-------------|
| `SetDisappearingTimer()` | Set auto-delete timer for a chat |

### Group Operations (13 methods)
| Method | Description |
|--------|-------------|
| `CreateGroup()` | Create a new group |
| `GetGroupInfo()` | Get group metadata |
| `GetJoinedGroups()` | List all joined groups |
| `UpdateGroupParticipants()` | Add / remove / promote / demote members |
| `SetGroupName()` | Change group name |
| `SetGroupTopic()` | Change group description |
| `SetGroupPhoto()` | Set group picture |
| `SetGroupAnnounce()` | Toggle announce-only mode |
| `SetGroupLocked()` | Lock group settings |
| `LeaveGroup()` | Leave a group |
| `JoinGroupWithLink()` | Join via invite link |
| `GetGroupInviteLink()` | Get (or reset) invite link |
| `SetGroupJoinApprovalMode()` | Toggle join approval |
| `SetGroupMemberAddMode()` | Control who can add members |

---

## Proposed New Endpoints

### High Priority

#### `POST /sendlocation`
Send a static GPS location.

```json
{
  "to": "5511999999999",
  "latitude": -23.5505,
  "longitude": -46.6333,
  "name": "São Paulo",
  "address": "Av. Paulista, 1000"
}
```

Go: `waE2E.Message{LocationMessage: &waE2E.LocationMessage{...}}`

---

#### `POST /sendelivelocation`
Share a live GPS location (streaming).

```json
{
  "to": "5511999999999",
  "latitude": -23.5505,
  "longitude": -46.6333,
  "accuracy_in_meters": 10,
  "speed_in_mps": 0,
  "degrees_clockwise_from_magnetic_north": 0,
  "sequence_number": 1,
  "time_offset": 0
}
```

Go: `waE2E.Message{LiveLocationMessage: &waE2E.LiveLocationMessage{...}}`

---

#### `POST /sendcontact`
Send a vCard contact.

```json
{
  "to": "5511999999999",
  "display_name": "John Doe",
  "vcard": "BEGIN:VCARD\nVERSION:3.0\nFN:John Doe\nTEL:+5511999999999\nEND:VCARD"
}
```

Go: `waE2E.Message{ContactMessage: &waE2E.ContactMessage{...}}`

---

#### `POST /sendreaction`
React to a message with an emoji.

```json
{
  "to": "5511999999999",
  "message_id": "ABCD1234EFGH5678",
  "sender_jid": "5511999999999@s.whatsapp.net",
  "emoji": "❤️"
}
```

Go: `client.SendMessage(ctx, to, client.BuildReaction(to, senderJID, messageID, emoji))`

To remove a reaction, send `emoji: ""`.

---

#### `POST /editmessage`
Edit a previously sent message.

```json
{
  "to": "5511999999999",
  "message_id": "ABCD1234EFGH5678",
  "new_text": "Updated message text"
}
```

Go: `client.SendMessage(ctx, to, client.BuildEdit(to, messageID, &waE2E.Message{Conversation: proto.String(newText)}))`

---

#### `POST /revokemessage`
Delete a previously sent message (for everyone).

```json
{
  "to": "5511999999999",
  "message_id": "ABCD1234EFGH5678",
  "sender_jid": "5511999999999@s.whatsapp.net"
}
```

Go: `client.SendMessage(ctx, to, client.BuildRevoke(to, senderJID, messageID))`

---

#### `POST /sendextendedtext`
Send formatted text with URL preview, mentions, or italic/bold.

```json
{
  "to": "5511999999999",
  "text": "Check out https://example.com and @5511888888888",
  "url": "https://example.com",
  "title": "Example Site",
  "description": "This is an example",
  "mentioned_jids": ["5511888888888@s.whatsapp.net"]
}
```

Go: `waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{...}}`

---

#### `POST /settyping`
Send typing or voice recording indicator.

```json
{
  "to": "5511999999999",
  "media": "text"
}
```

`media` values: `"text"` (typing), `"audio"` (recording), `""` (stop)

Go: `client.SendChatPresence(to, types.ChatPresenceComposing, types.ChatPresenceMediaText)`

---

#### `POST /setdisappearing`
Set auto-delete timer for a chat.

```json
{
  "chat": "5511999999999",
  "timer": 86400
}
```

`timer` in seconds: `0` (off), `86400` (24h), `604800` (7d), `7776000` (90d)

Go: `client.SetDisappearingTimer(chatJID, timer)`

---

### Medium Priority

#### `POST /sendsticker`
Send a sticker (WebP format, static or animated).

```json
{
  "to": "5511999999999",
  "sticker": "<base64 WebP>",
  "animated": false
}
```

Go: Upload sticker, then `waE2E.Message{StickerMessage: &waE2E.StickerMessage{...}}`

---

#### `POST /createpoll`
Create a poll.

```json
{
  "to": "5511999999999",
  "question": "What is your favorite color?",
  "options": ["Red", "Green", "Blue"],
  "selectable_options_count": 1
}
```

Go: `waE2E.Message{PollCreationMessage: &waE2E.PollCreationMessage{...}}`

---

#### `POST /votepoll`
Vote on a poll.

```json
{
  "to": "5511999999999",
  "poll_message_id": "ABCD1234EFGH5678",
  "poll_creator_jid": "5511999999999@s.whatsapp.net",
  "selected_options": ["Red"]
}
```

Go: `waE2E.Message{PollUpdateMessage: &waE2E.PollUpdateMessage{...}}`

---

#### `GET /groups`
List all groups the bot participates in.

```json
[
  {
    "jid": "123456789-000@g.us",
    "name": "Group Name",
    "participants": 25,
    "topic": "Group description"
  }
]
```

Go: `client.GetJoinedGroups()`

Already partially supported via `/cmd listgroups` — expose as REST endpoint.

---

#### `GET /groups/{jid}`
Get detailed info about a group.

```json
{
  "jid": "123456789-000@g.us",
  "name": "Group Name",
  "topic": "Group description",
  "participants": [
    { "jid": "5511999999999@s.whatsapp.net", "isAdmin": true }
  ],
  "announce": false,
  "locked": false,
  "disappearing_timer": 0
}
```

Go: `client.GetGroupInfo(jid)`

Already partially supported via `/cmd getgroup` — expose as REST endpoint.

---

#### `POST /groups`
Create a new group.

```json
{
  "name": "My Group",
  "participants": ["5511999999999", "5511888888888"]
}
```

Go: `client.CreateGroup(waProto.ReqCreateGroup{...})`

---

#### `POST /groups/{jid}/participants`
Add, remove, promote, or demote participants.

```json
{
  "action": "add",
  "participants": ["5511999999999"]
}
```

`action` values: `"add"`, `"remove"`, `"promote"`, `"demote"`

Go: `client.UpdateGroupParticipants(jid, []types.JID{...}, action)`

---

#### `POST /setpresence`
Set bot presence (online/offline).

```json
{
  "available": true
}
```

Go: `client.SendPresence(types.PresenceAvailable)` or `types.PresenceUnavailable`

---

#### `POST /subscribepresence`
Subscribe to a contact's presence updates (online/offline/last seen).

```json
{
  "jid": "5511999999999"
}
```

Go: `client.SubscribePresence(jid)` — events arrive via `events.Presence` webhook event.

---

### Low Priority (Advanced)

#### `POST /sendcontacts`
Send multiple contacts at once.

```json
{
  "to": "5511999999999",
  "contacts": [
    { "display_name": "John", "vcard": "BEGIN:VCARD..." },
    { "display_name": "Jane", "vcard": "BEGIN:VCARD..." }
  ]
}
```

Go: `waE2E.Message{ContactsArrayMessage: &waE2E.ContactsArrayMessage{...}}`

---

#### `POST /sendimage` + `POST /sendvideo` + `POST /sendaudio` (ViewOnce support)
Extend existing media endpoints with an optional `view_once` flag.

```json
{
  "to": "5511999999999",
  "image": "<base64>",
  "view_once": true
}
```

Go: Wrap the media message inside `waE2E.Message{ViewOnceMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{ImageMessage: ...}}}`

---

#### `POST /sendimage` + media endpoints (Ephemeral/disappearing)
Extend media endpoints with `ephemeral_expiration` in the `ContextInfo`.

```json
{
  "to": "5511999999999",
  "image": "<base64>",
  "ephemeral_expiration": 86400
}
```

Go: Set `ContextInfo.Expiration` on the media message.

---

## Reply Support (all message types)

All send endpoints should support an optional `reply_to` block:

```json
{
  "to": "5511999999999",
  "message": "That's great!",
  "reply_to": {
    "message_id": "ABCD1234EFGH5678",
    "sender_jid": "5511999999999@s.whatsapp.net",
    "quoted_text": "Original message text"
  }
}
```

Go: Populate `ContextInfo` with `StanzaId`, `Participant`, and `QuotedMessage`.

---

## Implementation Phases

### Phase 1 — Message Control (highest ROI)
- `POST /sendreaction`
- `POST /editmessage`
- `POST /revokemessage`
- `POST /settyping`

These use `BuildReaction()`, `BuildEdit()`, `BuildRevoke()` — whatsmeow already provides the helpers.

### Phase 2 — Location & Privacy
- `POST /sendlocation`
- `POST /sendelivelocation`
- `POST /setdisappearing`
- Reply support in existing endpoints

### Phase 3 — Contacts & Polls
- `POST /sendcontact`
- `POST /sendcontacts`
- `POST /createpoll`
- `POST /votepoll`

### Phase 4 — Group Management
- `GET /groups`
- `GET /groups/{jid}`
- `POST /groups`
- `POST /groups/{jid}/participants`

### Phase 5 — Presence & Advanced Media
- `POST /setpresence`
- `POST /subscribepresence`
- ViewOnce flag on media endpoints
- Ephemeral expiration on media endpoints
- `POST /sendsticker`

---

## Database Schema Additions

To track state for new features, consider adding PocketBase collections:

| Collection | Purpose |
|------------|---------|
| `messages` | Track sent messages (ID → JID mapping for edit/delete) |
| `reactions` | Track reactions per message |
| `polls` | Store poll question + options |
| `poll_votes` | Store individual votes |
| `groups` | Cache group metadata |
| `presence` | Store last-seen presence events |

---

## Notes

- All new endpoints follow the existing pattern: `postSendXxx(e *core.RequestEvent) error`
- JID parsing via `whatsapp.ParseJID()` remains the same
- Media upload for stickers uses `client.Upload(ctx, data, whatsmeow.MediaImage)`
- `BuildRevoke`, `BuildEdit`, `BuildReaction` return a `*waE2E.Message` directly — pass to `sendMessage()`
- Reactions require the original message ID and the original sender's JID
- For group endpoints, use `client.GetGroupInfo()` / `client.GetJoinedGroups()` / `client.CreateGroup()`
- Typing indicator should be followed by a stop call after message is sent
- ViewOnce wrapping: `waE2E.Message{ViewOnceMessage: &waE2E.FutureProofMessage{Message: innerMsg}}`
- Ephemeral: set `ContextInfo.Expiration = uint32(seconds)` on any message
