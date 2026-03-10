# Webhook Spec — ZapLab

---

## Overview

The webhook system dispatches events to external HTTP endpoints. Configuration is persisted in `webhook.json` (inside the data directory) and managed at runtime via the REST API or the **Webhooks** UI section — no restart required.

---

## Webhook types

| Type | Routing key | Fires when |
|---|---|---|
| **Default** | — | Every event that is persisted (`saveEvent` / `saveEventFile`) |
| **Error** | — | Every error persisted to the `errors` collection (`saveError`) |
| **Event-type** | `event_type` pattern | Event type matches exact name or wildcard prefix |
| **Text-pattern** | `id` (random) | Incoming/outgoing text message matches pattern + sender filter |
| **Cmd** *(legacy)* | command string | Message text starts with a registered command (e.g. `/order`) |

Event-type and text-pattern webhooks fire **in addition** to the default webhook.

---

## Payload structure

All webhooks receive a JSON array (even single events are wrapped in an array):

```json
[
  {
    "type":  "<event type string>",
    "raw":   { /* full whatsmeow event struct */ },
    "extra": null
  }
]
```

The `extra` field is non-null only for poll votes (decrypted ballot) and reactions.

Text-pattern webhooks always use `"type": "Message.Text"` as the envelope type.

---

## Event type strings

| Type string | Source |
|---|---|
| `Message` | Plain text / extended text |
| `Message.ImageMessage` | Received image |
| `Message.AudioMessage` | Received audio / PTT |
| `Message.VideoMessage` | Received video |
| `Message.DocumentMessage` | Received document |
| `Message.StickerMessage` | Received sticker |
| `Message.ContactMessage` | Received vCard contact |
| `Message.LocationMessage` | Received static location |
| `Message.LiveLocationMessage` | Received live location update |
| `Message.PollUpdateMessage` | Poll vote (decrypted) |
| `Message.EncReactionMessage` | Encrypted reaction (decrypted) |
| `ReceiptRead` | Message read by recipient |
| `ReceiptReadSelf` | Message read on another device |
| `ReceiptDelivered` | Message delivered |
| `ReceiptPlayed` | Audio / video played |
| `ReceiptSender` | Sender receipt |
| `ReceiptRetry` | Retry receipt |
| `Presence.Online` | Contact came online |
| `Presence.Offline` | Contact went offline |
| `Presence.OfflineLastSeen` | Contact offline with last-seen time |
| `HistorySync` | History sync batch |
| `AppStateSyncComplete` | App state sync complete |
| `Connected` | WhatsApp connection established |
| `PushNameSetting` | Push name updated |
| `SentMessage` | Outgoing message successfully sent |

---

## Event-type webhook

### Config schema

```json
{
  "event_type": "Message.*",
  "webhook": { "Scheme": "https", "Host": "example.com", "Path": "/messages" }
}
```

### Wildcard matching

`matchesEventType(pattern, evtType string) bool` in `internal/webhook/webhook.go`:

- **Exact**: `"Message.ImageMessage"` matches only `"Message.ImageMessage"`
- **Wildcard prefix**: `"Message.*"` matches `"Message"` and any `"Message.X"` subtype
- **All**: `"*"` is not supported; use the default webhook for catch-all

### Dispatch path

`saveEvent` / `saveEventFile` in `persist.go` → `wh.SendToEventWebhooks(evtType, raw, nil)` → goroutine per matching entry.

---

## Text-pattern webhook

### Config schema

```json
{
  "id":             "a1b2c3d4",
  "match_type":     "prefix",
  "pattern":        "/ping",
  "from":           "others",
  "case_sensitive": false,
  "webhook":        { "Scheme": "https", "Host": "example.com", "Path": "/ping" }
}
```

### Fields

