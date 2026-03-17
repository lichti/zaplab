# Frame Capture & Noise Handshake Inspector — Specification

## Overview

Two low-level protocol research tools that expose the internal log stream of the whatsmeow library and provide an annotated view of the WhatsApp Noise handshake.

---

## 1. Frame Capture

### Architecture

#### Custom Logger (`internal/whatsapp/capturelog.go`)

`CapturingLogger` implements the `waLog.Logger` interface and wraps any existing logger (e.g. `waLog.Stdout`). It is assigned to the whatsmeow client and database logger in `Bootstrap`:

```go
clientLog := NewCapturingLogger(waLog.Stdout("Client", logLevel, true), "Client")
client = whatsmeow.NewClient(device, clientLog)
```

Because whatsmeow derives its internal sub-loggers (Recv, Send, Socket) via `logger.Sub(module)`, and `CapturingLogger.Sub()` returns a new capturing logger with a composed module name, **all** log calls from whatsmeow are intercepted:

| whatsmeow sub-logger | Captured module |
|---|---|
| `client.Log` | `Client` |
| `client.recvLog` | `Client/Recv` |
| `client.sendLog` | `Client/Send` |
| `client.Log.Sub("Socket")` | `Client/Socket` |
| database logger | `Database` |

#### Ring Buffer

Thread-safe circular buffer with capacity 2 000 entries (`ringBufSize`). Stores all log levels including DEBUG. Implemented as a fixed-size array with a head pointer and mutex.

- `push(LogEntry)` — O(1), never blocks
- `snapshot(module, level, limit)` — returns entries in chronological order with optional filters

#### PocketBase Persistence

For **INFO, WARN, ERROR** entries only, a record is written to the `frames` collection via a buffered channel (`captureSink`, capacity 4 096). A background goroutine drains the channel:

- Non-blocking enqueue: if the channel is full, the entry is dropped (never blocks the WhatsApp connection)
- `StartLogConsumer()` must be called after `pb` is set (called inside `Bootstrap`)

#### Entry Format

```go
type LogEntry struct {
    Seq     uint64    `json:"seq"`     // monotonic counter
    Time    time.Time `json:"time"`    // capture timestamp
    Module  string    `json:"module"`  // e.g. "Client/Socket"
    Level   string    `json:"level"`   // DEBUG | INFO | WARN | ERROR
    Message string    `json:"message"` // formatted log message
}
```

### PocketBase Collection: `frames`

| Field | Type | Notes |
|---|---|---|
| `module` | text | Logger module path |
| `level` | text | DEBUG / INFO / WARN / ERROR |
| `seq` | text | Monotonic sequence number (string to avoid int overflow) |
| `msg` | text | Log message |
| `created` | autodate | |
| `updated` | autodate | |

Indexes: `module`, `level`, `created`, `(level, created)`.

### API Endpoints

All require auth.

| Method | Path | Description |
|---|---|---|
| `GET` | `/zaplab/api/frames` | Persistent DB (INFO+): paginated with `module`, `level`, `search`, `page`, `per_page` filters |
| `GET` | `/zaplab/api/frames/ring` | In-memory ring buffer: all levels, `module`, `level`, `limit` filters |
| `GET` | `/zaplab/api/frames/modules` | Distinct module names currently in the ring buffer |

#### `GET /zaplab/api/frames` response

```json
{
  "items": [
    { "id": "...", "module": "Client", "level": "INFO", "seq": "42", "msg": "Connected to server", "created": "..." }
  ],
  "total": 150,
  "page": 1,
  "per_page": 100
}
```

#### `GET /zaplab/api/frames/ring` response

```json
{
  "entries": [
    { "seq": 2048, "time": "...", "module": "Client/Recv", "level": "DEBUG", "message": "<iq ...>...</iq>" }
  ],
  "total": 500
}
```

### Frontend (`pb_public/js/sections/framecapture.js`)

#### Modes

| Mode | Source | Levels | Realtime |
|---|---|---|---|
| Live (ring) | `/frames/ring` | DEBUG + | No — manual refresh + pause |
| DB (INFO+) | `/frames` via PB | INFO + | Yes — PocketBase subscription |

