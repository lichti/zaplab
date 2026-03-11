# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Dev]

---

## [Unstable]

### Added
- **Message Recovery** — new feature to detect and notify about deleted (revoked) or edited messages; original content is retrieved from the local database and sent to the user's own JID.
- **General Settings UI** — new Settings section in the dashboard to manage general application configurations (like Message Recovery flags).

### Changed
- **Static Landing Page** — moved from `site/` to `docs/` for native GitHub Pages compatibility; enriched with detailed project information, added a "View Changelog" button, removed "Get Started Free" button, and updated version to `v1.0.0-beta.6`.

---

## [v1.0.0-beta.6] — 2026-03-10

### Added
- **Static Landing Page** (`site/`) — modern, responsive project website built with Tailwind CSS; highlights features, tech stack, and includes a deployment guide for GitHub Pages
- **GitHub Actions CI/CD** (`.github/workflows/release.yml`) — automated release pipeline triggered on version tags (`v*`); builds multi-platform Go binaries (Linux, macOS, Windows), creates GitHub Releases, and pushes multi-arch Docker images to Docker Hub and GHCR

### Fixed
- **Reaction / non-media message dispatch** — reaction messages (and any non-media message type) were incorrectly entering the `ImageMessage` download path and logging `"Failed to download"`; root cause was the Go interface-nil gotcha: `GetImageMessage()` returns a typed nil `(*T)(nil)` which, when stored as `interface{}` in the `mediaHandlers` slice, produces a non-nil interface value that bypasses the `m.msg == nil` guard; fixed by replacing the generic slice with explicit `if/else-if` typed nil checks, matching the pattern already applied to `LiveLocationMessage`

---

## [v1.0.0-beta.5] — 2026-03-10

### Added
- **Dynamic Webhooks UI** — new tabbed interface in the dashboard to manage URLs for default, error, event-type, and text-pattern webhooks; supports full CRUD (Create, Read, Update, Delete) and delivery testing directly from the UI
- **Dashboard Auto-refresh** — instance overview stats (Total events, Received, Sent, Edited, Deleted, Errors) and recent events list now auto-refresh every 60 seconds with a visible countdown timer
- **Event Browser Export** — download up to 1,000 events matching your current filters as a CSV file
- **Error Browser** — browse, filter, and export the `errors` collection from PocketBase
- **Message History** — specialized view for looking up edited and deleted messages, including the original content and visual diffs for edits

### Changed
- **Whatsmeow update** — bumped to the latest commit to ensure compatibility with recent WhatsApp protocol changes
- **PocketBase v0.36** — upgraded core engine for improved performance and security; automigrations are enabled by default
- **API Versioning** — all REST endpoints now prefixed with `/zaplab/api/` for better namespace management

### Fixed
- **Media Persistence** — resolved an issue where incoming media (images, videos, stickers) failed to save correctly to the PocketBase filesystem due to internal record state conflicts
- **Parallel Queries** — fixed Dashboard query cancellation by disabling `requestKey` auto-cancellation in the PocketBase JS SDK for parallel `Promise.all` calls

---

## [v1.0.0-beta.4] — 2026-03-10

### Added
- **Dashboard** frontend section — overview of the running instance: connection status card (live dot indicator + JID), account card (avatar, push name, phone, platform), two stat grids (All time / Last 24h) with Total Events, Received, Sent, Edited, Deleted and Errors counters, Recent Events list (last 10 with type badge and preview), Quick Actions buttons for all sections; auto-refreshes every 60 s with visible countdown; all 13 PocketBase queries run in parallel via `Promise.allSettled` with `requestKey: null` to avoid SDK auto-cancellation
- **Event Browser** frontend section — search and filter stored events from PocketBase by type, date range, message ID, sender, recipient/chat, and free text; click any event to inspect the full JSON (syntax-highlighted); media preview (image, video, audio, file download) when a `file` is attached; **Replay** panel to re-send the event's `Message` payload to any JID via `/zaplab/api/sendraw`
- **Message History** frontend section — lists all edited and deleted messages captured in the events store; filter by kind (All / Edited only / Deleted only), sender, chat and date range; clicking an entry shows the action event payload (kind badge, new content for edits, target ID for deletes, full syntax-highlighted JSON) and automatically looks up and displays the **original message** by `msgID` (content preview, media, full JSON); original message ID extracted from `Message.protocolMessage.key.ID` per whatsmeow's serialization
- **Edit Diff** panel in Message History — word-level visual diff (LCS algorithm) between the original and edited message text; deleted words shown in red with strikethrough, inserted words in green; whitespace-aware tokenization; block-diff fallback for very long texts (>400 tokens)
- **Export CSV** button in Event Browser — exports all events matching the current filter (up to 1 000 rows) as a downloadable CSV file; fetches all pages server-side before generating the file; columns: `id`, `type`, `msgID`, `created`, `sender`, `chat`, `preview`, `file`
- **Error Browser** frontend section — browse the PocketBase `errors` collection; filter by type, date range, and free text; click to inspect full JSON; Export CSV (up to 1 000 rows); nav button in sidebar
- **CSV export in Message History** — same pattern as Event Browser; columns: `id`, `kind`, `msgID`, `created`, `sender`, `chat`, `targetID`, `newContent`
- **Dashboard recent events clickable** — clicking any row navigates to Event Browser with that event pre-selected in the detail panel (`dashGoToEvent()`)
- **`POST /groups/{jid}/photo`** — set the group profile picture (base64 JPEG or PNG); returns `picture_id`; **Set Photo** operation added to Groups UI section with image file picker
- **Mentions (`@user`)** — `mentions: [string]` field added to `POST /sendmessage`, `POST /sendimage`, `POST /sendvideo`; backend extends `ReplyInfo.MentionedJIDs` and `ContextInfo.MentionedJID`; UI: collapsible textarea (one JID per line) for text, image and video types
- **View-once media** — `view_once: bool` field added to `POST /sendimage` and `POST /sendvideo`; wraps the inner message in `ViewOnceMessage/FutureProofMessage` when true; UI: checkbox shown for image and video types
- All API routes now prefixed: `/zaplab/api/<route>` for API endpoints, `/zaplab/tools/{path...}` for static files
- Frontend JS updated to match new route prefixes