| Field | Type | Values | Description |
|---|---|---|---|
| `id` | string | auto-generated | Random 4-byte hex ID assigned on creation |
| `match_type` | string | `prefix` / `contains` / `exact` | How to match the message text |
| `pattern` | string | any | Text to search for |
| `from` | string | `all` / `me` / `others` | Sender filter: `me` = `IsFromMe == true`, `others` = `IsFromMe == false` |
| `case_sensitive` | bool | `true` / `false` | When `false` (default), both sides are lowercased before comparison |
| `webhook` | url.URL | — | Destination URL |

### Matching logic

`matchesText(pattern, matchType, text string, caseSensitive bool) bool`:

| `match_type` | Logic |
|---|---|
| `prefix` | `strings.HasPrefix(text, pattern)` |
| `contains` | `strings.Contains(text, pattern)` |
| `exact` | `text == pattern` |

When `caseSensitive == false`, both `pattern` and `text` are lowercased before comparison.

### Dispatch path

`events.go` `*events.Message` handler → `wh.SendToTextWebhooks(getMsg(evt), evt.Info.IsFromMe, rawEvt, nil)` → goroutine per matching entry.

Matching runs on `getMsg(evt)` output: `evt.Message.Conversation` or `evt.Message.ExtendedTextMessage.Text`. Media captions are not matched.

The call is placed immediately after the meta log line, before any command handling — so all text messages (including commands) are evaluated.

---

## REST API

All endpoints are under `/zaplab/api/webhook`.

### `GET /zaplab/api/webhook`

Returns the complete configuration summary.

**Response:**
```json
{
  "default_url":     "https://example.com/webhook",
  "error_url":       "https://example.com/errors",
  "event_webhooks":  [ { "event_type": "Message.*", "url": "https://example.com/messages" } ],
  "text_webhooks":   [ { "id": "a1b2c3d4", "match_type": "prefix", "pattern": "/ping", "from": "others", "case_sensitive": false, "url": "https://example.com/ping" } ]
}
```

### `PUT /zaplab/api/webhook/default`

Set the default webhook URL.

**Body:** `{ "url": "https://example.com/webhook" }`

### `DELETE /zaplab/api/webhook/default`

Clear the default webhook.

### `PUT /zaplab/api/webhook/error`

Set the error webhook URL.

**Body:** `{ "url": "https://example.com/errors" }`

### `DELETE /zaplab/api/webhook/error`

Clear the error webhook.

### `GET /zaplab/api/webhook/events`

List all event-type webhooks.

**Response:** `{ "event_webhooks": [ { "event_type": "...", "url": "..." } ] }`

### `POST /zaplab/api/webhook/events`

Add or update an event-type webhook (upsert by `event_type`).

**Body:** `{ "event_type": "Message.*", "url": "https://example.com/messages" }`

### `DELETE /zaplab/api/webhook/events`

Remove an event-type webhook.

**Body:** `{ "event_type": "Message.*" }`

### `GET /zaplab/api/webhook/text`

List all text-pattern webhooks.

**Response:** `{ "text_webhooks": [ { "id": "...", "match_type": "...", ... } ] }`

### `POST /zaplab/api/webhook/text`

Add a new text-pattern webhook.

**Body:**
```json
{
  "match_type":     "prefix",
  "pattern":        "/ping",
  "from":           "others",
  "case_sensitive": false,
  "url":            "https://example.com/ping"
}
```

### `DELETE /zaplab/api/webhook/text`

Remove a text-pattern webhook by ID.

**Body:** `{ "id": "a1b2c3d4" }`

### `POST /zaplab/api/webhook/test`

Send a test payload `{ "test": true, "source": "zaplab", "message": "webhook test payload" }` to an arbitrary URL.

**Body:** `{ "url": "https://example.com/test" }`

---

## Frontend — Webhooks section

File: `pb_public/js/sections/webhook.js`
Factory: `webhookSection()`
Section key: `'webhook'`

### Layout

Two-column layout:

- **Left column** — Default Webhook card, Error Webhook card, Test Webhook card, toast notification
- **Right column** — Tab bar (Event Type | Text Pattern) + tab content

### Event Type tab

