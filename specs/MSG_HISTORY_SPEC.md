# Message History Spec

> Frontend-only feature — no new API endpoints.
> Uses the existing PocketBase SDK (`pb.collection('events').getList()`) directly from the browser.

---

## Overview

The Message History section surfaces all edited and deleted WhatsApp messages captured in the
PocketBase `events` collection. It provides:

- Filtered list of edit/delete action events (kind, sender, chat, date range)
- Per-event detail panel showing the action payload and the **original message** content
- Automatic cross-reference lookup: given the target message ID from the action event, a second
  PocketBase query fetches the original message record and renders its content, media, and JSON

---

## Layout

```
┌── Filter bar (sticky) ────────────────────────────────────────────────┐
│  Kind [All/Edited/Deleted]  Sender [text]  Chat [text]                │
│  From [date]  To [date]                       [Search]  [Reset]       │
└───────────────────────────────────────────────────────────────────────┘
┌── Event list (w-80, scroll) ──┬── Detail panel (flex-1, scroll) ──────┐
│  N total, M loaded            │  [kind badge]  [datetime]  [sender]   │
│                               │  [chat]  [target msgID]  [Copy JSON]  │
│  [time] [Edited|Deleted]      │──────────────────────────────────────  │
│  [sender] [chat]              │  Action Event                         │
│  [→ new content or target ID] │  ├─ [Edit] new content card           │
│                               │  │  OR [Delete] target ID card        │
│  ...                          │  └─ syntax-highlighted JSON           │
│  [Load more (N remaining)]    │                                       │
│                               │  Original Message                     │
│                               │  ├─ Loading spinner                   │
│                               │  ├─ Not found notice                  │
│                               │  └─ [type badge] preview text         │
│                               │     [media preview + download]        │
│                               │     [Full Payload JSON]  [Copy]       │
└───────────────────────────────┴───────────────────────────────────────┘
```

---

## Filters

| Filter | PocketBase filter | Notes |
|---|---|---|
| Kind = All | `type = 'Message' && (raw ~ '"Edit":"1"' \| raw ~ '"Edit":"7"' \| raw ~ '"Edit":"8"')` | Edit OR delete |
| Kind = Edited | `type = 'Message' && raw ~ '"Edit":"1"'` | `Info.Edit="1"` (MessageEdit) set by whatsmeow |
| Kind = Deleted | `type = 'Message' && (raw ~ '"Edit":"7"' \| raw ~ '"Edit":"8"')` | `Info.Edit="7"` (SenderRevoke) or `"8"` (AdminRevoke) |
| Sender | `raw ~ 'VALUE'` | Substring match — JID or PushName |
| Chat / Group | `raw ~ 'VALUE'` | Substring match — group or individual chat JID |
| Date From | `created >= 'YYYY-MM-DD 00:00:00'` | Native `<input type="date">` |
| Date To | `created <= 'YYYY-MM-DD 23:59:59'` | Native `<input type="date">` |

All active filters are combined with `&&`. Pressing `Enter` triggers the search.

---

## Kind Classification (client-side)

After records are loaded, each item is classified in JavaScript (`mhKind(item)`):

| Condition | Kind |
|---|---|
| `Info.Edit === "1"` | `edit` |
| `IsEdit === true` (legacy — not reliably set) | `edit` |
| `Message.protocolMessage.type === 14` | `edit` (MESSAGE_EDIT) |
| `Info.Edit === "7"` or `"8"` | `delete` (SenderRevoke / AdminRevoke) |
| `Message.protocolMessage` present, other type | `delete` (fallback) |

> `IsEdit` is `false` for edits received from other clients because whatsmeow's `UnwrapRaw()`
> only sets it when the outer message is wrapped in an `editedMessage` proto field. Client-side
> edits (sent from your own device) arrive as a top-level `protocolMessage` with `type=14`,
> bypassing that unwrap path. `Info.Edit="1"` is the authoritative indicator.

---

## Original Message Lookup

The action event contains the ID of the targeted message at:

```
raw.Message.protocolMessage.key.ID
```

whatsmeow serializes this field with uppercase `ID` (Go struct field name). The JS helper
`_mhOriginalId()` tries `key.ID`, `key.Id`, and `key.id` in order for robustness.

Once the target ID is known, a second PocketBase query fetches the original record:

```js
pb.collection('events').getList(1, 5, {
  sort:   '-created',
  filter: `msgID = '${targetId}'`,
})
```

