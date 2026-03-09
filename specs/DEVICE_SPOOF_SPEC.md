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

**File:** `internal/whatsapp/bootstrap.go` — `applyDeviceSpoof(mode string)`

Called at the start of `Bootstrap()`, before `sqlstore.New()` and `whatsmeow.NewClient()`.
All changes are applied to the `store.DeviceProps` and `store.BaseClientPayload` package-level
globals, which whatsmeow uses to build the registration and login payloads.

**Flag wiring:**
- `app.go` — `deviceSpoof *string` field on `App`
- `main.go` — `--device-spoof` persistent flag, passed to `whatsapp.Init()`
- `internal/whatsapp/deps.go` — `deviceSpoof *string` package-level var, set by `Init()`

---

## Known Limitations

- **Device ID cannot be spoofed.** The `Device` number in the JID is assigned by WhatsApp's
  server during pairing. A companion device always gets `Device > 0`, regardless of the
  `UserAgent.Platform` or `DeviceProps.PlatformType` sent during the handshake.

- **Pairing-time vs login-time identity.** `DeviceProps` (including `PlatformType`) is only
  sent during the initial pairing registration. `UserAgent` fields are sent on every connection.
  Changing `--device-spoof` on an already-paired device only updates the `UserAgent` identity
  on the next login — `DeviceProps` from the original pairing remain on the server.

- **No guarantee of feature unlock.** WhatsApp's server may gate live location and other
  features on the `Device` number (primary = 0) rather than the `UserAgent.Platform`.
  Spoofing the platform may not unlock restricted capabilities.

- **Detection risk.** WhatsApp may cross-check the declared platform against behavioral
  signals (e.g. message patterns, timing, feature usage). Mismatches could trigger session
  termination or bans.

---

## Why Live Location Still May Not Work

Even with `--device-spoof android` or `--device-spoof ios`, the device's JID `Device` number
remains `> 0` (companion), which is the field WhatsApp's server most likely uses to determine
live location update authority. See `SIMULATION_SPEC.md` for a full analysis.
