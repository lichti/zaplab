# Device Spoof Spec

> **Status: Experimental**
> These settings change the identity payload sent during the WhatsApp WebSocket handshake.
> Effects on server behavior (feature gating, live location, etc.) are not guaranteed.
> Re-pair the device after changing `--device-spoof` for full effect.

---

## Overview

By default, whatsmeow connects as a **companion/linked device** — the same category as
WhatsApp Web and WhatsApp Desktop. The `--device-spoof` flag configures which identity
is presented to the WhatsApp server in the WebSocket handshake payload.

This does **not** make zaplab a true primary device. The device number in the paired JID
(`device.ID.Device`) is assigned by the WhatsApp server during pairing and is always `> 0`
for companion devices. Only the official mobile apps (Android/iOS) pair with `Device = 0`.

---

## Background: What the Server Sees

The WhatsApp handshake sends a `ClientPayload` protobuf on every connection containing:

| Field | Location | Purpose |
|---|---|---|
| `UserAgent.Platform` | `BaseClientPayload` | Broad client category (ANDROID, IOS, WEB, …) |
| `UserAgent.DeviceType` | `BaseClientPayload` | Hardware category (PHONE, TABLET, DESKTOP, …) |
| `UserAgent.Manufacturer` | `BaseClientPayload` | e.g. "Samsung", "Apple" |
| `UserAgent.Device` | `BaseClientPayload` | e.g. "Galaxy S23", "iPhone15,3" |
| `UserAgent.OsVersion` | `BaseClientPayload` | e.g. "14.0.0", "17.4.1" |
| `WebInfo.WebSubPlatform` | `BaseClientPayload` | Present only for Web clients; absent on native apps |
| `DeviceProps.PlatformType` | `DeviceProps` (pairing only) | Companion registration type (ANDROID_PHONE, IOS_PHONE, CHROME, …) |
| `DeviceProps.Os` | `DeviceProps` (pairing only) | OS name string sent at pairing time |
| `Device` (JID field) | Stored JID | 0 = primary, > 0 = companion — **set by server, cannot be spoofed** |

---

## Available Modes (`--device-spoof`)

### `companion` (default)

Standard whatsmeow companion/linked device. No impersonation.

```
UserAgent.Platform    = WEB
UserAgent.DeviceType  = DESKTOP
WebInfo               = present (WEB_BROWSER)
DeviceProps.PlatformType = ANDROID_PHONE
DeviceProps.Os        = "WhatsApp Android"
```

### `android`

Impersonates a native Android WhatsApp app.

```
UserAgent.Platform    = ANDROID
UserAgent.DeviceType  = PHONE
UserAgent.Manufacturer = "Samsung"
UserAgent.Device      = "Galaxy S23"
UserAgent.OsVersion   = "14.0.0"
WebInfo               = nil (absent — native apps don't send this)
DeviceProps.PlatformType = ANDROID_PHONE
DeviceProps.Os        = "Android"
```

### `ios`

Impersonates a native iPhone WhatsApp app.

```
UserAgent.Platform    = IOS
UserAgent.DeviceType  = PHONE
UserAgent.Manufacturer = "Apple"
UserAgent.Device      = "iPhone15,3"
UserAgent.OsVersion   = "17.4.1"
WebInfo               = nil (absent — native apps don't send this)
DeviceProps.PlatformType = IOS_PHONE
DeviceProps.Os        = "iOS"
```

---

## Usage

```bash
# Standard companion (default)
./bin/zaplab serve --http 0.0.0.0:8090

# Impersonate Android phone — re-pair after first use
./bin/zaplab serve --http 0.0.0.0:8090 --device-spoof android

# Impersonate iPhone — re-pair after first use
./bin/zaplab serve --http 0.0.0.0:8090 --device-spoof ios
```

**Re-pairing procedure:**
1. Stop zaplab
2. Delete the WhatsApp database: `rm <data-dir>/db/whatsapp.db`
3. Restart with the desired `--device-spoof` flag
4. Scan the QR code or use phone pairing — the new identity will be registered

---

## Implementation

**File:** `internal/whatsapp/bootstrap.go`

Two layers of spoofing are applied in `Bootstrap()`:

### Layer 1 — `applyDeviceSpoof(mode)` (before `whatsmeow.NewClient`)

Applied to global vars used by the store to build all payloads:
- `store.DeviceProps` — companion registration packet (only sent at pairing time)
- `store.BaseClientPayload.UserAgent` — base of every handshake payload

### Layer 2 — `client.GetClientPayload` hook (after `whatsmeow.NewClient`)

For `android` and `ios` modes, a hook is installed on the whatsmeow client:

```go
client.GetClientPayload = func() *waWa6.ClientPayload {
    payload := client.Store.GetClientPayload()
    payload.Device = proto.Uint32(0)  // override companion Device > 0 → 0 (primary)
    return payload
}
```

This hook is called at every WebSocket handshake (`handshake.go:96`) and overrides the
`ClientPayload.Device` field that would normally be set to the companion's device number
(stored JID `Device > 0`). Setting it to `0` tells the server the client is a primary device.

**Flag wiring:**
- `app.go` — `deviceSpoof *string` field on `App`
- `main.go` — `--device-spoof` persistent flag, passed to `whatsapp.Init()`
- `internal/whatsapp/deps.go` — `deviceSpoof *string` package-level var, set by `Init()`

---

## Known Limitations and Risks

- **Device number in JID is still > 0.** The `Device` field in the stored JID is assigned by
  the WhatsApp server at pairing and persists in the database. The `GetClientPayload` hook
  overrides the number in the *handshake payload* to `0`, but the server already knows the real
  device number from the initial pairing. It may accept the session, reject it, or silently
  ignore the override.

- **Possible connection failure.** Sending `Device=0` in the login payload when the server
  expects a companion device number may cause the server to reject the WebSocket connection
  or terminate the session immediately.

- **Possible ban.** WhatsApp may flag the session mismatch as suspicious behavior, leading
  to a temporary or permanent ban of the paired phone number.

- **Pairing-time identity is fixed.** `DeviceProps` (including `PlatformType`) is sent only
  during the initial pairing. Changing `--device-spoof` on an already-paired device updates
  the `UserAgent` on every reconnect but cannot change what was recorded at pairing time.

- **Signal protocol sessions.** The device number is part of the Signal protocol address
  (`SignalAddress(user, deviceID)`). Sending `Device=0` while holding keys registered under
  `Device > 0` may break encrypted session establishment with some contacts.

- **No guarantee of feature unlock.** Even if the server accepts `Device=0`, live location
  and other gated features may depend on additional server-side checks beyond the device number.

---

## Why Live Location May Still Not Work Even With Device=0

Sending `Device=0` in the payload is necessary but possibly not sufficient. The server may
also check:
- The ADV (Advanced Multi-Device) identity proof registered at pairing time
- The session's key material against the device record on the server
- Behavioral signals that distinguish native apps from third-party clients

See `SIMULATION_SPEC.md` for the full analysis of the live location limitation.
