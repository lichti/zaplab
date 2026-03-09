# Route Simulation Spec

> **Status: Work in Progress — not fully functional**
> The live location update mechanism is implemented but does not behave correctly.
> This is a fundamental architectural limitation of the WhatsApp multi-device protocol,
> not a bug that can be fixed within zaplab alone. See the technical analysis below.

---

## Overview

Simulates a moving device sending live WhatsApp location updates along a GPX route.
A background goroutine interpolates the device position at configurable speed and interval,
calling `SendLiveLocation` with incrementing `sequence_number` and computed bearing/speed values.

Each simulation tick saves a `SimulationLocationUpdate` event to the database regardless of
whether the WhatsApp send succeeds, so progress is always visible in Live Events.

---

## Why Live Location Updates Are Broken — Technical Analysis

### The fundamental problem: companion device vs. primary device

zaplab uses [whatsmeow](https://github.com/tulir/whatsmeow), which reimplements the WhatsApp
binary protocol from scratch and always connects as a **companion/linked device** — the same
category as WhatsApp Web, WhatsApp Desktop, etc.

In WhatsApp's multi-device (ADV) architecture:

- **Primary device** — the phone (Android/iOS). It is the authoritative identity anchor.
  It can initiate and update live location shares.
- **Companion device** — a linked secondary device. It can send and receive messages, but
  certain features — including reliable live location initiation and position updates —
  are gated by the server based on device type.

### What whatsmeow can and cannot do

whatsmeow connects via WebSocket + Noise Protocol directly to WhatsApp's servers. It registers
with a companion device flag in the ADV key exchange. The server enforces different capabilities
per device type:

| Capability | whatsmeow (zaplab) | Native phone app |
|---|---|---|
| Send text, media, documents | ✅ | ✅ |
| Send static location | ✅ | ✅ |
| Initiate live location share | ⚠️ partial | ✅ |
| Update live location pin (move on map) | ❌ unreliable | ✅ |
| Access internal WhatsApp JS state | ❌ N/A | ❌ N/A |

### Why WA-JS also cannot solve this

[WA-JS](https://github.com/wppconnect-team/wa-js) takes a completely different approach: it
injects JavaScript into a real WhatsApp Web browser session (via Puppeteer, Playwright, or
TamperMonkey), hijacks webpack's module system, and drives WhatsApp Web's own internal
functions (`addAndSendMsgToChat`, `MsgStore`, `ChatPresence`, etc.) directly.

Because WA-JS drives the actual WhatsApp Web session, the server sees it as an official client.
It can even inject `isOfficialClient: true` into WhatsApp's internal state.

**However, WA-JS also cannot reliably send live location updates.** Its own codebase marks the
relevant events as deprecated:

```typescript
// WA-JS eventTypes.ts
'chat.live_location_update' // @deprecated Temporary unsuported by WhatsApp Web Multi-Device
'chat.live_location_end'    // @deprecated Temporary unsuported by WhatsApp Web Multi-Device
```

There is **no `sendLiveLocation` function in WA-JS**. The live location update pipeline was
broken when WhatsApp rolled out multi-device support and relied on internal hooks that no
longer exist in the same form. WA-JS can detect that a live location share started (via
`MsgStore.on('add')`) but cannot push position updates reliably.

### Current implementation approach (and why it may still fail)

The current zaplab implementation:

1. Frontend calls `POST /sendelivelocation` to establish the initial share and captures the
   returned `send_response.ID` (the original message ID).
2. Frontend calls `POST /simulate/route` passing `message_id`.
3. The backend goroutine sends each position update via
   `whatsmeow.SendRequestExtra{ID: originalMsgID}` — reusing the original message ID.

The intent of step 3 is to make WhatsApp treat the update as an edit to the original message
rather than a new message. This is the correct protocol-level approach, but:

- WhatsApp's server may reject the duplicate message ID from a companion device.
- WhatsApp may accept the message server-side but the recipient's client may render it as a
  new share bubble instead of updating the existing pin.
- The behavior is inconsistent across WhatsApp client versions and platforms.

### Alternatives that could work

| Approach | Notes |
|---|---|
| **WhatsApp Business Cloud API** (Meta) | Official API for registered business numbers. Supports live location. Requires Meta approval and a business account. |
| **Android/iOS emulator + automation** | Run a real WhatsApp app (Appium, UIAutomator2, XCUITest). Full primary device authority. Complex setup, fragile to WhatsApp updates. |
| **WA-JS / WPPConnect** | Puppeteer-driven WhatsApp Web session. More authority than whatsmeow, but live location updates are still broken post-multi-device migration. |
| **Rooted Android device** | Direct access to WhatsApp's internal SQLite and shared memory. Fragile and legally grey. |

---

## Package: `internal/simulation`

### `gpx.go` — GPX parser

Parses GPX XML using `encoding/xml` (no external dependencies).

**Priority order when extracting points:**
1. Track segments (`<trk>/<trkseg>/<trkpt>`)
2. Route points (`<rte>/<rtept>`)
3. Waypoints (`<wpt>`)

**Entry points:**
- `ParseGPX(data []byte) ([]TrackPoint, error)` — raw XML bytes
- `ParseGPXBase64(encoded string) ([]TrackPoint, error)` — standard base64 encoded GPX

Requires at least 2 points; returns error otherwise.

---

### `route.go` — Geometry

**Haversine formula** computes great-circle distance in km between two lat/lon points.

**`NewRoute(points []TrackPoint) *Route`**
Precomputes cumulative distance array for O(log n) lookups.

**`route.PointAt(distKm, speedKmh float64) RoutePoint`**
Binary search finds the segment containing `distKm`, then linearly interpolates lat/lon.
Also computes:
- `Bearing` — initial bearing in degrees (0–360) of the current segment
- `SpeedMps` — `speedKmh / 3.6`

---

### `manager.go` — Simulation lifecycle

**`SimRequest`**
```go
type SimRequest struct {
    To              string  // destination JID string
    GPXBase64       string  // standard base64 encoded GPX file
    SpeedKmh        float64 // default: 60
    IntervalSeconds float64 // update interval, default: 5
    Caption         string  // optional live location caption
    MessageID       string  // original message ID from POST /sendelivelocation
}
```

**`Start(toJID types.JID, req SimRequest) (*ActiveSim, error)`**
- Parses GPX, builds route, validates
- Launches goroutine with `context.WithCancel`
- Registers simulation in a `sync.Mutex`-protected map
- Returns `*ActiveSim` with metadata (id, estimated duration, distance, waypoints)

**Goroutine behavior:**
- Sends first update immediately (elapsed ≈ 0)
- Waits `IntervalSeconds` via `time.Ticker`
- Calculates `distKm = elapsed * kmPerSec` on each tick
- Saves a `SimulationLocationUpdate` event on every tick (success or failure)
- Terminates when `distKm >= route.Total` or context cancelled
- Removes itself from the active map on exit

**`Stop(id string) bool`** — cancels a running simulation, returns false if not found.

**`List() []*ActiveSim`** — snapshot of all active simulations.

---

## API Endpoints

### `POST /simulate/route`

Start a route simulation.

**Two-step flow required:**
1. Call `POST /sendelivelocation` to establish the initial live location share
2. Use the `send_response.ID` from step 1 as `message_id` in this request

**Request:**
```json
{
  "to": "5511999999999",
  "gpx_base64": "<standard base64 encoded GPX file>",
  "speed_kmh": 60,
  "interval_seconds": 5,
  "caption": "On my way!",
  "message_id": "<send_response.ID from POST /sendelivelocation>"
}
```

| Field | Required | Default | Description |
|---|---|---|---|
| `to` | yes | — | Destination JID or phone number |
| `gpx_base64` | yes | — | Base64-encoded GPX file |
| `message_id` | yes | — | Message ID from the initial `/sendelivelocation` call |
| `speed_kmh` | no | 60 | Simulated speed in km/h |
| `interval_seconds` | no | 5 | Seconds between location updates |
| `caption` | no | `""` | Text shown alongside the live location |

**Response `200`:**
```json
{
  "message": "Simulation started",
  "simulation": {
    "id": "abc12345",
    "to": "5511999999999",
    "total_distance_km": 12.5,
    "estimated_minutes": 12.5,
    "waypoints": 320,
    "speed_kmh": 60,
    "interval_seconds": 5
  }
}
```

**Error responses:**
- `400` — missing fields, invalid JID, or invalid/malformed GPX
- `400` — `message_id` not provided
- `400` — GPX has fewer than 2 points or zero-length route

---

### `DELETE /simulate/route/{id}`

Stop a running simulation.

**Response `200`:**
```json
{ "message": "Simulation stopped" }
```

**Response `404`:**
```json
{ "message": "simulation not found" }
```

---

### `GET /simulate/route`

List all currently active simulations.

**Response `200`:**
```json
{
  "simulations": [
    {
      "id": "abc12345",
      "to": "5511999999999",
      "total_distance_km": 12.5,
      "estimated_minutes": 12.5,
      "waypoints": 320,
      "speed_kmh": 60,
      "interval_seconds": 5
    }
  ]
}
```

---

## Example usage

```bash
# Step 1 — establish the live location share and capture the message ID
INIT=$(curl -s -X POST http://localhost:8090/sendelivelocation \
  -H "Content-Type: application/json" \
  -H "X-API-Token: $API_TOKEN" \
  -d '{"to":"5511999999999","latitude":-23.5505,"longitude":-46.6333,"accuracy_in_meters":10,"sequence_number":1}')

MSG_ID=$(echo $INIT | jq -r '.send_response.ID')

# Step 2 — start the route simulation
GPX_B64=$(base64 -i route.gpx)
curl -X POST http://localhost:8090/simulate/route \
  -H "Content-Type: application/json" \
  -H "X-API-Token: $API_TOKEN" \
  -d "{\"to\":\"5511999999999\",\"gpx_base64\":\"$GPX_B64\",\"speed_kmh\":40,\"interval_seconds\":3,\"message_id\":\"$MSG_ID\",\"caption\":\"On my way\"}"

# List active simulations
curl http://localhost:8090/simulate/route \
  -H "X-API-Token: $API_TOKEN"

# Stop a simulation
curl -X DELETE http://localhost:8090/simulate/route/abc12345 \
  -H "X-API-Token: $API_TOKEN"
```

---

## Notes

- Multiple simultaneous simulations are supported (different `to` targets or same target).
- Simulations are in-memory only — they do not survive a server restart.
- Position is computed from wall-clock elapsed time, not from tick count,
  so processing delays don't accumulate into position drift.
- A `SimulationLocationUpdate` event is saved on every tick regardless of send success,
  making simulation progress always observable in the Live Events stream.

---

## Known Limitations

- **Companion device restriction:** whatsmeow always connects as a linked/companion device.
  WhatsApp's server gates reliable live location updates on the device type. This is a
  protocol-level limitation that cannot be fixed within zaplab.
- **Multiple share bubbles:** Updates may appear as new messages instead of moving the pin
  on the existing share, depending on the WhatsApp client version receiving the updates.
- **No persistence:** Simulations are lost on server restart with no recovery mechanism.
- **WA-JS also broken:** Even WA-JS (which runs inside a real WhatsApp Web session with
  official client authority) marks live location update events as deprecated and unsupported
  since WhatsApp's multi-device migration. There is no `sendLiveLocation` in WA-JS.
- **Not production-ready:** This feature is experimental and intended for research only.
  The only reliable alternatives are the WhatsApp Business Cloud API (Meta) or driving
  a real Android/iOS WhatsApp app via device automation.
