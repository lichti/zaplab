# Beta 8 Features Specification

Five features introduced in the `feature/beta8` branch.

---

## 1. Full-text Message Search

### Backend

**Endpoint:** `GET /zaplab/api/search`

**Query parameters:**

| Parameter | Type   | Description                                  |
|-----------|--------|----------------------------------------------|
| `q`       | string | Required. Search term (min 1 char after trim) |
| `type`    | string | Optional. Filter by event `type` column      |
| `chat`    | string | Optional. Filter by `Info.Chat` JID          |
| `limit`   | int    | Default 50, max 200                          |
| `offset`  | int    | Default 0                                    |

**Implementation:** Raw SQLite on the PocketBase `events` table using `LIKE '%q%'` on:
- `json_extract(raw, '$.Message.Conversation')`
- `json_extract(raw, '$.Message.ExtendedTextMessage.Text')`
- `json_extract(raw, '$.Message.ImageMessage.Caption')`
- `json_extract(raw, '$.Message.VideoMessage.Caption')`
- `json_extract(raw, '$.Message.DocumentMessage.Caption')`
- Exact `msgID` match

**Response:**
```json
{
  "results": [
    {
      "id": "pb_record_id",
      "type": "Message",
      "msgID": "AABBCCDD...",
      "chat": "5511999@s.whatsapp.net",
      "sender": "5511888@s.whatsapp.net",
      "text_preview": "first 200 chars...",
      "created": "2026-03-17 12:00:00.000Z"
    }
  ],
  "total": 42,
  "limit": 50,
  "offset": 0,
  "q": "hello"
}
```

**File:** `internal/api/search.go`

### Frontend

Section ID: `search` | JS file: `pb_public/js/sections/search.js` | State prefix: `sr`

Key state: `srQuery`, `srType`, `srChat`, `srResults`, `srTotal`, `srLimit`, `srOffset`, `srSelectedEvent`

Key methods: `srSearch(resetOffset)`, `srClear()`, `srNextPage()`, `srPrevPage()`, `srOpenEvent(ev)`, `srOpenConversation(chat)`

- Result cards show sender, chat, type badge, text preview, timestamp.
- "Open conversation" button sets `cvSelectedChat` and navigates to `conversation` section.
- Raw event drawer opens on card click.

---

## 2. Conversation View

### Backend

**Endpoints:**

#### `GET /zaplab/api/conversation/chats`

Query params: `limit` (default 50)

Returns deduplicated chat list ordered by most-recent message (`MAX(created) DESC`).

```json
{
  "chats": [
    {
      "chat": "5511999@s.whatsapp.net",
      "msg_count": 127,
      "last_msg": "2026-03-17 12:00:00.000Z"
    }
  ]
}
```

#### `GET /zaplab/api/conversation`

Query params: `chat` (required), `limit` (default 100), `before` (RFC3339 cursor)

Returns messages for the given chat in DESC order (JS reverses for bubble display).

```json
{
  "messages": [
    {
      "id": "pb_record_id",
      "msgID": "AABBCCDD...",
      "chat": "5511999@s.whatsapp.net",
      "sender": "5511888@s.whatsapp.net",
      "is_from_me": false,
      "msg_type": "text",
      "text": "Hello!",
      "caption": "",
      "file_url": "",
      "thumb_url": "",
      "created": "2026-03-17 12:00:00.000Z",
      "raw": { ... }
    }
  ],
  "has_more": true,
  "next_before": "2026-03-10 08:00:00.000Z"
}
```

**Message type detection** (in priority order): text → image → video → audio → document → sticker → location → reaction → unknown.

File URL: `/api/files/events/<id>/<file>` with `?thumb=300x300` for thumbnails.

**File:** `internal/api/conversation.go`

### Frontend

Section ID: `conversation` | JS file: `pb_public/js/sections/conversation.js` | State prefix: `cv`

Key state: `cvChats`, `cvChatFilter`, `cvSelectedChat`, `cvMessages`, `cvHasMore`, `cvNextBefore`, `cvSelectedMsg`

Key methods: `cvLoadChats()`, `cvSelectChat(chat)`, `cvLoadMessages(chat, before)`, `cvLoadMore()`

- Two-pane layout: left chat list (filter input + scrollable list with last-message preview), right bubble area.
- Sent messages (is_from_me) aligned right; received left.
- Inline media thumbnails for image/video/audio.
- "Load older messages" button when `cvHasMore` is true.
- Drawer panel shows full raw JSON on message click.
- `cvSelectChat` is called from search section's `srOpenConversation`.

---

## 3. Script Triggers (Event Hooks)

### Migration

File: `migrations/1743000000_create_script_triggers.go`

Collection: `script_triggers` (ID: `tr1g9x0k2mzpw5q`)

Fields:
- `script_id` — TextField, required
- `event_type` — TextField, required
- `jid_filter` — TextField (optional substring on `Info.Chat`)
- `text_pattern` — TextField (optional case-insensitive contains on message text)
- `enabled` — BoolField

Indexes: `event_type`, `enabled`, `created`

### Dispatch Architecture

To avoid an import cycle (`api` imports `whatsapp`; `whatsapp` must not import `api`):

1. `internal/whatsapp/deps.go` declares `var TriggerDispatchFunc func(evtType string, rawJSON []byte)`.
2. `internal/whatsapp/persist.go` calls `go TriggerDispatchFunc(evtType, rawBytes)` after saving each event.
3. `internal/api/triggers.go:InitTriggerDispatch()` sets `whatsapp.TriggerDispatchFunc = dispatchTriggers`.
4. `InitTriggerDispatch()` is called from `internal/api/api.go:Init()`.

