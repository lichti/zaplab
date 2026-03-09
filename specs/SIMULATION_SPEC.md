# Route Simulation Spec

> **Status: Work in Progress — not fully functional**
> The live location update mechanism (reusing the original message ID via `SendRequestExtra`)
> is implemented but may not behave correctly across all WhatsApp clients and protocol versions.
> This feature is under active investigation. Do not rely on it in production.

## Overview

Simulates a moving device sending live WhatsApp location updates along a GPX route.
A background goroutine interpolates the device position at configurable speed and interval,
calling `SendLiveLocation` with incrementing `sequence_number` and computed bearing/speed values.

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
- Terminates when `distKm >= route.Total` or context cancelled
- Removes itself from the active map on exit

**`Stop(id string) bool`** — cancels a running simulation, returns false if not found.

**`List() []*ActiveSim`** — snapshot of all active simulations.

---

## API Endpoints

### `POST /simulate/route`

Start a route simulation.

**Request:**
```json
{
  "to": "5511999999999",
  "gpx_base64": "<standard base64 encoded GPX file>",
  "speed_kmh": 60,
  "interval_seconds": 5,
  "caption": "On my way!"
}
```

| Field | Required | Default | Description |
|---|---|---|---|
| `to` | yes | — | Destination JID or phone number |
| `gpx_base64` | yes | — | Base64-encoded GPX file |
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
# Encode GPX file
GPX_B64=$(base64 -i route.gpx)

# Start simulation
curl -X POST http://localhost:8090/simulate/route \
  -H "Content-Type: application/json" \
  -H "X-API-Token: $API_TOKEN" \
  -d "{\"to\":\"5511999999999\",\"gpx_base64\":\"$GPX_B64\",\"speed_kmh\":40,\"interval_seconds\":3,\"caption\":\"Heading to the office\"}"

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
- The recipient must have accepted the initial live location share for updates to appear.
  The first call to `POST /sendelivelocation` establishes the share;
  subsequent calls from the simulation update the position on the map.

---

## Known Limitations (Work in Progress)

- **Update behavior is not guaranteed:** WhatsApp's live location update protocol requires
  reusing the original message ID (`SendRequestExtra{ID: originalMsgID}`). While this is
  implemented, the behavior may vary depending on the WhatsApp client version and protocol state.
- **Multiple share bubbles:** Depending on the client and session state, updates may still
  appear as new messages instead of moving the pin on the existing share.
- **No persistence:** Simulations are lost on server restart with no recovery mechanism.
- **Not production-ready:** This feature is experimental and intended for research and testing only.