- Dropdown + manual input for selecting the event type
- URL field
- **Add / Update** button (upsert)
- Table of configured event-type webhooks with **Remove** button per row
- Known event types as clickable chips (fills the input)
- Wildcard info box

### Text Pattern tab

- `match_type` select (Starts with / Contains / Exact match)
- `pattern` text input
- `from` select (All / Others only / Me only)
- `case_sensitive` checkbox
- URL input
- **Add** button
- Table with badge-colored `match_type`, pattern, sender filter, case flag, URL, **Remove** button
- Info box explaining sender filter and examples

### State (namespace: `wh.*`)

| Key | Type | Description |
|---|---|---|
| `wh.defaultUrl` | string | Current default webhook URL |
| `wh.errorUrl` | string | Current error webhook URL |
| `wh.eventWebhooks` | array | List of `{ event_type, url }` |
| `wh.textWebhooks` | array | List of `{ id, match_type, pattern, from, case_sensitive, url }` |
| `wh.newEventType` | string | Form input for new event-type webhook |
| `wh.newEventUrl` | string | Form URL for new event-type webhook |
| `wh.newTextMatchType` | string | `"prefix"` / `"contains"` / `"exact"` |
| `wh.newTextPattern` | string | Pattern for new text webhook |
| `wh.newTextFrom` | string | `"all"` / `"me"` / `"others"` |
| `wh.newTextCaseSensitive` | bool | Case-sensitive flag |
| `wh.newTextUrl` | string | URL for new text webhook |
| `wh.testUrl` | string | URL for test webhook |
| `wh.testResult` | object | Response from test endpoint |
| `wh.saving` | bool | In-flight save indicator |
| `wh.testing` | bool | In-flight test indicator |
| `wh.toast` | `{ ok, message }` | Status notification |
| `wh.activeTab` | string | `"event"` / `"text"` |

### Methods

| Method | API call |
|---|---|
| `loadWebhookConfig()` | `GET /webhook` |
| `whSaveDefault()` | `PUT /webhook/default` |
| `whClearDefault()` | `DELETE /webhook/default` |
| `whSaveError()` | `PUT /webhook/error` |
| `whClearError()` | `DELETE /webhook/error` |
| `whAddEventWebhook()` | `POST /webhook/events` |
| `whRemoveEventWebhook(eventType)` | `DELETE /webhook/events` |
| `whAddTextWebhook()` | `POST /webhook/text` |
| `whRemoveTextWebhook(id)` | `DELETE /webhook/text` |
| `whTest()` | `POST /webhook/test` |

---

## Files Changed

| File | Change |
|---|---|
| `internal/webhook/webhook.go` | Added `TextWebhook`, `TextWebhookAPI`, `EventTypeWebhook`, `EventTypeWebhookAPI` structs; `TextWebhooks` and `EventWebhooks` fields on `Config`; all CRUD methods; `matchesText`, `matchesFrom`, `matchesEventType`, `generateID`; `SendToEventWebhooks`, `SendToTextWebhooks`, `SendTo`, `ClearDefaultWebhook`, `ClearErrorWebhook` |
| `internal/whatsapp/events.go` | Added `wh.SendToTextWebhooks(getMsg(evt), evt.Info.IsFromMe, rawEvt, nil)` in `*events.Message` handler |
| `internal/whatsapp/persist.go` | Added `wh.SendToEventWebhooks(evtType, raw, nil)` in both `saveEvent` and `saveEventFile` |
| `internal/api/api.go` | Added `wh *webhook.Config` var; extended `Init`; registered 12 new routes; implemented all handlers |
| `main.go` | Updated `api.Init(app.pb, wh)` |
| `pb_public/js/sections/webhook.js` | New — `webhookSection()` factory with full state and async methods |
| `pb_public/js/zaplab.js` | Added `webhookSection()` to `Object.assign`, `initWebhook()` to `init()` |
| `pb_public/index.html` | Added `<script>` tag, sidebar nav button, full webhook section HTML |