### `dispatchTriggers` logic

```
for each trigger WHERE event_type=evtType AND enabled=true:
  if jid_filter != "" and Info.Chat does not contain jid_filter → skip
  if text_pattern != "" and message text does not contain text_pattern (case-insensitive) → skip
  go runScript(code, timeout, {"event": parsedEventMap})
```

Message text is extracted from: `Message.Conversation`, `Message.ExtendedTextMessage.Text`, or media captions.

### REST API

| Method | Path | Description |
|--------|------|-------------|
| GET    | `/zaplab/api/script-triggers` | List all triggers |
| POST   | `/zaplab/api/script-triggers` | Create trigger |
| PATCH  | `/zaplab/api/script-triggers/{id}` | Update trigger |
| DELETE | `/zaplab/api/script-triggers/{id}` | Delete trigger |
| GET    | `/zaplab/api/script-triggers/event-types` | Distinct event types from DB |

**File:** `internal/api/triggers.go`

### Frontend

Section ID: `triggers` | JS file: `pb_public/js/sections/triggers.js` | State prefix: `tr`

Key state: `trTriggers`, `trEventTypes`, `trScriptOptions`, `trSelected`, `trShowNew`

Key methods: `trLoad()`, `trCreate()`, `trSave(t)`, `trDelete(t)`, `trToggleEnabled(t)`

- Trigger list with event type badge, script name, filter summary, enabled toggle.
- Inline edit form for selected trigger (event type dropdown, script dropdown, JID filter, text pattern, enabled).
- New trigger form (same fields) shown when `trShowNew` is true.

---

## 4. Expanded `wa.*` Scripting Bindings

### New bindings in `internal/api/scripts.go`

| Binding | Signature | Description |
|---------|-----------|-------------|
| `wa.jid` | string | Own JID of the connected account |
| `wa.sendImage` | `(to, b64data, mime, caption)` | Send image message |
| `wa.sendAudio` | `(to, b64data, mime, ptt)` | Send audio; `ptt=true` for voice note |
| `wa.sendDocument` | `(to, b64data, mime, filename, caption)` | Send file attachment |
| `wa.sendLocation` | `(to, lat, lng, name)` | Send location pin |
| `wa.sendReaction` | `(to, msgId, emoji)` | React to a message |
| `wa.editMessage` | `(to, msgId, newText)` | Edit a sent text message |
| `wa.revokeMessage` | `(to, msgId)` | Delete for everyone |
| `wa.setTyping` | `(to, typing)` | Set composing/paused presence |
| `wa.getContacts` | `()` → array | All stored contacts |
| `wa.getGroups` | `()` → array | All joined groups |
| `wa.db.query` | `(sql, params)` | Query `whatsapp.db` |

### `runScript` signature change

```go
func runScript(code string, timeout time.Duration, env map[string]any) (string, error)
```

Variables in `env` are injected before execution via `vm.Set(k, v)`. Used by triggers to inject `event`.

### New Go backend function

`whatsapp.SendDocumentFile(to types.JID, data []byte, filename, caption string, reply *ReplyInfo)`

Sets both `FileName` and `Caption` on the `DocumentMessage` proto.

---

## 5. Media Gallery

### Backend

**Endpoint:** `GET /zaplab/api/media/gallery`

Query params: `type` (image|video|audio|document|sticker), `chat`, `limit` (default 50), `offset` (default 0)

Queries `events WHERE type='Message' AND file != ''`.

Media type detection via SQLite CASE on `json_extract` field presence:
- `ImageMessage` → image
- `VideoMessage` → video
- `AudioMessage` → audio
- `DocumentMessage` → document
- `StickerMessage` → sticker

File URL: `/api/files/events/<id>/<file>`
Thumb URL: `/api/files/events/<id>/<file>?thumb=300x300`

**Response:**
```json
{
  "items": [
    {
      "id": "pb_record_id",
      "msgID": "AABBCCDD...",
      "chat": "5511999@s.whatsapp.net",
      "sender": "5511888@s.whatsapp.net",
      "is_from_me": false,
      "media_type": "image",
      "file_url": "/api/files/events/.../filename.jpg",
      "thumb_url": "/api/files/events/.../filename.jpg?thumb=300x300",
      "caption": "optional caption",
      "created": "2026-03-17 12:00:00.000Z"
    }
  ],
  "total": 250,
  "limit": 50,
  "offset": 0
}
```

**File:** `internal/api/gallery.go`

### Frontend

Section ID: `gallery` | JS file: `pb_public/js/sections/gallery.js` | State prefix: `gl`

Key state: `glItems`, `glTotal`, `glLimit`, `glOffset`, `glTypeFilter`, `glChatFilter`, `glLightboxItem`

Key methods: `glLoad(resetOffset)`, `glApplyFilters()`, `glNextPage()`, `glPrevPage()`, `glOpenLightbox(item)`, `glCloseLightbox()`

- Responsive grid (Tailwind `grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5`).
- Each card shows thumbnail (or type icon fallback) + type badge + date.
- Lightbox: `<img>` for images, `<video controls>` for video, `<audio controls>` for audio, download link for documents/stickers.
- Escape key closes lightbox.
- Pagination with `glPageLabel()` showing `from–to of total`.
