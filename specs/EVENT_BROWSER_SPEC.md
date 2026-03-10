# Event Browser Spec

> Frontend-only feature — no new API endpoints.
> Uses the existing PocketBase SDK (`pb.collection('events').getList()`) directly from the browser.

---

## Overview

The Event Browser provides a searchable, filterable view of all events stored in the PocketBase `events` collection. It replaces the need to open the PocketBase admin UI for event inspection. Key capabilities:

- Multi-field filtering (type, date range, message ID, sender, recipient, free text)
- Incremental loading (50 per page, load-more)
- Full JSON inspection with syntax highlighting
- Media preview and download (for events with attached files)
- Replay: re-send the event's `Message` payload to any JID via `/zaplab/api/sendraw`
- CSV export: download all matching events (up to 1 000 rows) as a CSV file

---

## Layout

```
┌── Filter bar (sticky) ────────────────────────────────────────────────┐
│  Type [autocomplete]  From [date]  To [date]  MsgID [text]            │
│  Sender [text]  Recipient/Chat [text]  Text (contains) [text]         │
│                                [Search]  [Reset]  [Export CSV]        │
└───────────────────────────────────────────────────────────────────────┘
┌── Event list (w-80, scroll) ──┬── Detail panel (flex-1, scroll) ──────┐
│  N total, M loaded            │  [type badge]  [datetime]  [msgID]    │
│                               │  [Copy JSON]                          │
│  [time] [type] [sender]       │────────────────────────────────────── │
│  [preview text]               │  JSON viewer (syntax highlighted)     │
│                               │                                       │
│  [time] [type] [sender]       │────── Media (if file present) ─────── │
│  [preview text]               │  image / video / audio player         │
│  ...                          │  [Download]                           │
│                               │                                       │
│  [Load more (N remaining)]    │────── Replay via Send Raw ──────────── │
│                               │  To: [input]  [Send Raw]              │
│                               │  [toast]                              │
│                               │  Message payload (preview)            │
└───────────────────────────────┴───────────────────────────────────────┘
```

---

## Filters

| Filter | Field | PocketBase filter | Notes |
|---|---|---|---|
| Type | `type` | `type = 'VALUE'` | Exact match; autocomplete datalist |
| Date From | `created` | `created >= 'YYYY-MM-DD 00:00:00'` | Native `<input type="date">` |
| Date To | `created` | `created <= 'YYYY-MM-DD 23:59:59'` | Native `<input type="date">` |
| Message ID | `msgID` | `msgID = 'VALUE'` | Exact match |
| Sender | `raw` | `raw ~ 'VALUE'` | Contains search — matches JID or PushName inside raw JSON |
| Recipient / Chat | `raw` | `raw ~ 'VALUE'` | Contains search — matches Chat JID inside raw JSON |
| Text (contains) | `raw` | `raw ~ 'VALUE'` | Full-text search inside raw JSON payload |

All active filters are combined with `&&`. Pressing `Enter` in any input triggers the search.

---

## Event List

Each row shows:
- Timestamp (`fmtTime(item.created)`) — compact HH:MM:SS
- Type badge (colored via `typeClass(item.type)`)
- Sender label — extracted from `raw.Info.PushName` or `raw.Info.MessageSource.Sender`
- Preview text — extracted from the `Message` field inside `raw`

Preview text extraction priority (first non-null):
```
raw.Message.conversation
raw.Message.extendedTextMessage.text
raw.Message.imageMessage.caption  || '[image]'
raw.Message.videoMessage.caption  || '[video]'
raw.Message.audioMessage          → '[audio]'
raw.Message.documentMessage.fileName || '[document]'
raw.Message.stickerMessage        → '[sticker]'
raw.Message.locationMessage       → '[location]'
raw.Message.pollCreationMessage   → '[poll: NAME]'
raw.Message.reactionMessage.text  → '[reaction: EMOJI]'
raw.Message.contactMessage        → '[contact: NAME]'
raw.Info.PushName                 → 'from NAME'
JSON.stringify(raw).slice(0, 80)  → fallback
```

---

## Detail Panel

### JSON Viewer

Renders the full PocketBase record (all fields: `id`, `type`, `raw`, `extra`, `file`, `msgID`, `created`, `updated`) via the shared `highlight()` utility.

### Media Section

Shown only when `item.file` is non-empty.

| Media type (by file extension) | Display |
|---|---|
| `jpg`, `jpeg`, `png`, `gif`, `webp` | `<img>` thumbnail (`?thumb=300x300`), click opens full-size in new tab |
| `mp4`, `webm`, `mov`, `mkv` | `<video controls>` player |
| `mp3`, `ogg`, `opus`, `wav`, `m4a`, `aac` | `<audio controls>` player |
| Other | File icon + filename |

