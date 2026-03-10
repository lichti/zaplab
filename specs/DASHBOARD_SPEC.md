# Dashboard Spec

> Frontend-only feature — no new API endpoints.
> Uses the existing PocketBase SDK directly from the browser plus the `/zaplab/api/account` endpoint.

---

## Overview

The Dashboard is the default landing section of the ZapLab UI. It provides a real-time overview
of the running instance:

- **Connection status** — live dot + JID of the connected account
- **Account card** — avatar, push name, phone number, platform
- **Statistics grid** — all-time and last-24h counters for all event categories
- **Recent events** — the 10 most recent records with type badge and content preview
- **Quick Actions** — shortcut buttons navigating to the main UI sections
- **Auto-refresh** — data is re-fetched every 60 seconds; a countdown badge shows seconds until next refresh

---

## Layout

```
┌── Connection status ──────────────────────────┐
│  [●] Connected  ·  user@s.whatsapp.net        │
└───────────────────────────────────────────────┘

┌── Account ──────────────────────────────────────────────────────────────┐
│  [avatar]  Push Name          Phone          Platform                   │
└─────────────────────────────────────────────────────────────────────────┘

┌── All time ───────────────────────────────────────────────────────────┐
│  Total Events  Received  Sent  Edited  Deleted  Errors                │
│  [n]           [n]       [n]   [n]     [n]      [n]                   │
└───────────────────────────────────────────────────────────────────────┘

┌── Last 24h ───────────────────────────────────────────────────────────┐
│  Total Events  Received  Sent  Edited  Deleted  Errors                │
│  [n]           [n]       [n]   [n]     [n]      [n]                   │
└───────────────────────────────────────────────────────────────────────┘

┌── Recent Events ──────────────────────────────────────────────────────┐
│  [type badge]  [preview text]                       [timestamp]       │
│  ...                                                                  │
└───────────────────────────────────────────────────────────────────────┘

┌── Quick Actions ───────────────────────────────────────────────────────┐
│  [Events] [Event Browser] [Message History] [Send] [Groups] [Spoof]   │
└────────────────────────────────────────────────────────────────────────┘
```

---

## Statistics

### All-time counters (PocketBase `events` and `errors` collections)

| Counter | PocketBase Query |
|---|---|
| Total Events | `events` — no filter |
| Received | `events` — `type ~ 'Message' && type != 'SentMessage'` |
| Sent | `events` — `type = 'SentMessage'` |
| Edited | `events` — `type = 'Message' && raw ~ 'IsEdit'` |
| Deleted | `events` — `type = 'Message' && raw ~ 'protocolMessage' && raw !~ 'IsEdit'` |
| Errors | `errors` — no filter |

### Last-24h counters

Same filters with `&& created >= '<ISO timestamp>'` appended. The 24h cutoff is computed
client-side: `new Date(Date.now() - 86400000).toISOString().slice(0,19).replace('T',' ')`.

---

## Parallel Queries

All 13 PocketBase queries (12 count queries + 1 recent-events list) are dispatched simultaneously
via `Promise.allSettled`. Each call includes `{ requestKey: null }` to prevent the PocketBase JS
SDK from auto-cancelling concurrent requests to the same collection.

Individual query failures are logged to the console but do not block the rest of the dashboard
from updating — failed counters fall back to `0` and the recent events list falls back to `[]`.

---

## Auto-refresh

A single `setInterval` starts in `initDashboard()` and fires every 60 seconds, calling
`dashFetch()`. A `dash.countdown` counter decrements every second via a separate `setInterval`
and is displayed in the dashboard header as "Next refresh in Xs".

---

## JS Implementation

**File:** `pb_public/js/sections/dashboard.js` — `dashboardSection()` factory

**State prefix:** `dash.*`

### State

| Property | Type | Description |
|---|---|---|
| `dash.loading` | `boolean` | True while any query is in flight |
| `dash.fetchError` | `string\|null` | Error message from last failed fetch |
| `dash.lastFetch` | `Date\|null` | Timestamp of last successful fetch |
| `dash.countdown` | `number` | Seconds until next auto-refresh |
| `dash.totalEvents` | `number` | All-time total events count |
| `dash.totalReceived` | `number` | All-time received count |
| `dash.totalSent` | `number` | All-time sent count |
| `dash.totalEdited` | `number` | All-time edited count |
| `dash.totalDeleted` | `number` | All-time deleted count |
| `dash.totalErrors` | `number` | All-time errors count |
| `dash.h24Events` | `number` | Last-24h total events |
| `dash.h24Received` | `number` | Last-24h received |
| `dash.h24Sent` | `number` | Last-24h sent |
| `dash.h24Edited` | `number` | Last-24h edited |
| `dash.h24Deleted` | `number` | Last-24h deleted |
| `dash.h24Errors` | `number` | Last-24h errors |
| `dash.recentEvents` | `array` | Last 10 event records |

### Methods

| Method | Description |
|---|---|
| `initDashboard()` | Starts 60s refresh interval and 1s countdown interval; fetches initial data |
| `dashFetch()` | Runs all 13 PocketBase queries via `Promise.allSettled`, updates state |
| `dashConnStatus()` | Returns `'connected'` / `'connecting'` / `'disconnected'` from `connectionStatus` |
| `dashJID()` | Returns the account JID from `accountData` |

### Alpine Reactivity Note

All stat counters are updated with individual property assignments
(`this.dash.totalEvents = val(0)`, etc.) rather than `Object.assign(this.dash, {...})`.
Alpine.js v3 tracks reactivity through Proxy setter calls — bulk assignment via
`Object.assign` bypasses individual setters and fails to trigger template updates for
nested objects.

---

## Files Changed

| File | Change |
|---|---|
| `pb_public/js/sections/dashboard.js` | New — `dashboardSection()` factory |
| `pb_public/js/zaplab.js` | Added `dashboardSection()` to `Object.assign`, `this.initDashboard()` first in `init()` |
| `pb_public/index.html` | Added `<script src>` tag, nav button (bar-chart icon), and full Dashboard section HTML |
| `pb_public/js/sections/events.js` | Added `requestKey: null` to `loadInitialEvents` to prevent SDK auto-cancellation |
| `pb_public/js/utils.js` | Added null/undefined guard to `highlight()` to prevent Alpine crash on null input |
