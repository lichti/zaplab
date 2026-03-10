# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- **Event Browser** frontend section — search and filter stored events from PocketBase by type, date range, message ID, sender, recipient/chat, and free text; click any event to inspect the full JSON (syntax-highlighted); media preview (image, video, audio, file download) when a `file` is attached; **Replay** panel to re-send the event's `Message` payload to any JID via `/zaplab/api/sendraw`
- **Message History** frontend section — lists all edited and deleted messages captured in the events store; filter by kind (All / Edited only / Deleted only), sender, chat and date range; clicking an entry shows the action event payload (kind badge, new content for edits, target ID for deletes, full syntax-highlighted JSON) and automatically looks up and displays the **original message** by `msgID` (content preview, media, full JSON); original message ID extracted from `Message.protocolMessage.key.ID` per whatsmeow's serialization
- All API routes now prefixed: `/zaplab/api/<route>` for API endpoints, `/zaplab/tools/{path...}` for static files
- Frontend JS updated to match new route prefixes

---

## [v1.0.0-beta.3] — 2026-03-10

### Added

#### Spoof Messages API & UI
- `POST /spoof/reply` — send a text message that appears to reply to a fake quoted message from a spoofed sender
- `POST /spoof/reply-private` — same as above, but delivered to the recipient's private DM
- `POST /spoof/reply-img` — spoofed reply with a fake image bubble attributed to a spoofed JID (body limit: 50 MB)
- `POST /spoof/reply-location` — spoofed reply with a fake location bubble attributed to a spoofed JID
- `POST /spoof/timed` — send a self-destructing (ephemeral) text message
- `POST /spoof/demo` — run a pre-scripted spoofed conversation sequence in the background (`boy`/`girl` × `br`/`en`)
- Exported Go wrappers in `internal/whatsapp/spoof.go`: `SpoofReply`, `SpoofReplyPrivate`, `SpoofReplyImg`, `SpoofReplyLocation`, `SendTimedMessage`, `SpoofDemo`
- **Spoof Messages** frontend section (`pb_public/js/sections/spoof.js`) with per-type conditional fields, image file picker, gender/language selectors, curl preview, and response viewer

#### Contact Management API & UI
- `GET /contacts` — list all contacts from the local WhatsApp device store
- `POST /contacts/check` — check whether phone numbers are registered on WhatsApp (`IsOnWhatsApp`)
- `GET /contacts/{jid}` — fetch stored info for a specific contact
- **Contacts Management** frontend section (`pb_public/js/sections/contactsmgmt.js`) with contact list table (filterable, CSV export), phone check results, info card, and contact picker

#### Media Download API
- `POST /media/download` — download and decrypt a WhatsApp media file from the CDN (image, video, audio, document, sticker); returns raw binary with detected mime type (body limit: 50 MB)

#### Device Spoof (experimental)
- `--device-spoof` flag (`companion` / `android` / `ios`) — configures the identity payload
  sent to WhatsApp during the WebSocket handshake to impersonate different device types.
  For `android` and `ios` modes, also overrides `ClientPayload.Device=0` via the
  `client.GetClientPayload` hook to attempt primary device impersonation.
  Experimental; re-pair after changing. See `specs/DEVICE_SPOOF_SPEC.md`.

### Changed
- **Contacts & Polls** frontend section is now send-only (`contact`, `contacts`, `poll`, `votepoll`) — management actions moved to dedicated **Contacts Management** section

### Documentation
- Created `specs/SPOOF_SPEC.md` — protocol details, endpoint reference, Go implementation, frontend mapping
- Created `specs/CONTACTS_MGMT_SPEC.md` — contact management endpoint reference and frontend spec
- Updated `specs/API_SPEC.md` — added all new endpoints and updated body limits table
- Updated `specs/FRONTEND_ARCHITECTURE_SPEC.md` — reflects current section file list and `zaplab()` factory
- Updated `specs/CONTACTS_POLLS_SPEC.md` — noted management split
- Updated `README.md` and `README.pt-BR.md` — all new endpoints and UI sections documented

---