File URL pattern: `{origin}/api/files/events/{recordId}/{fileName}`
Thumbnail URL: same + `?thumb=300x300`

The **Download** button is an `<a download>` link pointing to the file URL.

### Replay Panel

Extracts the `Message` field from the event's `raw` payload:
```js
raw.Message || raw.message || raw.whatsapp_message
```

Sends it via `POST /zaplab/api/sendraw` with the user-supplied `To` JID:
```json
{ "to": "<user input>", "message": <extracted Message object> }
```

The payload preview renders the extracted `Message` object with syntax highlighting before the user clicks Send.

If `raw` does not contain a recognizable `Message` field, the Send Raw button is disabled and the preview shows a warning.

---

## JS Implementation

**File:** `pb_public/js/sections/eventbrowser.js` — `eventBrowserSection()` factory

**State prefix:** `eb.*`

| Method | Description |
|---|---|
| `initEventBrowser()` | No-op (no reactive watches needed) |
| `_ebEsc(s)` | Escapes `'` and `\` for PocketBase filter strings |
| `_ebBuildFilter()` | Assembles the PocketBase filter string from active form fields |
| `ebSearch()` | Resets to page 1, calls `pb.collection('events').getList()` |
| `ebLoadMore()` | Increments page, appends results to `eb.items` |
| `ebReset()` | Clears all filters and results |
| `ebSelect(item)` | Sets `eb.selected`, resets replay state |
| `ebHasMore()` | `eb.items.length < eb.total` |
| `_ebRaw(item)` | Returns parsed `item.raw` as an object |
| `ebPreviewText(item)` | Extracts human-readable preview from raw |
| `ebSenderLabel(item)` | Extracts PushName or Sender JID prefix |
| `ebFmtDateTime(iso)` | Full locale date+time string |
| `ebFileUrl(item)` | PocketBase file URL |
| `ebThumbUrl(item)` | PocketBase thumbnail URL (`?thumb=300x300`) |
| `ebMediaType(item)` | `'image'` \| `'video'` \| `'audio'` \| `'file'` \| `null` |
| `ebCopyJSON()` | Copies full JSON of selected record to clipboard |
| `_ebReplayMessage()` | Extracts `Message` object from `raw` |
| `ebReplayHighlight()` | Syntax-highlighted HTML of the replay payload |
| `ebReplay()` | POSTs `{ to, message }` to `/zaplab/api/sendraw` |
| `ebExportCSV()` | Fetches all matching records (up to 1 000), builds and downloads a CSV file |

---

## CSV Export

The **Export CSV** button is enabled only after a search returns at least one result (`eb.total > 0`).

### Behavior

1. Fetches all pages matching the current filter in batches of 200, stopping at 1 000 total records.
2. All calls use `requestKey: null` to avoid SDK auto-cancellation.
3. Builds a CSV string with proper RFC 4180 escaping (values containing commas, quotes, or newlines are double-quoted; internal quotes are doubled).
4. Triggers a browser download as `events_export.csv`.

### CSV Columns

| Column | Source |
|---|---|
| `id` | PocketBase record ID |
| `type` | Event type string |
| `msgID` | WhatsApp message ID |
| `created` | ISO timestamp |
| `sender` | `ebSenderLabel()` — PushName or JID prefix |
| `chat` | Chat/group JID prefix extracted from `raw.Info.MessageSource.Chat` |
| `preview` | `ebPreviewText()` — human-readable message preview |
| `file` | Attached filename (empty if none) |

---

## Media Event Persistence — Implementation Note

Incoming messages with media attachments (image, audio, video, document, sticker, vCard) are saved via `saveEventFile()` in `internal/whatsapp/persist.go`. The attached file is stored in the PocketBase filesystem linked to the event record.

**Correct pattern (PocketBase v0.36):**
```go
file, _ := filesystem.NewFileFromBytes(data, fileName)
record.Set("file", file)   // pass *filesystem.File object
pb.Save(record)             // PocketBase handles upload atomically
```

Setting only the filename string (`record.Set("file", file.Name)`) on a new record is rejected by PocketBase v0.36 validation. The old manual-upload pattern (`pb.NewFilesystem()` + `fs.UploadFile()`) must **not** be used alongside `pb.Save` for file fields.

---

## Files Changed

| File | Change |
|---|---|
| `pb_public/js/sections/eventbrowser.js` | New — `eventBrowserSection()` factory; added `ebExportCSV()` and `eb.exporting` state |
| `pb_public/js/zaplab.js` | Added `eventBrowserSection()` to `Object.assign`, `this.initEventBrowser()` to `init()` |
| `pb_public/index.html` | Added `<script src>` tag, nav button (database icon), full section HTML, and Export CSV button |
