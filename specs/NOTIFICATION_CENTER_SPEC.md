# Notification Center — Technical Specification

## Overview

The Notification Center is an in-app persistent alert system that surfaces important events directly in the ZapLab dashboard without requiring the user to monitor individual sections. Alerts are created automatically by backend triggers, persisted in a dedicated PocketBase collection, delivered in real time via the existing SSE broker, and managed through a dedicated UI section.

---

## Architecture

```
WhatsApp event
       │
       ▼
  Go trigger
  (mentions.go / activitytracker.go / webhookdelivery.go)
       │
       ▼
  CreateNotification()          ←── internal/whatsapp/notifications.go
  ├── pb.Save(record)           ←── writes to `notifications` collection
  └── ssePublish("Notification", data)  ←── broadcasts to all SSE subscribers
                                          │
                                          ▼
                              Browser (events.js SSE stream)
                              ├── eventsSection receives SSE message
                              └── notificationsSection.$watch('events')
                                  updates nc.notifications + nc.unreadCount
```

---

## Database Collection: `notifications`

**Migration:** `1749000000_create_notifications.go`
**Collection ID:** `ntf1a2b3c4d5e6f7`

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | text (auto) | — | PocketBase record ID |
| `type` | text | ✓ | `mention` \| `tracker_state` \| `webhook_failure` |
| `title` | text | ✓ | Short human-readable label (e.g. "You were mentioned") |
| `body` | text | — | Full description with context |
| `entity_id` | text | — | Related record ID (mention ID, session ID, delivery ID) |
| `entity_jid` | text | — | Related JID (sender for mentions, tracked JID for tracker) |
| `read_at` | text | — | ISO 8601 timestamp; empty string = unread |
| `data` | json | — | Raw payload (≤ 8 192 bytes) for additional context |
| `created` | autodate | — | Auto-populated on insert |
| `updated` | autodate | — | Auto-populated on update |

**Indexes:** `idx_ntf_type`, `idx_ntf_readat`, `idx_ntf_created`

**Access rules:** ListRule, ViewRule, UpdateRule, DeleteRule all set to `""` (any authenticated user).

---

## Trigger Points

### 1. Mention (`mention`)

**File:** `internal/whatsapp/mentions.go` — `DetectAndRecordMentions()`

**Condition:** After saving a mention record where `is_bot = true` (the connected device's own JID was @mentioned).

**Payload:**
```json
{
  "message_id": "<WA message ID>",
  "chat_jid":   "<chat JID>",
  "sender_jid": "<sender JID>",
  "context":    "<first 200 chars of message text>"
}
```

---

### 2. Tracker State Change (`tracker_state`)

**File:** `internal/whatsapp/activitytracker.go` — `(*atTracker).probe()`

**Condition:** After each probe completes, if `prev != state` (the inferred device state changed).

**States:** `Online` | `Standby` | `Offline`

**Payload:**
```json
{
  "jid":      "<tracked JID>",
  "previous": "Online",
  "current":  "Offline",
  "rtt_ms":   -1
}
```

Note: `rtt_ms = -1` means probe timed out (→ `Offline`).

---

### 3. Webhook Failure (`webhook_failure`)

**File:** `internal/whatsapp/webhookdelivery.go` — `InitWebhookDeliveryLogger()` callback

**Condition:** After saving a delivery record where `status == "failed"` (all retries exhausted).

**Payload:**
```json
{
  "webhook_url": "https://n8n.example.com/webhook/abc",
  "event_type":  "Message",
  "attempt":     3,
  "http_status": 503,
  "error_msg":   "connection refused"
}
```

---

## Backend API

See [API_SPEC.md](API_SPEC.md#notification-center) for full endpoint documentation.

| Method | Path | Description |
|---|---|---|
| `GET` | `/zaplab/api/notifications` | List (filterable by status, type, limit, offset) |
| `PUT` | `/zaplab/api/notifications/:id/read` | Mark one as read |
| `POST` | `/zaplab/api/notifications/read-all` | Mark all unread as read |
| `DELETE` | `/zaplab/api/notifications/:id` | Delete one |
| `POST` | `/zaplab/api/notifications/purge` | Delete all read |

All endpoints require authentication (`X-API-Token` or PocketBase session).

---

## SSE Real-Time Delivery

`CreateNotification()` calls `ssePublish("Notification", data)` immediately after saving the DB record. The payload is delivered to all active SSE clients on the existing `/zaplab/api/events/stream` connection as a `message` event with `type: "Notification"`.

The `notificationsSection()` frontend watches `this.events` (updated by `eventsSection` SSE listener) for items with `type === "Notification"` and prepends them to `nc.notifications`, incrementing `nc.unreadCount`. This reuses the existing SSE infrastructure with no additional connections.

---

## Frontend

**File:** `pb_public/js/sections/notifications.js`

### State

```javascript
nc: {
  notifications: [],  // loaded/received notification records
  unreadCount:   0,   // always reflects total unread (even outside section)
  loading:       false,
  error:         '',
  tab:           'unread', // 'unread' | 'all'
  typeFilter:    '',
}
```

### Methods

| Method | Description |
|---|---|
| `ncLoad()` | Fetch from API with current tab/type filters |
| `ncMarkRead(id)` | PUT /:id/read + optimistic local update |
| `ncMarkAllRead()` | POST /read-all + local state update |
| `ncDelete(id)` | DELETE /:id + remove from local list |
| `ncPurge()` | POST /purge + reload |
| `initNotifications()` | Hooks `$watch('activeSection')` for auto-load; hooks `$watch('events')` for SSE real-time updates |

### UI Components

- **Topbar bell icon** — bell SVG, yellow tint when `nc.unreadCount > 0`; red badge with count (`9+` cap); clicking navigates to the `notifications` section.
- **Sidebar link** — bell icon with inline red counter pill when unread.
- **Section header** — title, "Mark all read" button (disabled when count=0), "Purge read" button, reload button.
- **Tabs** — Unread (with count) / All.
- **Type dropdown filter** — All types / Mentions / Tracker state / Webhook failures.
- **Notification cards** — type icon (@ blue for mention, ⚡ purple for tracker, ⚠ red for webhook); title + `NEW` badge for unread; body text; timestamp; mark-read checkmark (unread only); delete X button. Unread cards have a subtle blue tint border.
- **Empty state** — illustrated empty state with contextual message.

---

## Design Decisions

1. **Reuse existing SSE stream** — rather than a separate WebSocket or polling loop, the Notification Center piggybacks on the shared SSE broker already used by Live Events. No new infrastructure needed.

2. **`read_at` as text (not datetime)** — keeps the schema simple; empty string = unread, ISO 8601 string = read timestamp. Avoids SQLite NULL quirks with PocketBase datetime fields.

3. **`tracker_state` notifications on every transition** — probes fire every ~2 s. State changes are relatively rare (Online↔Standby↔Offline), so the volume is manageable. Noise is further reduced because transitions only notify (not every probe result).

4. **`webhook_failure` only on final failure** — the `OnDelivery` callback fires for every attempt. Notifications are created only when `status == "failed"` (final outcome), not on intermediate retry failures, to avoid alert fatigue.

5. **No separate config flags (beta)** — all three notification types are always enabled. Per-type opt-out toggles can be added in a future iteration (e.g., `Config.NotificationTypes []string`).