#### State

| Property | Description |
|---|---|
| `fcEntries` | Displayed entries |
| `fcLiveMode` | `true` = ring buffer, `false` = DB |
| `fcModuleFilter` | Selected module (empty = all) |
| `fcLevelFilter` | Selected level (empty = all) |
| `fcSearch` | Text search (DB mode only) |
| `fcPaused` | Freeze live appending |
| `fcConnStatus` | Realtime subscription status |
| `fcModules` | Distinct modules from ring buffer |

---

## 2. Noise Handshake Inspector

### Purpose

Documents and visualises the WhatsApp `Noise_XX_25519_AESGCM_SHA256` handshake as implemented in `go.mau.fi/whatsmeow/handshake.go` and `socket/noisehandshake.go`.

### Protocol Summary

WhatsApp uses the **Noise XX** interactive handshake pattern. Both parties authenticate their static keys to each other.

```
Initiator (companion)         Responder (WhatsApp server)
─────────────────────────────────────────────────────────
  Generate ephemeral key pair (e)
  Authenticate(e.pub) → update hash
  → send ClientHello { e.pub }

                                ← ServerHello { se.pub, enc(ss), enc(cert) }
  Authenticate(se)
  MixKey(DH(e.priv, se))
  ss_plain ← Decrypt(enc_ss)     // server static key
  MixKey(DH(e.priv, ss_plain))
  cert_plain ← Decrypt(enc_cert)
  Verify certificate chain

  enc_noise_pub ← Encrypt(noiseKey.pub)
  MixKey(DH(noiseKey.priv, se))
  enc_payload ← Encrypt(ClientPayload)
  → send ClientFinish { enc_noise_pub, enc_payload }

  Split(salt) → (writeKey, readKey)
─────────────────────────────────────────────────────────
              Session established — all frames now AES-256-GCM
```

#### Certificate chain

The server's static key is bound to a WhatsApp root certificate using EdDSA (Ed25519):

- `WACertPubKey` — hardcoded 32-byte root public key in whatsmeow
- `waCert.CertChain` contains an intermediate certificate and a leaf certificate
- The leaf certificate's `key` field must exactly match the decrypted server static key
- This prevents MITM attacks that substitute a different server static key

#### Key types

| Key | Algorithm | Usage |
|---|---|---|
| Ephemeral key (e) | Curve25519 | Fresh per connection; provides forward secrecy |
| Server ephemeral (se) | Curve25519 | Server's per-connection key |
| Server static (ss) | Curve25519 | Server's long-term identity; certified by WhatsApp root |
| Noise static (noiseKey) | Curve25519 | Companion device's long-term identity |
| Identity key | Ed25519 | Used for Signal Protocol (separate from Noise) |

### API Endpoint

#### `GET /zaplab/api/wa/keys`

Returns the device's **public** key material only. Private keys are never exposed.

```json
{
  "jid": "15551234567.0:1@s.whatsapp.net",
  "noise_pub": "a1b2c3d4...",        // hex-encoded 32-byte Curve25519 public key
  "identity_pub": "e5f6a7b8...",     // hex-encoded 32-byte Ed25519 public key
  "registration_id": 12345,
  "platform": "WhatsApp Android",
  "business_name": "",
  "push_name": "My Device"
}
```

Returns `503` if the client is not yet bootstrapped.

### Frontend (`pb_public/js/sections/noisehandshake.js`)

Two-panel layout:

**Left panel**: Static step-by-step handshake timeline with:
- Phase name and actor badge (client / server / both)
- Cryptographic detail (key operations, formulas)
- Proto message types involved
- Protocol notes (Noise XX semantics, certificate pinning, post-handshake encryption)

**Right panel**:
- Device public keys (JID, Noise static key with copy button, Identity key with copy button, device info)
- Live connection event log: filters the in-memory ring buffer for `Client` module entries matching connection/handshake keywords

---

## Security

- `/zaplab/api/wa/keys` returns **public keys only** — private keys never leave the process
- The `CapturingLogger` captures log messages but does NOT log private keys (whatsmeow never logs them)
- Ring buffer is in-process memory only; no file system persistence
- All endpoints require authentication