### Fixed
- `highlight()` utility: guard against `null`/`undefined` input that caused Alpine expression crash (`Cannot destructure property '_isNew' of null`)
- `loadInitialEvents`: add `requestKey: null` to prevent PocketBase SDK auto-cancellation when Dashboard queries run in parallel during `init()`
- `contactsmgmt` HTML: `mgmt.result.count` expression guarded with ternary — Alpine evaluates `x-text` even when `x-show` hides the element
- **Media event persistence** — incoming messages with media attachments (image, audio, video, document, sticker, vCard) were logging `"Failed to save event"` and falling back to a file-less record; root cause was `saveEventFile` manually uploading via `pb.NewFilesystem()` then setting only the filename string on the record, which PocketBase v0.36 rejects for new records; fixed by passing the `*filesystem.File` object directly to `record.Set("file", file)` and letting `pb.Save` handle the upload atomically

---

## [v1.0.0-beta.3] — 2026-03-10

### Added
- **Webhooks UI** frontend section — configure dynamic URLs for default, error, event-type, and text-pattern webhooks directly from the browser; supports full CRUD for all webhook types
- **Event-Type Filtering** — route incoming WhatsApp events to specific endpoints based on their internal type (e.g. `Message.ImageMessage` or wildcards like `Message.*`)
- **Text-Pattern Filtering** — route messages based on content (Prefix, Contains, Exact) with sender/chat filtering
- **Webhook Delivery Test** — button to send a test payload to any URL to verify integration
- **Sidebar Persistence** — expanded/collapsed state and active section are now remembered across page refreshes via `localStorage`

### Changed
- Webhook configuration now stored in a separate `webhook.json` file in the data directory
- Internal event dispatcher updated to support multiple parallel webhook calls per event

---

## [v1.0.0-beta.2] — 2026-03-10

### Added
- **Message Control UI** — new frontend section to interact with existing messages: send Reactions (emoji), Edit messages (text-only), and Revoke (delete for everyone) messages
- **Presence Control** — toggle between "Available" (online) and "Unavailable" (offline); set typing/recording indicators for specific chats from the UI
- **Disappearing Messages** — set the ephemeral timer (Off, 24h, 7d, 90d) for any chat or group
- **vCard Contacts** — send single or multiple contacts directly from the UI
- **Polls** — create interactive polls with custom options and cast votes on existing polls via the Contacts & Polls section
- **Status Polling** — live connection indicator (dot) and account summary card in the sidebar

### Fixed
- Fixed a bug where reaction events were missing the target message ID in the events store

---

## [v1.0.0-beta.1] — 2026-03-10

### Added
- Initial release of ZapLab
- Core whatsmeow integration with PocketBase v0.36
- REST API for sending text, image, video, audio, and documents
- Real-time event persistence and dispatch to a default webhook
- QR code pairing and account management UI
- Contacts and Groups listing and basic management
- Docker and Docker Compose orchestration

[Unstable]: https://github.com/lichti/zaplab/tree/main
[Dev]: https://github.com/lichti/zaplab/tree/dev
[v1.0.0-beta.6]: https://github.com/lichti/zaplab/compare/v1.0.0-beta.5...v1.0.0-beta.6
[v1.0.0-beta.5]: https://github.com/lichti/zaplab/compare/v1.0.0-beta.4...v1.0.0-beta.5
[v1.0.0-beta.4]: https://github.com/lichti/zaplab/compare/v1.0.0-beta.3...v1.0.0-beta.4
[v1.0.0-beta.3]: https://github.com/lichti/zaplab/compare/v1.0.0-beta.2...v1.0.0-beta.3
[v1.0.0-beta.2]: https://github.com/lichti/zaplab/compare/v1.0.0-beta.1...v1.0.0-beta.2
[v1.0.0-beta.1]: https://github.com/lichti/zaplab/releases/tag/v1.0.0-beta.1
