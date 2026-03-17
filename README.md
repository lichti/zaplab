# zaplab — The Ultimate WhatsApp Tool

> **Versão em Português:** [README.pt-BR.md](./README.pt-BR.md)

A Go toolkit for studying and testing the WhatsApp Web protocol, with a built-in REST API for integrations (n8n, webhooks, and more). All events (received messages, receipts, presence, history sync, send errors) are persisted in PocketBase (SQLite) and dispatched to configurable webhooks. Messages can be sent via a REST API.

---

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Requirements](#requirements)
- [Local Build](#local-build)
- [Local Execution](#local-execution)
- [Docker](#docker)
- [First Run — WhatsApp Pairing](#first-run--whatsapp-pairing)
- [Updating](#updating)
- [Versioning](#versioning)
- [Environment Variables](#environment-variables)
- [Binary Flags](#binary-flags)
- [REST API](#rest-api)
- [Webhook System](#webhook-system)
- [WhatsApp Commands](#whatsapp-commands)
- [Data Model (PocketBase)](#data-model-pocketbase)
- [Frontend — ZapLab UI](#frontend--zaplab-ui)
- [Admin UI](#admin-ui)
- [Ports](#ports)

---

## Overview

**Core technologies:**

| Component | Library / Service |
|---|---|
| WhatsApp protocol | [whatsmeow](https://github.com/tulir/whatsmeow) |
| Backend / database / admin | [PocketBase](https://pocketbase.io/) v0.36 |
| HTTP router | PocketBase built-in (stdlib `net/http`) |
| Workflow automation | [n8n](https://n8n.io/) (optional, port 5678) |
| Secure exposure | Cloudflare Tunnel (optional) |

---

## Architecture

```
┌─────────────────────────────────────────────┐
│                  main package                │
│  (flags, PocketBase hooks, wiring)           │
└────────┬────────────┬────────────┬───────────┘
         │            │            │
         ▼            ▼            ▼
┌──────────────┐ ┌──────────┐ ┌────────────────┐
│internal/     │ │internal/ │ │internal/       │
│webhook       │ │whatsapp  │ │api             │
│              │ │          │ │                │
│Config        │ │Bootstrap │ │RegisterRoutes  │
│SendToDefault │ │handler   │ │POST /sendmsg   │
│SendToError   │ │HandleCmd │ │POST /cmd       │
│SendToCmd     │ │ParseJID  │ │GET  /health    │
│AddCmdWebhook │ │Send*     │ │...             │
│...           │ │persist   │ │                │
└──────────────┘ └──────────┘ └────────────────┘
         │            │
         ▼            ▼
  webhook.json   PocketBase SQLite
  (data/)        (data/db/)
```

**Lifecycle flow (PocketBase hooks):**

```
pb.Start()
  → OnBootstrap (wraps core):
      1. loads webhook.json + Init() of packages
      2. e.Next() → core bootstrap (DB, migrations, cache, settings)
      3. Bootstrap() — connects to WhatsApp
  → OnServe (wraps serve):
      1. registers /* route (static files)
      2. RegisterRoutes() — REST API
      3. e.Next() → start HTTP server
```

---

## Project Structure

```
.
├── main.go                         # Entry point
├── app.go                          # App struct (shared main state)
├── go.mod / go.sum
├── migrations/                     # PocketBase migrations (auto-applied)
├── internal/
│   ├── webhook/
│   │   └── webhook.go              # Webhook configuration and dispatch
│   ├── config/
│   │   └── config.go               # General application configuration
│   ├── whatsapp/
│   │   ├── deps.go                 # Package vars + Init()
│   │   ├── types.go                # Internal payloads
│   │   ├── bootstrap.go            # Bootstrap() — WhatsApp connection
│   │   ├── events.go               # handler() — all WA events
│   │   ├── commands.go             # HandleCmd() + cmdXxx()
│   │   ├── send.go                 # Send*() — message sending
│   │   ├── groups.go               # Group management functions
│   │   ├── spoof.go                # Spoofed/edited messages
│   │   ├── helpers.go              # ParseJID, download, getTypeOf
│   │   └── persist.go              # saveEvent, saveError, saveEventFile
│   └── api/
│       └── api.go                  # REST API (HTTP handlers)
├── pb_public/                      # ZapLab frontend (served at /tools/)
│   ├── index.html                  # HTML structure (~1 700 lines, no inline JS/CSS)
│   ├── css/
│   │   └── zaplab.css              # Custom styles (syntax highlight, scrollbar, animations)
│   └── js/
│       ├── utils.js                # Shared helpers (fmtTime, highlight, highlightCurl, …)
│       ├── zaplab.js               # Main factory — merges sections + shared state + init()
│       └── sections/
│           ├── pairing.js          # Connection — QR code display, status polling, logout
│           ├── account.js          # Account — profile picture, push name, about, platform
│           ├── events.js           # Live Events — realtime subscription + resizer
│           ├── send.js             # Send Message — all media types + reply_to
│           ├── sendraw.js          # Send Raw — raw waE2E.Message JSON editor
│           ├── ctrl.js             # Message Control — react/edit/delete/typing/disappearing
│           ├── contacts.js         # Contacts & Polls — vCard / poll create / vote
│           └── groups.js           # Group Management — list/info/create/participants/settings
├── bin/                            # Compiled binaries (git-ignored)
├── Makefile
├── Dockerfile
├── docker-compose.yml
├── entrypoint.sh
└── .env.example                    # Environment variables template
```

---

## Requirements

**Local build:**
- Go 1.25+
- No CGO required — PocketBase v0.36 uses `modernc.org/sqlite` (pure Go)

**Docker:**
- Docker 24+
- Docker Compose v2

---

## Local Build

```bash
# Format + vet + download deps + compile
make build

# Create symlink without platform suffix (optional)
make link
```

The generated binary is placed in `bin/`:
```
bin/zaplab_<GOOS>_<GOARCH>
# e.g.: bin/zaplab_linux_amd64
#       bin/zaplab_darwin_amd64
```

> The binary is self-contained — the entire `pb_public/` frontend is embedded at compile time via `//go:embed`. No extra files are needed to run it.
>
> For frontend development, set `ZAPLAB_DEV=1` to serve files from disk and avoid recompiling on every UI change.

---

## Local Execution

```bash
# Run the already-compiled binary (default port 8090)
make run

# Manual equivalent:
./bin/zaplab serve --http 0.0.0.0:8090

# With debug logs:
./bin/zaplab serve --http 0.0.0.0:8090 --debug

# Build + run in one step:
make build-run
```

Data is persisted in `$HOME/.zaplab/` by default:

```
~/.zaplab/
├── pb_data/            # PocketBase database (events, errors, collections...)
├── db/
│   └── whatsapp.db     # WhatsApp session (device credentials)
├── history/            # JSON dumps from HistorySync
├── n8n/                # n8n workflow data
└── webhook.json        # Webhook configuration
```

To change the base directory:

```bash
# Via environment variable (persistent):
export ZAPLAB_DATA_DIR=/custom/path
make run

# Via flag (one-shot):
./bin/zaplab serve --data-dir /custom/path

# Via Makefile variable:
make run DATA_DIR=/custom/path
```

Individual sub-paths can be overridden independently:

```bash
./bin/zaplab serve \
  --data-dir /base/path \
  --whatsapp-db-address "file:/other/path/whatsapp.db?_foreign_keys=on" \
  --webhook-config-file /etc/zaplab/webhook.json
```

---

## Docker

### Build the image

```bash
make build-img
```

### Start the full stack

```bash
make run-docker     # docker compose up -d
make logs           # follow logs (required to capture QR code)
make ps             # container status
make down           # stop
make clean-docker   # stop + remove volumes, images, and orphans
```

### Access container shell

```bash
make shell
```

### Services in docker-compose.yml

| Service | Image | Port | Description |
|---|---|---|---|
| `engine` | local build | 8090 | Bot + PocketBase |
| `n8n` | n8nio/n8n | 5678 | Workflow automation |
| `cloudflared` | cloudflare/cloudflared | — | Tunnel for public exposure |

---

## First Run — WhatsApp Pairing

### Step 1 — Configure environment

Copy `.env.example` to `.env` and set your tokens:

```bash
cp .env.example .env
# Edit .env and fill in:
#   API_TOKEN=your-secret-api-token
#   TUNNEL_TOKEN=your-cloudflare-tunnel-token  (if using cloudflared)
```

### Step 2 — Start the stack

```bash
make run-docker
```

### Step 3 — Pair WhatsApp

On the first run the bot has no session, so it prints a QR code in the logs:

```bash
make logs
```

The terminal will show something like:

```
█████████████████████████
█ ▄▄▄▄▄ █▀▀  ▄█ ▄▄▄▄▄ █
█ █   █ █▄▀▄▀▀█ █   █ █
...
INFO  Client connected
INFO  Client logged in
```

On your phone's WhatsApp: **Settings → Linked Devices → Link a Device** and scan the QR code.

### Step 4 — Create PocketBase superuser

Open `http://localhost:8090/_/` and follow the setup wizard to create the first admin account.

### Step 5 — Verify

```bash
curl http://localhost:8090/health
# {"pocketbase":"ok","whatsapp":true}
```

> The session is persisted in `data/db/whatsapp.db`. On subsequent runs the bot reconnects automatically — no QR code needed.

---

## Updating

### Local binary update

```bash
git pull
make build
make run
```

### Docker update

```bash
git pull
make down
make build-img
make run-docker
make logs
```

> WhatsApp session and PocketBase data are in `data/` (volume-mounted), so they survive image rebuilds.

### After a PocketBase schema migration

If a new version adds migrations, they run automatically on startup via `migratecmd.MustRegister`. No manual intervention is needed.

```bash
# You can check applied migrations in the PocketBase admin UI:
# http://localhost:8090/_/ → Settings → Migrations
```

### Unlinking / re-pairing WhatsApp

```bash
# Delete the WhatsApp session (device credentials only — NOT PocketBase data):
rm ~/.zaplab/db/whatsapp.db   # adjust path if using a custom ZAPLAB_DATA_DIR

make run-docker
make logs   # scan QR code again
```

---

## Versioning

Releases follow [Semantic Versioning](https://semver.org/): `vMAJOR.MINOR.PATCH[-prerelease]`
Examples: `v1.0.0-beta.1`, `v1.0.0-rc.1`, `v1.0.0`, `v1.1.0`

The version is embedded at build time from the nearest git tag via `-ldflags`.

```bash
# Check the version of the compiled binary
./bin/zaplab version

# Create and push a new release tag
make tag TAG=v1.0.0-beta.1
git push origin v1.0.0-beta.1
```

| Situation | Version string |
|---|---|
| No git tag yet | `dev` |
| Exactly on tag `v1.0.0` | `v1.0.0` |
| 3 commits after the tag | `v1.0.0-3-gabc1234` |
| Uncommitted changes present | `v1.0.0-dirty` |

---

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `ZAPLAB_DATA_DIR` | No | Base directory for all runtime data. Defaults to `$HOME/.zaplab`. Overridable with `--data-dir`. |
| `API_TOKEN` | Yes | Token to authenticate external REST API calls. If not set, static token auth is disabled. |
| `TUNNEL_TOKEN` | No | Cloudflare Tunnel token (only if using `cloudflared`). |
| `ZAPLAB_DEV` | No | Set to `1` to serve the dashboard UI from `./pb_public/` on disk instead of the embedded copy (enables hot-reload during frontend development). |

---

## Binary Flags

In addition to PocketBase's standard flags (`serve`, `--http`, `--dir`, etc.), the binary accepts:

| Flag | Default | Description |
|---|---|---|
| `--data-dir` | `$ZAPLAB_DATA_DIR` or `$HOME/.zaplab` | Base directory for all runtime data |
| `--debug` | `false` | Enable DEBUG-level logs |
| `--whatsapp-db-dialect` | `sqlite3` | WhatsApp database dialect (`sqlite3` or `postgres`) |
| `--whatsapp-db-address` | `<data-dir>/db/whatsapp.db` | WhatsApp database DSN |
| `--whatsapp-request-full-sync` | `false` | Request full history (10 years) on first login |
| `--whatsapp-history-path` | `<data-dir>/history` | Directory for HistorySync JSON dumps |
| `--webhook-config-file` | `<data-dir>/webhook.json` | Path to webhook configuration file |
| `--device-spoof` | `companion` | Device identity presented to WhatsApp: `companion` (default), `android`, `ios` — ⚠️ **WIP**, experimental, re-pair after changing |

> PocketBase's `--dir` flag (pb_data location) also defaults to `<data-dir>/pb_data`.

**Example:**
```bash
./bin/zaplab serve \
  --http 0.0.0.0:8090 \
  --data-dir /srv/zaplab \
  --debug
```

---

## REST API

> Full API reference: [`specs/API_SPEC.md`](./specs/API_SPEC.md)

All routes (except `/health`) require the header:

```
X-API-Token: <value of API_TOKEN>
```

### `GET /health`

Checks whether the server and WhatsApp connection are active. No authentication required.

```json
// 200 OK
{ "pocketbase": "ok", "whatsapp": true }

// 503 Service Unavailable (WhatsApp disconnected)
{ "pocketbase": "ok", "whatsapp": false }
```

### `GET /ping`

```json
{ "message": "Pong!" }
```

### `GET /wa/status`

Returns the current WhatsApp connection state and the paired phone's JID. No authentication required.

```json
{ "status": "connected", "jid": "5511999999999@s.whatsapp.net" }
```

| `status` value | Meaning |
|---|---|
| `connecting` | Client is connecting to WhatsApp servers |
| `qr` | Waiting for QR code scan — fetch `/wa/qrcode` |
| `connected` | Paired and online |
| `disconnected` | Connection lost, reconnect in progress |
| `timeout` | QR code expired, new code coming |
| `loggedout` | Session logged out, restart to re-pair |

### `GET /wa/qrcode`

Returns the current QR code as a base64-encoded PNG data URI. Only available when `status` is `qr`.

```json
{ "status": "qr", "image": "data:image/png;base64,..." }
```

Returns `404` if no QR code is currently available.

### `POST /wa/logout`

Logs out and clears the WhatsApp session. The server must be restarted to pair a new device.

```json
{ "message": "logged out" }
```

### `GET /wa/account`

Returns the connected account details fetched from the device store and WhatsApp servers.

```json
{
  "jid":           "5511999999999@s.whatsapp.net",
  "phone":         "5511999999999",
  "push_name":     "John Doe",
  "business_name": "",
  "platform":      "android",
  "status":        "Hey there! I am using WhatsApp.",
  "avatar_url":    "https://mmg.whatsapp.net/..."
}
```

Returns `503` when WhatsApp is not connected. `avatar_url` is empty if the account has no profile picture.

### `POST /sendmessage`

Sends a text message. Supports `@user` mentions via the `mentions` field.

```json
{
  "to": "5511999999999",
  "message": "Hello @Alice!",
  "mentions": ["5511888888888@s.whatsapp.net"]
}
```

The `to` field accepts:
- Number with country code: `"5511999999999"`
- Number with `+`: `"+5511999999999"`
- Full JID: `"5511999999999@s.whatsapp.net"`
- Group JID: `"123456789@g.us"`

### `POST /sendimage`

Sends an image. `image` must be **Base64**-encoded. Supports `view_once` (recipient can open once only) and `mentions`.

```json
{
  "to": "5511999999999",
  "message": "Optional caption",
  "image": "<base64>",
  "view_once": false,
  "mentions": []
}
```

### `POST /sendvideo`

Supports `view_once` and `mentions`.

```json
{
  "to": "5511999999999",
  "message": "Optional caption",
  "video": "<base64>",
  "view_once": false
}
```

### `POST /sendaudio`

```json
{
  "to": "5511999999999",
  "audio": "<base64>",
  "ptt": true
}
```

`ptt: true` sends as a voice message (push-to-talk). `ptt: false` sends as an audio file.

### `POST /senddocument`

```json
{
  "to": "5511999999999",
  "message": "Optional description",
  "document": "<base64>"
}
```

> Media size limit: **50 MB** per request.

### `POST /sendraw`

Sends any arbitrary `waE2E.Message` JSON directly — the primary interface for WhatsApp protocol exploration.
The `message` field is decoded with `protojson.Unmarshal` into `*waE2E.Message` and sent as-is.

```json
{
  "to": "5511999999999",
  "message": { "conversation": "Hello from SendRaw!" }
}
```

See [`specs/SEND_RAW_SPEC.md`](./specs/SEND_RAW_SPEC.md) for examples and full documentation.

### `POST /sendlocation`

Sends a static GPS location pin.

```json
{ "to": "5511999999999", "latitude": -23.5505, "longitude": -46.6333, "name": "São Paulo", "address": "Av. Paulista, 1000" }
```

### `POST /sendelivelocation`

Sends a live GPS location update. Repeat with incrementing `sequence_number` to update the position.

```json
{ "to": "5511999999999", "latitude": -23.5505, "longitude": -46.6333, "accuracy_in_meters": 10, "caption": "Heading to the office", "sequence_number": 1 }
```

### `POST /setdisappearing`

Sets the auto-delete timer for a chat or group. `timer`: `0` (off), `86400` (24h), `604800` (7d), `7776000` (90d).

```json
{ "to": "5511999999999", "timer": 86400 }
```

### Reply support — `reply_to` field

All send endpoints accept an optional `reply_to` block to quote a previous message:

```json
{
  "to": "5511999999999",
  "message": "That's great!",
  "reply_to": { "message_id": "ABCD1234EFGH5678", "sender_jid": "5511999999999@s.whatsapp.net", "quoted_text": "Original text" }
}
```

### `POST /sendreaction`

Adds or removes an emoji reaction to a message.

```json
{ "to": "5511999999999", "message_id": "ABCD1234EFGH5678", "sender_jid": "5511999999999@s.whatsapp.net", "emoji": "❤️" }
```

Pass `"emoji": ""` to remove an existing reaction.

### `POST /editmessage`

Edits a previously sent text message (bot's own messages only, within ~20 minutes).

```json
{ "to": "5511999999999", "message_id": "ABCD1234EFGH5678", "new_text": "Updated text" }
```

### `POST /revokemessage`

Deletes a message for everyone. Group admins can revoke other members' messages.

```json
{ "to": "5511999999999", "message_id": "ABCD1234EFGH5678", "sender_jid": "5511999999999@s.whatsapp.net" }
```

### `POST /settyping`

Sends a typing or voice-recording indicator. Call again with `"state": "paused"` to stop.

```json
{ "to": "5511999999999", "state": "composing", "media": "text" }
```

`state`: `"composing"` | `"paused"` — `media`: `"text"` (typing) | `"audio"` (recording)

### `POST /sendcontact`

Sends a single vCard contact.

```json
{
  "to": "5511999999999",
  "display_name": "John Doe",
  "vcard": "BEGIN:VCARD\nVERSION:3.0\nFN:John Doe\nTEL;TYPE=CELL:+5511999999999\nEND:VCARD"
}
```

Optionally include `"reply_to": { "message_id": "...", "sender_jid": "...", "quoted_text": "..." }`.

---

### `POST /sendcontacts`

Sends multiple vCard contacts in a single bubble.

```json
{
  "to": "5511999999999",
  "display_name": "2 contacts",
  "contacts": [
    { "name": "Alice", "vcard": "BEGIN:VCARD\nVERSION:3.0\nFN:Alice\nTEL:+5511111111111\nEND:VCARD" },
    { "name": "Bob",   "vcard": "BEGIN:VCARD\nVERSION:3.0\nFN:Bob\nTEL:+5522222222222\nEND:VCARD" }
  ]
}
```

---

### `POST /createpoll`

Creates a WhatsApp poll. The encryption key is managed internally.

```json
{
  "to": "5511999999999",
  "question": "Favourite colour?",
  "options": ["Blue", "Green", "Red"],
  "selectable_count": 1
}
```

`selectable_count`: `1` = single choice, `0` = unlimited (multiple choice).

---

### `POST /votepoll`

Casts a vote on an existing poll. `poll_message_id` and `poll_sender_jid` must match the original poll exactly.

```json
{
  "to": "5511999999999",
  "poll_message_id": "ABCD1234EFGH5678",
  "poll_sender_jid": "5511999999999@s.whatsapp.net",
  "selected_options": ["Blue"]
}
```

---

### `GET /groups`

Returns all groups the bot is currently a member of.

```json
{ "groups": [ { "JID": "123456789-000@g.us", "Name": "Group", "Participants": [...] } ] }
```

### `GET /groups/{jid}`

Returns detailed info for a group. JID must be URL-encoded (e.g. `123456789-000%40g.us`).

### `POST /groups`

Creates a new group. Name is limited to 25 characters.

```json
{ "name": "My Group", "participants": ["5511999999999", "5511888888888"] }
```

### `POST /groups/{jid}/participants`

Adds, removes, promotes, or demotes participants.

```json
{ "action": "add", "participants": ["5511999999999"] }
```

`action`: `"add"` | `"remove"` | `"promote"` | `"demote"`

### `PATCH /groups/{jid}`

Updates group settings. Include only the fields to change.

```json
{ "name": "New Name", "topic": "New description", "announce": true, "locked": false }
```

### `POST /groups/{jid}/photo`

Sets the group profile picture. `image` must be **Base64**-encoded JPEG or PNG.

```json
{ "image": "<base64>" }
```

```json
// Response 200
{ "message": "Group photo updated", "picture_id": "..." }
```

### `POST /groups/{jid}/leave`

Makes the bot leave the group.

### `GET /groups/{jid}/invitelink`

Returns the group invite link. Add `?reset=true` to revoke and generate a new one.

```json
{ "link": "https://chat.whatsapp.com/AbCdEf123456" }
```

### `POST /groups/join`

Joins a group using an invite link or code.

```json
{ "link": "https://chat.whatsapp.com/AbCdEf123456" }
```

---

### `GET /groups/{jid}/participants`

Returns only the participant list for a group (lighter than `GET /groups/{jid}`).

```json
// Response
{
  "jid": "123456789-000@g.us",
  "participants": [
    { "jid": "5511999999999@s.whatsapp.net", "phone": "5511999999999", "is_admin": true, "is_super_admin": false }
  ]
}
```

---

### `POST /media/download`

Downloads and decrypts a WhatsApp media file. Returns raw binary (not JSON). Body limit: **50 MB**.

```json
// Request
{
  "url":        "https://mmg.whatsapp.net/...",
  "media_key":  "<base64 media key>",
  "media_type": "image"
}
```

`media_type`: `image` | `video` | `audio` | `document` | `sticker`

---

### `GET /contacts`

Returns all contacts from the local WhatsApp device store.

```json
// Response
{ "contacts": [{ "JID": "...", "FullName": "John Doe", "PushName": "Johnny", ... }], "count": 1 }
```

---

### `POST /contacts/check`

Checks whether phone numbers are registered on WhatsApp.

```json
// Request
{ "phones": ["5511999999999", "5522888888888"] }

// Response
{ "results": [{ "Query": "5511999999999", "JID": "5511999999999@s.whatsapp.net", "IsIn": true }], "count": 1 }
```

---

### `GET /contacts/{jid}`

Returns stored info for a specific contact (JID must be URL-encoded).

```json
// Response
{ "JID": "5511999999999@s.whatsapp.net", "Found": true, "FullName": "John Doe", "PushName": "Johnny", ... }
```

---

### `POST /spoof/reply`

Sends a message that appears to reply to a fake quoted message from a spoofed sender.

```json
// Request
{
  "to":          "5511999999999",
  "from_jid":    "5533777777777@s.whatsapp.net",
  "msg_id":      "",
  "quoted_text": "This never happened",
  "text":        "Yes it did!"
}
```

---

### `POST /spoof/reply-private`

Same as `/spoof/reply` but sends to the recipient's private DM regardless of `to`.

---

### `POST /spoof/reply-img`

Spoofed reply with a fake image bubble attributed to `from_jid`. Body limit: **50 MB**.

```json
// Request
{
  "to":          "5511999999999",
  "from_jid":    "5533777777777@s.whatsapp.net",
  "msg_id":      "",
  "image":       "<base64>",
  "quoted_text": "Caption in the fake bubble",
  "text":        "My reply"
}
```

---

### `POST /spoof/reply-location`

Spoofed reply with a fake location bubble attributed to `from_jid`.

```json
// Request
{ "to": "5511999999999", "from_jid": "5533777777777@s.whatsapp.net", "msg_id": "", "text": "Reply" }
```

---

### `POST /spoof/timed`

Sends a self-destructing (ephemeral) text message.

```json
// Request
{ "to": "5511999999999", "text": "This will disappear" }
```

---

### `POST /spoof/demo`

Runs a pre-scripted spoofed conversation in the background. Returns immediately. Body limit: **50 MB**.

```json
// Request
{
  "to":       "5511999999999",
  "from_jid": "5533777777777@s.whatsapp.net",
  "gender":   "boy",
  "language": "br",
  "image":    "<base64 — optional>"
}

// Response (immediate)
{ "message": "Demo started (boy/br)" }
```

`gender`: `boy` | `girl` · `language`: `br` | `en`

---

### `POST /wa/qrtext`

Generates a QR Code PNG (base64) for any text string.

```json
// Request
{ "text": "https://chat.whatsapp.com/AbCdEf123456" }

// Response
{ "image": "data:image/png;base64,..." }
```

---

### `POST /cmd`

Executes a bot command via API (equivalent to typing `/cmd <cmd> <args>` in WhatsApp).

```json
// Request
{
  "cmd": "set-default-webhook",
  "args": "https://my-server.com/webhook"
}

// Response 200
{ "message": "<command output>" }
```

---

## Webhook System

Events are dispatched to configured URLs. Configuration is persisted in `webhook.json` (inside the data directory) and can be changed at runtime via the REST API or the **Webhooks** UI section — no restart needed.

### Payload structure

All webhooks receive a JSON array:

```json
[
  {
    "type": "Message.ImageMessage",
    "raw": { /* complete whatsmeow event */ },
    "extra": null
  }
]
```

### Webhook types

| Type | Description |
|---|---|
| **Default** | Receives **all** events regardless of type |
| **Error** | Receives only processing errors (send failures, download errors, etc.) |
| **Event-type** | Receives events whose type matches an exact name or wildcard pattern (e.g. `Message.*`) |
| **Text-pattern** | Receives text messages whose content matches a rule (`prefix`, `contains`, `exact`), with optional sender filter and case-sensitivity |
| **Cmd** | Legacy: receives messages whose first token matches a registered command string |

Event-type and text-pattern webhooks fire **in addition to** the default webhook — both run independently.

### Webhook REST API

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/zaplab/api/webhook` | Returns the full config (all webhook types) |
| `PUT` | `/zaplab/api/webhook/default` | Set default webhook URL `{"url":"..."}` |
| `DELETE` | `/zaplab/api/webhook/default` | Clear default webhook |
| `PUT` | `/zaplab/api/webhook/error` | Set error webhook URL `{"url":"..."}` |
| `DELETE` | `/zaplab/api/webhook/error` | Clear error webhook |
| `GET` | `/zaplab/api/webhook/events` | List event-type webhooks |
| `POST` | `/zaplab/api/webhook/events` | Add/update event-type webhook `{"event_type":"Message.*","url":"..."}` |
| `DELETE` | `/zaplab/api/webhook/events` | Remove event-type webhook `{"event_type":"..."}` |
| `GET` | `/zaplab/api/webhook/text` | List text-pattern webhooks |
| `POST` | `/zaplab/api/webhook/text` | Add text-pattern webhook (see below) |
| `DELETE` | `/zaplab/api/webhook/text` | Remove text-pattern webhook `{"id":"..."}` |
| `POST` | `/zaplab/api/webhook/test` | Send a test payload `{"url":"..."}` |

#### Event-type webhook — wildcard matching

Use `Message.*` to receive all message sub-types (`Message.ImageMessage`, `Message.AudioMessage`, etc.). Exact names like `Message.ImageMessage` are also supported.

Known event types: `Message`, `Message.ImageMessage`, `Message.AudioMessage`, `Message.VideoMessage`, `Message.DocumentMessage`, `Message.StickerMessage`, `Message.ContactMessage`, `Message.LocationMessage`, `Message.LiveLocationMessage`, `Message.PollUpdateMessage`, `Message.EncReactionMessage`, `ReceiptRead`, `ReceiptDelivered`, `Presence.Online`, `Presence.Offline`, `HistorySync`, `SentMessage`, and more.

#### Text-pattern webhook — fields

| Field | Values | Description |
|---|---|---|
| `match_type` | `prefix` \| `contains` \| `exact` | How to match the message text |
| `pattern` | any string | The text to match (e.g. `/ping`, `order`, `hello world`) |
| `from` | `all` \| `me` \| `others` | Filter by sender: self, contacts, or both |
| `case_sensitive` | `true` \| `false` | Default `false` (case-insensitive) |
| `url` | URL | Destination webhook URL |

```bash
# Add a text-pattern webhook: fire on messages starting with "/ping" from anyone
curl -X POST http://localhost:8090/zaplab/api/webhook/text \
  -H "Content-Type: application/json" \
  -d '{"match_type":"prefix","pattern":"/ping","from":"all","case_sensitive":false,"url":"https://my-server.com/ping"}'
```

### Configure via WhatsApp commands (legacy)

```
/cmd set-default-webhook https://my-server.com/webhook
/cmd set-error-webhook   https://my-server.com/errors
/cmd add-cmd-webhook     /order|https://my-server.com/orders
/cmd rm-cmd-webhook      /order
/cmd print-cmd-webhooks-config
```

### `webhook.json` file structure

```json
{
  "default_webhook": { "Scheme": "https", "Host": "my-server.com", "Path": "/webhook" },
  "error_webhook":   { "Scheme": "https", "Host": "my-server.com", "Path": "/errors" },
  "event_webhooks": [
    { "event_type": "Message.*", "webhook": { "Scheme": "https", "Host": "my-server.com", "Path": "/messages" } }
  ],
  "text_webhooks": [
    { "id": "a1b2c3d4", "match_type": "prefix", "pattern": "/ping", "from": "others", "case_sensitive": false, "webhook": { "Scheme": "https", "Host": "my-server.com", "Path": "/ping" } }
  ],
  "webhook_config": []
}
```

> The file is rewritten automatically on every change. No restart needed.

---

## WhatsApp Commands

Commands are typed **in the bot's own private chat** (chat with itself).

### Built-in commands

| Command | Description |
|---|---|
| `<getIDSecret>` | Returns the `ChatID` of the current conversation (the secret is randomly generated at boot) |
| `/setSecrete <value>` | Resets the getIDSecret secret |
| `/resetSecrete` | Generates a new random secret |
| `/cmd <command> [args]` | Executes any command listed below |

### Available commands via `/cmd`

**Webhooks:**

| Command | Arguments | Description |
|---|---|---|
| `set-default-webhook` | `<url>` | Sets the default webhook |
| `set-error-webhook` | `<url>` | Sets the error webhook |
| `add-cmd-webhook` | `<cmd>\|<url>` | Associates a command with a URL |
| `rm-cmd-webhook` | `<cmd>` | Removes a command association |
| `print-cmd-webhooks-config` | — | Displays the current configuration |

**Groups:**

| Command | Arguments | Description |
|---|---|---|
| `getgroup` | `<group_jid>` | Displays information about a group |
| `listgroups` | — | Lists all groups the bot participates in |

**Spoofed messages:**

| Command | Arguments | Description |
|---|---|---|
| `send-spoofed-reply` | `<chat_jid> <msgID:!\|#ID> <spoofed_jid> <spoofed_text>\|<text>` | Sends a reply with a fake sender |
| `sendSpoofedReplyMessageInPrivate` | `<chat_jid> <msgID:!\|#ID> <spoofed_jid> <spoofed_text>\|<text>` | Same, in private mode |
| `send-spoofed-img-reply` | `<chat_jid> <msgID:!\|#ID> <spoofed_jid> <file> <spoofed_text>\|<text>` | Spoofed reply with image |
| `send-spoofed-demo` | `<boy\|girl> <br\|en> <chat_jid> <spoofed_jid>` | Sends a demo sequence |
| `send-spoofed-demo-img` | `<boy\|girl> <br\|en> <chat_jid> <spoofed_jid> <img_file>` | Demo with image |
| `spoofed-reply-this` | `<chat_jid> <msgID:!\|#ID> <spoofed_jid> <text>` | Spoof a quoted message |

**Editing/deletion:**

| Command | Arguments | Description |
|---|---|---|
| `removeOldMsg` | `<chat_jid> <msgID>` | Deletes a previously sent message |
| `editOldMsg` | `<chat_jid> <msgID> <new_text>` | Edits a previously sent message |
| `SendTimedMsg` | `<chat_jid> <text>` | Sends a message with expiration (60s) |

> For `msgID`: use `!` to generate a random ID, or `#<ID>` to use a specific ID.

---

## Data Model (PocketBase)

Collections are created automatically via migrations on first run.

### `events`

Stores all events received from WhatsApp.

| Field | Type | Description |
|---|---|---|
| `id` | text | PocketBase ID (auto) |
| `type` | text | Event type (`Message`, `Message.ImageMessage`, `ReceiptRead`, etc.) |
| `raw` | json | Full event payload from whatsmeow |
| `extra` | json | Extra data (e.g.: decrypted poll vote) |
| `file` | file | Downloaded and stored media file for media-type events (image, audio, video, document, sticker, vCard); populated automatically when a media message is received |
| `msgID` | text | WhatsApp message ID |
| `created` | datetime | Creation timestamp (auto) |

**Indexes:** `type`, `created`, `(type, created)`

### `errors`

Stores send errors and operational failures.

| Field | Type | Description |
|---|---|---|
| `id` | text | PocketBase ID (auto) |
| `type` | text | Error type (`SentMessage`, `SendImage`, etc.) |
| `raw` | json | Payload of the message/response that failed |
| `EvtError` | text | Textual error description |
| `created` | datetime | Creation timestamp (auto) |

**Indexes:** `type`, `created`, `(type, created)`

### `history`

Stores history sync metadata (content goes to `data/history/*.json`).

| Field | Type | Description |
|---|---|---|
| `customer` | relation | Reference to `customers` |
| `phone_number` | number | Phone number |
| `msgID` | text | Message ID |
| `raw` | json | History data |

### Other collections

Created by migrations but not actively used by the bot:
- `customers` — customer registry
- `phone_numbers` — associated phone numbers

---

## Frontend — ZapLab UI

A built-in web interface for interacting with all API features without writing any code.

**Access:** `http://localhost:8090/` (automatically redirects to `/zaplab/tools/`)

**Features:**
- **URI-based Navigation** — Deep linking support (`/#/section`) and browser Back/Forward compatibility.
- **Multi-tab Support** — Sidebar links support "Open in new tab" (Ctrl/Cmd+Click).
- **Session Persistence** — Authentication persists across refreshes and tabs.

**Stack:** Alpine.js 3 · Tailwind CSS · dark/light mode · no build step required

---

### Screenshots

| | |
|---|---|
| ![Connection](docs/images/ui-connection.png) | ![Account](docs/images/ui-account.png) |
| ![Live Events](docs/images/ui-live-events.png) | ![Send Message](docs/images/ui-send-message.png) |
| ![Send Raw](docs/images/ui-send-raw.png) | ![Message Control](docs/images/ui-message-control.png) |
| ![Contacts & Polls](docs/images/ui-contacts-polls.png) | ![Groups](docs/images/ui-groups.png) |
| ![Settings](docs/images/ui-settings.png) | |

---

### Sections

| Section | Description |
|---|---|
| **Dashboard** | Overview of the running instance: connection status, account info, all-time and last-24h stats (events, received, sent, edited, deleted, errors), recent events list, and quick action buttons; auto-refreshes every 60 s |
| **Connection** | WhatsApp pairing via QR code, live connection status indicator, logout |
| **Account** | View profile picture, push name, phone number, business name, about and platform |
| **Live Events** | Real-time event stream from PocketBase — filterable by type, syntax-highlighted JSON, resizable panel |
| **Event Browser** | Search and filter stored events by type, date range, message ID, sender, recipient, or free text; inspect full JSON; preview and download media; replay message via Send Raw; **Export CSV** (up to 1 000 rows matching the active filter) |
| **Error Browser** | Browse the `errors` PocketBase collection; filter by type, date range, and free text; inspect full raw JSON; **Export CSV** |
| **Message History** | List all edited and deleted messages; filter by kind (All / Edited / Deleted), sender, chat and date range; shows the action event payload and the original message content; **Enhanced Edit Diff** — word-level or char-level tokenisation, inline or side-by-side view, diff stats bar (added/removed/similarity), **edit chain timeline** showing the full edit history of a message; **Export CSV** |
| **DB Explorer** | Browse, edit, and restore all 12 internal whatsmeow SQLite tables (device identity, Signal sessions, pre-keys, sender keys, app state, contacts, etc.); column-level protocol documentation; hex BLOB display; **inline cell editing** with automatic backup before every write; **backup & restore** (VACUUM INTO snapshots with one-click restore and full whatsmeow reinitialisation); **Reconnect / Full Reinit** buttons to observe protocol behaviour after DB modifications |
| **Protocol Timeline** | Vertical chronological timeline of all WhatsApp protocol events; color-coded event type badges; per-event protocol summary (sender, message preview, sync type, disconnect reason); **real-time updates** via PocketBase subscription; pause/resume; filter by type and free text; expandable JSON detail panel |
| **Proto Schema Browser** | Browse the **complete WhatsApp protobuf schema** compiled into the binary; package filter + full-text search; fields table (number, name, type, label, oneof group); **click-through navigation** between nested message and enum types; breadcrumb back-navigation; all 56 `go.mau.fi/whatsmeow/proto` packages exposed |
| **Frame Capture** | Real-time log stream browser — captures all whatsmeow internal log calls (Client, Socket, Send, Recv, Database modules) via a custom logger wrapper; **Live mode** (in-memory ring buffer, all levels including DEBUG frame XML); **DB mode** (persistent INFO+ with server-side filters); level badges, module labels, expandable detail |
| **Noise Handshake Inspector** | Annotated step-by-step visualisation of the `Noise_XX_25519_AESGCM_SHA256` handshake (Setup, ClientHello, ServerHello, cert verification, ClientFinish, key split) with cryptographic detail; **device public key panel** (Noise static key, Identity key, JID, registration ID); live connection event log |
| **Signal Session Visualizer** | Decodes all Double Ratchet session blobs from `whatsmeow_sessions` using `record.NewSessionFromBytes`; shows session version, sender chain counter, receiver chain count, previous counter, archived states, remote identity key; second tab decodes group SenderKey records from `whatsmeow_sender_keys` (key ID, chain iteration, signing key) |
| **Event Annotations** | Attach research notes and tags to any WhatsApp protocol event; filterable by event ID or JID; realtime updates via PocketBase subscription; **Annotate button** in Event Browser pre-fills context automatically; full CRUD API |
| **Advanced Stats & Heatmap** | GitHub-style activity heatmap (7×24 grid of day-of-week × hour); daily message sparkline SVG; event type distribution bar chart; summary cards (total, last 24 h, 7 d, 30 d, last event, edited, deleted); configurable period (7 / 30 / 90 / 365 days / all time) |
| **Send Message** | Send all message types with curl preview and response viewer |
| **Send Raw** | Send any `waE2E.Message` JSON directly — full protocol exploration |
| **Message Control** | React, edit, revoke/delete, set typing indicator, set disappearing timer |
| **Spoof Messages** | Spoofed replies (text, image, location), timed messages, demo conversation sequences |
| **Contacts & Polls** | Send vCard contacts (single or multiple), create polls, cast votes |
| **Contacts Management** | List device contacts, check phone numbers on WhatsApp, get contact info |
| **Groups** | List, get info, create, manage participants (add/remove/promote/demote), update settings, leave, get/reset invite link with QR code, join by link |
| **Media** | Download and decrypt WhatsApp media files (image, video, audio, document, sticker) |
| **Route Simulation** ⚠️ *WIP* | Simulate device movement along a GPX route sending live location updates — **experimental, not fully functional** |
| **Webhooks** | Configure default, error, event-type, and text-pattern webhooks; test webhook delivery; tabbed view with full CRUD for all webhook types |
| **Settings** | General application configuration: toggle Message Recovery for edits and deletes; manage API token |
| **User Profile** | Update dashboard display name and email; manually trigger password changes |

---

### Send Message — supported types

| Type | Description |
|---|---|
| Text | Plain text, with optional reply-to quoting |
| Image | Base64 PNG/JPEG with optional caption and reply-to |
| Video | Base64 MP4 with optional caption and reply-to |
| Audio | Base64 audio, PTT (voice note) or file mode |
| Document | Base64 any format with optional caption |
| Location | Static GPS pin with name and address |
| Live Location | Live GPS updates with accuracy and caption |
| Contact | Single vCard |
| Contacts | Multiple vCards in one bubble |
| Reaction | Add or remove emoji reaction on any message |

All send forms include a **curl preview** tab (syntax-highlighted, one-click copy) and a **response** tab with formatted JSON output.

---

## First Run & Authentication

ZapLab uses a dual-layer authentication system:

1.  **Dashboard (Web UI)**: Uses PocketBase Users.
2.  **REST API**: Uses a static `X-API-Token` header.

### Creating the First User

ZapLab automatically creates a default user on the first run if the database is empty:
- **Email**: `zaplab@zaplab.local`
- **Password**: Randomly generated and printed to the terminal on startup.

You can also manually create a user or reset a password using the CLI:

```bash
# Create a new user
./bin/zaplab user create admin@example.com my-password

# Reset a password
./bin/zaplab user reset-password zaplab@zaplab.local new-password
```

Alternatively, you can use the **PocketBase Admin UI** at `http://localhost:8090/_/`:
1.  If it's the first run, follow the prompts to create your **Admin account**.
2.  Once inside the Admin Panel, navigate to the **`users`** collection in the sidebar.
3.  Click **"New Record"** and create a user with an email and password.
4.  Use these credentials to sign in at the ZapLab Dashboard.

---

## Admin UI

After starting the server, the PocketBase admin interface is available at:

```
http://127.0.0.1:8090/_/
```

Allows you to view and filter events/errors, manage collections, configure access rules, and perform backups.

---

## Ports

| Service | Default Port |
|---|---|
| Bot / PocketBase API + Admin | 8090 |
| n8n (automation) | 5678 |