## [v1.0.0-beta.2] — 2026-03-09

### Added
- `SimulationLocationUpdate` event saved on every simulation tick (regardless of WhatsApp send success), making route simulation progress always visible in Live Events
- `whatsapp.SaveEvent()` exported so sub-packages can persist events independently
- Explicit `LiveLocationMessage` handler in the event dispatcher — incoming live location updates are now saved as `"Message.LiveLocationMessage"` in the events collection
- Test GPX file `tests/central-park-walk.gpx` — ~5 km walk through Central Park at 10 km/h (~30 min) for simulation testing

### Fixed
- **Route Simulation:** each simulation tick was creating a new WhatsApp live location share instead of updating the existing one — fixed by reusing the original message ID via `whatsmeow.SendRequestExtra{ID: originalMsgID}`
- **Route Simulation:** errors from `SendLiveLocation` were silently ignored — the goroutine now captures and records them in the event payload
- **Events:** incoming `LiveLocationMessage` was not reliably stored due to a fallthrough in the media-handler loop (Go interface-nil gotcha); now handled explicitly
- **Events:** `LocationMessage` events renamed from generic `"Message"` to `"Message.LocationMessage"` for clarity

### Changed
- Route Simulation marked as **Work in Progress** (⚠ WIP) across frontend, documentation, and source code — the feature is experimental and not yet fully functional

---

## [v1.0.0-beta.1] — 2026-03-07

### Added

