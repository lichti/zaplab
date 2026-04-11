# Activity Tracker — Technical Specification

## Overview

The Device Activity Tracker infers whether a WhatsApp contact's device is **Online**, **Standby**, or **Offline** by measuring the round-trip time (RTT) of low-level protocol probes. It ports the algorithm from [`gommzystudio/device-activity-tracker`](https://github.com/gommzystudio/device-activity-tracker) (Node/Baileys) to Go/whatsmeow.

> **Important:** WhatsApp only delivers delivery receipts for contacts that have you saved. Non-mutual contacts are silently ignored by the server regardless of probe volume.

> **Risk:** Sending continuous probes may be detected by WhatsApp and result in account suspension. Use only with your own accounts or with explicit consent.

---

## Feature Flag

The tracker is disabled by default. Enable/disable persisted in `config.json` as `activity_tracker_enabled`.

```
POST /zaplab/api/activity-tracker/enable
POST /zaplab/api/activity-tracker/disable   → also stops all active trackers
```

---

## Algorithm

### Probe methods

| Method | Probe message | Notes |
|--------|--------------|-------|
| `delete` | `ProtocolMessage{Type: REVOKE, Key: {fakeID}}` | Default |
| `reaction` | `ReactionMessage{Key: {fakeID}, Text: "👍"}` | Alternative |

In both cases the *outer* message ID (`SendRequestExtra{ID: probeID}`) is controlled. The delivery receipt arrives referencing `probeID`.

### RTT measurement

1. Record `sentAt = time.Now()` before sending.
2. Register `probeID → channel` in `atProbeWaiters`.
3. The `events.Receipt` handler calls `NotifyProbeReceipt(msgID)` for each message ID in the receipt.
4. `NotifyProbeReceipt` signals the channel with `rtt = time.Since(sentAt).Milliseconds()`.
5. Timeout of 10 s without receipt → state = `Offline`, rtt_ms = -1.

### State classification

```
recentRTTs  = last 3 samples (moving average)
globalRTTs  = last 2000 samples (global median baseline)
threshold   = globalMedian × 0.90

if len(globalRTTs) < 5 → Online  (not enough baseline)
else if movingAvg(recentRTTs) < threshold → Online
else → Standby
timeout → Offline
```

### Probe interval

```
interval = 2s + rand(0..500ms)  // ~2-2.5s between probes
```

---

## Data Model

### `device_activity_sessions`

| Field | Type | Description |
|-------|------|-------------|
| `jid` | text | Tracked contact JID |
| `probe_method` | text | `delete` or `reaction` |
| `started_at` | text | ISO-8601 timestamp |
| `stopped_at` | text | ISO-8601 timestamp (empty while running) |
| `created` | datetime | Auto |

### `device_activity_probes`

| Field | Type | Description |
|-------|------|-------------|
| `session_id` | text | FK → `device_activity_sessions.id` |
| `jid` | text | Tracked contact JID |
| `rtt_ms` | number | RTT in milliseconds; -1 on timeout |
| `state` | text | `Online`, `Standby`, or `Offline` |
| `median_ms` | number | Global median at time of probe |
| `threshold_ms` | number | Classification threshold (median × 0.90) |
| `created` | datetime | Auto |

---

## API Reference

All endpoints require `X-API-Token`. Mutating endpoints are audit-logged.

### Feature flag

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/zaplab/api/activity-tracker/enable` | Enable the feature (persists to config.json) |
| `POST` | `/zaplab/api/activity-tracker/disable` | Disable the feature + stop all trackers |

### Tracker management

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/zaplab/api/activity-tracker/status` | Feature flag + active trackers snapshot |
| `POST` | `/zaplab/api/activity-tracker/start` | Start tracker for one JID |
| `POST` | `/zaplab/api/activity-tracker/stop` | Stop tracker for one JID |
| `POST` | `/zaplab/api/activity-tracker/start-bulk` | Start trackers for a list of JIDs |
| `POST` | `/zaplab/api/activity-tracker/stop-all` | Stop all trackers (keeps feature enabled) |

### Contacts & history

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/zaplab/api/activity-tracker/contacts` | Known individual contacts enriched with tracking state |
| `GET` | `/zaplab/api/activity-tracker/history` | Probe history for a JID |

### Request / response examples

#### `GET /status`
```json
{
  "enabled": true,
  "trackers": [
    { "jid": "5511999@s.whatsapp.net", "session_id": "abc123", "probe_method": "delete", "state": "Online" }
  ]
}
```

#### `POST /start`
```json
// Request
{ "jid": "5511999999999", "probe_method": "delete" }

// Response
{ "jid": "5511999999999@s.whatsapp.net", "session_id": "abc123", "probe_method": "delete" }
```

#### `POST /start-bulk`
```json
// Request
{ "jids": ["5511999999999", "5522888888888"], "probe_method": "delete" }

// Response
{ "started": 2, "skipped": 0, "failed": 0 }
```

#### `GET /contacts`
```json
{
  "contacts": [
    { "jid": "5511999@s.whatsapp.net", "phone": "5511999", "name": "John", "tracking": true, "state": "Online" },
    { "jid": "5522888@s.whatsapp.net", "phone": "5522888", "name": "Jane", "tracking": false, "state": "" }
  ],
  "total": 2
}
```

#### `GET /history?jid=5511999@s.whatsapp.net&days=1&limit=200`
```json
{
  "probes": [
    { "id": "...", "session_id": "abc123", "jid": "5511999@s.whatsapp.net", "rtt_ms": 312, "state": "Online", "median_ms": 450.5, "threshold_ms": 405.45, "created": "2026-04-11 16:00:00.000Z" }
  ],
  "total": 1
}
```

---

## In-memory state

Each tracked JID runs one goroutine (`atTracker.run`). State is kept in memory:

```go
type atTracker struct {
    JID          types.JID
    SessionID    string
    ProbeMethod  string
    cancel       context.CancelFunc
    recentRTTs   []float64   // last 3 samples
    globalRTTs   []float64   // last 2000 samples
    CurrentState ATState
}
```

The global map `atTrackers map[string]*atTracker` is protected by `atMu sync.RWMutex`.
Receipt correlation uses `atProbeWaiters map[string]*probeWaiter` protected by `atProbeMu sync.Mutex`.

---

## Frontend

Section: **Activity Tracker** (sidebar icon: signal/radar)

### Active Trackers tab
- Start form: JID input + probe method selector
- Live tracker list with Online/Standby/Offline badges (5s auto-poll)
- Stop button per tracker; History button jumps to Probe History tab

### Contacts tab
- Full contact list from whatsmeow store (groups excluded)
- Search by name, phone, or JID
- Checkbox per row (click anywhere on row to toggle)
- Quick-select: **All** / **None** / **Untracked**
- Actions: **Track Selected (N)** / **Track All** / **Stop All**
- Already-tracked contacts show state badge + Stop/History inline

### Probe History tab
- Filter by JID, look-back period, and limit
- Table: state badge, RTT (color-coded: green <500ms, yellow <1500ms, red/timeout), median, threshold, timestamp