The first matching record is displayed. If no record is found, a "not found" notice is shown
with the target ID so the user knows which message was not captured.

---

## Detail Panel

### Action Event

Shows a kind-specific summary card followed by the full syntax-highlighted JSON:

- **Edited**: blue card with the new text content extracted from
  `Message.protocolMessage.editedMessage.message.conversation` (or caption for media)
- **Deleted**: red card with the target message ID

### Original Message

| State | Display |
|---|---|
| Loading | Spinner in section header |
| Not found | Notice text + target msgID |
| Found | Summary card (type badge, preview text, timestamp) + optional media + Full Payload JSON |

Media reuses the shared `ebMediaType()`, `ebFileUrl()`, `ebThumbUrl()` helpers from
`eventBrowserSection` (available on `this` via `Object.assign` merge).

---

## JS Implementation

**File:** `pb_public/js/sections/msghistory.js` — `msgHistorySection()` factory

**State prefix:** `mh.*`

| Method | Description |
|---|---|
| `initMsgHistory()` | No-op |
| `_mhEsc(s)` | Escapes `'` and `\` for PocketBase filter strings |
| `_mhBuildFilter()` | Assembles filter string from kind + sender + chat + dates |
| `mhSearch()` | Resets to page 1, calls `pb.collection('events').getList()` |
| `mhLoadMore()` | Increments page, appends results |
| `mhReset()` | Clears all filters and results |
| `mhSelect(item)` | Sets `mh.selected`, then fetches the original message |
| `mhHasMore()` | `mh.items.length < mh.total` |
| `_mhRaw(item)` | Returns parsed `item.raw` as an object |
| `_mhOriginalId(item)` | Extracts target message ID from `Message.protocolMessage.key.ID` |
| `mhKind(item)` | `'edit'` \| `'delete'` \| `'unknown'` |
| `mhKindLabel(kind)` | `'Edited'` \| `'Deleted'` |
| `mhKindBadgeClass(kind)` | Tailwind classes for kind badge |
| `mhSenderLabel(item)` | PushName or Sender JID prefix |
| `mhChatLabel(item)` | Chat/group JID prefix |
| `mhTargetId(item)` | Same as `_mhOriginalId(item)` — used in templates |
| `mhNewContent(item)` | Extracts new text/caption from edit payload |
| `mhFmtDateTime(iso)` | Full locale date+time string |
| `mhCopyJSON()` | Copies action event JSON to clipboard |
| `mhCopyOrigJSON()` | Copies original message JSON to clipboard |

---

---

## Edit Diff

When an edited message is selected and its original message is found, the detail panel shows
a word-level visual diff between the original and new content.

### Algorithm

- **Tokenization**: text is split into words and whitespace runs (`/\S+|\s+/g`).
- **LCS**: a flat `Int32Array` DP table of size `(m+1)*(n+1)` computes the longest common
  subsequence between the two token arrays, then backtracks to produce `{type, val}` operations
  (`eq` / `del` / `ins`).
- **Fallback**: texts with more than 400 tokens on either side use block-level diff (whole
  old text struck through, whole new text highlighted) to avoid O(n²) freeze.

### CSS Classes

| Class | Meaning | Style |
|---|---|---|
| `.diff-del` | Removed tokens | Red background, strikethrough |
| `.diff-ins` | Inserted tokens | Green background |

Both classes have dark-mode and light-mode variants defined in `pb_public/css/zaplab.css`.

### JS Methods

| Method | Description |
|---|---|
| `_mhTokenize(text)` | Splits text into word/whitespace tokens |
| `_mhLCS(a, b)` | LCS DP + backtrack — returns `[{type, val}]` operations |
| `mhDiffHtml(item)` | Calls `_mhLCS`, maps ops to HTML spans, returns safe HTML string |

`mhDiffHtml` is bound to `x-html` in the Edit Diff panel. Returns `null` when either side
is unavailable (panel hidden via `x-if`).

---

## Files Changed

| File | Change |
|---|---|
| `pb_public/js/sections/msghistory.js` | New — `msgHistorySection()` factory with LCS diff methods |
| `pb_public/js/zaplab.js` | Added `msgHistorySection()` to `Object.assign`, `this.initMsgHistory()` to `init()` |
| `pb_public/index.html` | Added `<script src>` tag, nav button (eye-slash icon), full section HTML, and Edit Diff panel |
| `pb_public/css/zaplab.css` | Added `.diff-del` and `.diff-ins` classes (dark + light mode) |