#### Core
- **Initial public release** of zaplab — Go toolkit for studying and testing the WhatsApp Web protocol
- Embedded [PocketBase](https://pocketbase.io/) v0.36+ backend (SQLite, no CGO) with custom WhatsApp integration
- [whatsmeow](https://github.com/tulir/whatsmeow) integration for the WhatsApp Web protocol
- `zaplab version` subcommand with version string embedded at build time via `-ldflags "-X main.Version=..."`
- Git-tag-based versioning: `make tag TAG=vX.Y.Z` / `make tag-push`
- MIT License

#### API (`internal/api`)
- Authentication via `X-API-Token` header (all routes except `/health`)
- `GET /health` — health check endpoint
- `POST /pair/qr` — initiate QR code pairing
- `POST /pair/phone` — initiate phone number pairing
- `GET /account` — retrieve account info (profile picture, push name, phone, business name, about, platform)
- `POST /sendtext` — send plain text message (with optional reply-to)
- `POST /sendimage` — send image (base64 PNG/JPEG, optional caption and reply-to)
- `POST /sendvideo` — send video (base64 MP4, optional caption and reply-to)
- `POST /sendaudio` — send audio (base64, PTT or file mode)
- `POST /senddocument` — send document (base64, any format)
- `POST /sendlocation` — send static GPS pin (name, address)
- `POST /sendelivelocation` — send live GPS location update
- `POST /sendcontact` — send single vCard contact
- `POST /sendcontacts` — send multiple vCard contacts in one bubble
- `POST /sendpoll` — create a poll
- `POST /votepoll` — cast a vote on a poll
- `POST /sendreaction` — add or remove emoji reaction
- `POST /revokemessage` — revoke (delete for everyone) a message
- `POST /deletemessage` — delete a message for self
- `POST /editmessage` — edit a sent message
- `POST /sendpresence` — send typing / recording indicator
- `POST /setdisappearing` — set disappearing messages timer
- `GET /groups` — list all groups
- `GET /groups/{jid}` — get group info
- `POST /groups` — create a new group
- `POST /groups/{jid}/participants` — add participants
- `DELETE /groups/{jid}/participants` — remove participants
- `POST /groups/{jid}/participants/promote` — promote to admin
- `POST /groups/{jid}/participants/demote` — demote from admin
- `PATCH /groups/{jid}` — update group name / description / settings
- `POST /groups/{jid}/leave` — leave a group
- `GET /groups/{jid}/invitelink` — get invite link
- `DELETE /groups/{jid}/invitelink` — reset invite link
- `POST /groups/join` — join a group by invite link
- `GET /contacts/{jid}` — get contact info
- `POST /simulate/route` — start GPX route simulation *(experimental — WIP)*
- `DELETE /simulate/route/{id}` — stop a running simulation *(experimental — WIP)*
- `GET /simulate/route` — list active simulations *(experimental — WIP)*

#### Frontend — ZapLab UI (`pb_public/`, served at `/tools/`)
- Alpine.js 3 + Tailwind CSS (CDN) — no build step
- Dark / light theme toggle, persisted in localStorage
- Collapsible sidebar with section navigation
- **Connection** section — QR code pairing, phone pairing, live connection status, logout
- **Account** section — profile picture, push name, phone number, business name, about, platform
- **Live Events** section — real-time PocketBase event stream, filterable by type, syntax-highlighted JSON, resizable panel
- **Send Message** section — all message types with curl preview and response viewer
- **Send Raw** section — send arbitrary `waE2E.Message` JSON for protocol exploration
- **Message Control** section — react, edit, revoke/delete, typing indicator, disappearing timer
- **Contacts & Polls** section — vCard contacts (single/multiple), polls, voting
- **Groups** section — full group management (list, info, create, participants, settings, invite link with QR)
- **Route Simulation** section *(experimental — WIP)* — GPX upload, speed/interval controls, two-step start, polling, stop
- **Settings** section — API token configuration (localStorage)
- Curl preview tabs for all API operations (syntax-highlighted, one-click copy)

#### Infrastructure
- Multi-stage Dockerfile: `golang:1.25-bookworm` builder → `debian:bookworm-slim` runtime
- Docker Compose stack: zaplab engine + n8n 2.10.4 + Cloudflare Tunnel
- Health check on `/health`, n8n depends on engine being healthy
- `entrypoint.sh` simplified with `exec "$@"` pattern
- Makefile targets: `build`, `link`, `run`, `build-img`, `run-docker`, `shell`, `tag`, `tag-push`, `git-init`, `clean`, `clean-docker`, and more
- Data directory configurable via `--data-dir` flag or `ZAPLAB_DATA_DIR` env var (default: `$HOME/.zaplab`)
- Webhook dispatcher (`internal/webhook`) — default webhook + error webhook + per-command routing
- PocketBase migrations for `events`, `errors`, and `sent_messages` collections
- Automatic reconnect with exponential backoff on disconnect (5 s → 10 s → … → 5 min)

#### Simulation (`internal/simulation`) *(experimental — WIP)*
- `ParseGPX` / `ParseGPXBase64` — GPX XML parser with no external dependencies
- `NewRoute` / `PointAt` — haversine distance, binary search interpolation, bearing and speed computation
- Background goroutine lifecycle management with `context.WithCancel`

#### Documentation & Specs
- `README.md` (English) — full API reference, setup guide, frontend documentation, screenshots
- `README.pt-BR.md` (Portuguese) — full translation
- `specs/` directory with detailed specs for all features:
  `API_SPEC.md`, `MESSAGE_CONTROL_SPEC.md`, `LOCATION_REPLY_SPEC.md`, `CONTACTS_POLLS_SPEC.md`,
  `GROUPS_SPEC.md`, `GROUPS_UI_SPEC.md`, `FRONTEND_ARCHITECTURE_SPEC.md`, `FRONTEND_SPEC.md`,
  `SEND_PREVIEW_SPEC.md`, `SEND_RAW_SPEC.md`, `SIMULATION_SPEC.md`, `WHATSMEOW_ANALYSIS.md`
- Test files in `tests/`: payload examples, `central-park-walk.gpx` (5 km GPX for simulation testing)
- `.env.example` with all configurable environment variables

---

[Unreleased]: https://github.com/lichti/zaplab/compare/v1.0.0-beta.3...HEAD
[v1.0.0-beta.3]: https://github.com/lichti/zaplab/compare/v1.0.0-beta.2...v1.0.0-beta.3
[v1.0.0-beta.2]: https://github.com/lichti/zaplab/compare/v1.0.0-beta.1...v1.0.0-beta.2
[v1.0.0-beta.1]: https://github.com/lichti/zaplab/releases/tag/v1.0.0-beta.1
