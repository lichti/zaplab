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
| Kind = All | `type = 'Message' && (raw ~ '"IsEdit":true' \| raw ~ 'protocolMessage')` | Edit OR delete |
| Kind = Edited | `type = 'Message' && raw ~ '"IsEdit":true'` | `IsEdit` flag set by whatsmeow |
| Kind = Deleted | `type = 'Message' && raw ~ 'protocolMessage' && raw !~ '"IsEdit":true'` | Protocol message without edit flag |
| Sender | `raw ~ 'VALUE'` | Substring match — JID or PushName |
| Chat / Group | `raw ~ 'VALUE'` | Substring match — group or individual chat JID |
| Date From | `created >= 'YYYY-MM-DD 00:00:00'` | Native `<input type="date">` |
| Date To | `created <= 'YYYY-MM-DD 23:59:59'` | Native `<input type="date">` |

All active filters are combined with `&&`. Pressing `Enter` triggers the search.

---

## Kind Classification (client-side)

After records are loaded, each item is classified in JavaScript:

| Condition | Kind |
|---|---|
| `raw.IsEdit === true` | `edit` |
| `raw.Message.protocolMessage.type === 14` | `edit` (MESSAGE_EDIT) |
| `raw.Message.protocolMessage` present, type ≠ 14 | `delete` (REVOKE = 0) |

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

## Files Changed

| File | Change |
|---|---|
| `pb_public/js/sections/msghistory.js` | New — `msgHistorySection()` factory |
| `pb_public/js/zaplab.js` | Added `msgHistorySection()` to `Object.assign`, `this.initMsgHistory()` to `init()` |
| `pb_public/index.html` | Added `<script src>` tag, nav button (eye-slash icon), and full section HTML |
