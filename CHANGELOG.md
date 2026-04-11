# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Backlog]

---

## [Dev]

### Added
- **Script Repository** — new `scripts/` directory containing example `.js` scripts with howto comments at the top. Each script documents how to configure it in the UI (Name, Description, Timeout, Event type, Text pattern). Establishes the convention for reusable community scripts. Example: `scripts/group-ranking.js`.
- **`/ranking` Group Activity Command** — script trigger that posts top-5 most active, top-5 least active, and silent member count in a group when `/ranking <dias>` is sent by the bot. Sources data from `GET /zaplab/api/groups/<jid>/overview?period=<dias>`. Only fires in groups and only when the bot sends the command. Example script at `scripts/group-ranking.js`.
- **`zaplab.api(path)` sandbox function** — calls the internal ZapLab REST API from within a script. Auth auto-injected: uses `API_TOKEN` env var as `X-API-Token` header if set; otherwise mints a 2-minute PocketBase superuser JWT. Returns the parsed JSON body as a JS object. Panics on non-2xx response or network error.
- **`scripts.code` field size raised to 65 535 chars** — migration `1747999000_increase_script_code_size.go` raises the PocketBase TextField max from the default 5 000 to 65 535, allowing larger scripts.
- **Network Graph A.3 — Direction, Visual Options, Shortest Path, Multi-select** — Direction selector (Any/Sent/Received), Event Types multi-select filter, Contact filter; Visual options panel (Labels: auto/always/hover/hidden; Node size: messages/uniform; Edge color: count/direction); Shortest path panel (Set A / Set B / Find path / Clear + hop count display); Multi-select summary (total msgs, per-node remove buttons); Focus 1-hop toggle button per selected node.
- **Inline audio and video players in Conversation View** — media messages in the conversation bubble view now show inline `<video>` and `<audio>` players.
- **Contact Overview Dashboard** — rich analytics profile for a single contact JID.
  - New endpoint `GET /zaplab/api/contacts/{jid}/overview?period=N`.
  - Returns: DM stats (total/received/sent/media), group activity, edit/delete counts, common groups (up to 15 with names), activity heatmap 7×24, daily sparkline, first/last seen.
  - **Frontend**: sidebar contact list with search, summary cards, sparkline, heatmap, common groups table with quick-link to Group Overview, action buttons (View Chat, Search, Network Graph).
- **Group Overview Dashboard** — rich analytics panel for a single group JID.
  - New endpoint `GET /zaplab/api/groups/{jid}/overview?period=N`.
  - Returns: summary (total messages/media/active-members/known-members/silent-members/edited/deleted), member ranking top-25 with name enrichment, activity heatmap 7×24, daily sparkline, membership evolution (joins/leaves per day), message type distribution.
  - **Frontend**: sidebar group list with search, summary cards, sparkline, heatmap, member ranking table with Profile button (→ Contact Overview), type distribution bars, membership events log, action buttons (View Chat, Members, Search).
- **Network Graph — Advanced Filters** — new filter panel in the Network Graph section.
  - New query params: `date_from`, `date_to` (ISO-8601, override `period`), `min_msgs` (minimum node message count), `include_groups` (boolean).
  - **Frontend**: collapsible "Filters" panel with date pickers, min-msgs input, groups toggle, Reset and Apply buttons.
- **Network Graph — Export** — three export buttons added to the header: **PNG** (canvas snapshot), **JSON** (nodes + edges + stats), **CSV** (edge list).

- **Device Activity Tracker** — RTT-based device state inference (Online / Standby / Offline) ported from `gommzystudio/device-activity-tracker`. Feature-flagged; disabled by default.
  - **Mechanism**: sends a delete-revoke or reaction probe to the target JID every ~2 s (with up to 500 ms jitter); measures delivery receipt RTT; classifies state using a 3-sample moving average vs. 90% of the 2000-sample global median RTT. Timeout of 10 s without receipt → Offline.
  - **Data**: two new PocketBase collections — `device_activity_sessions` (jid, probe_method, started_at, stopped_at) and `device_activity_probes` (session_id, jid, rtt_ms, state, median_ms, threshold_ms).
  - **API**: `POST /zaplab/api/activity-tracker/enable`, `POST .../disable`, `POST .../start` (jid, probe_method), `POST .../stop`, `GET .../status`, `GET .../history`, `GET .../contacts`, `POST .../start-bulk` (jids[], probe_method), `POST .../stop-all`.
  - **Frontend**: "Activity Tracker" section with three tabs — *Active Trackers* (start/stop per JID, live Online/Standby/Offline badges, 5 s auto-poll), *Contacts* (full contact list from whatsmeow store with checkboxes, Track Selected / Track All / Stop All / search / quick-select buttons), *Probe History* (RTT table with state, median, threshold per probe). Feature enable/disable toggle at section header. Risk warning always visible.
  - **Receipt hook**: `events.Receipt` handler now calls `NotifyProbeReceipt()` for each message ID so the tracker goroutine can measure RTT without blocking the event loop.
  - **Risks**: WhatsApp rate-limit/ban risk; only works with contacts that have you saved; legal/privacy — use only with own accounts or explicit consent.
- **Presence Subscribe** — `POST /zaplab/api/presence/subscribe` subscribes to presence events (Online / Offline / Last Seen) for a given JID via `client.SubscribePresence()`. Events are persisted automatically by the existing `events.Presence` handler and queryable via `GET /zaplab/api/presence/timeline`. Note: WhatsApp only delivers presence for contacts that have you saved — non-contacts are silently ignored server-side.
- **Re-pair without server restart** — `POST /zaplab/api/wa/repaire` tears down the current whatsmeow client, reopens the store, opens a fresh QR channel, and reconnects — allowing the user to scan a new QR code directly from the Pairing section after a logout without restarting the server. The frontend Pairing section now shows a **Re-pair** button instead of "Restart the server" when status is `loggedout`.

### Performance
- **Async event handler** — `handler()` (called synchronously by whatsmeow's node-processing goroutine) now returns immediately for all event types. Connection lifecycle events (`Connected`, `Disconnected`, `LoggedOut`, `StreamReplaced`) update in-memory state synchronously (fast mutex ops) and dispatch I/O to goroutines; all other events (`Message`, `Presence`, `ChatPresence`, `Receipt`, `GroupInfo`, `HistorySync`, etc.) are dispatched via `go handleAsync()`. Eliminates the "Node handling took Xs" WARN for all event types and prevents event backlog under high load.

### Fixed
- **LoggedOut event not handled** — `events.LoggedOut` was never dispatched in the event handler, so the connection status remained stuck on `disconnected` and the `loggedout` state was never set. Now properly handled: sets `StatusLoggedOut`, logs the reason, and records a `conn_events` entry.
- **Trigger script execution logging** — `dispatchTriggers` now logs each trigger fire (trigger_id, script_id, event_type, chat) and persists `last_run_status`, `last_run_output`, `last_run_duration_ms`, `last_run_error` on the script record after every trigger execution. Previously all output and errors were silently discarded.
- **IIFE requirement for scripts** — top-level `return` is a syntax error in goja; all scripts that need conditional early exit must use the IIFE pattern `(function(){ ... })()`. The `group-ranking.js` example script was updated to follow this pattern.
- **Activity Heatmap not rendering** — two bugs: (1) Alpine `x-for` cannot have multiple sibling `<template x-if>` children (single root required); fixed with a `display:contents` wrapper div and a separate `stHeatRows()` method; (2) `stCellStyle()` returned `''` for count=0, leaving the CSS `--heat-light`/`--heat-dark` variables undefined and making empty cells transparent (overriding the intended `bg-gray-100`); fixed by always emitting both CSS variables, including for count=0 (`#ebedf0`/`#161b22`).
- **Activity Heatmap period filter not reactive** — pre-computed `stHeatRows()` cells were plain JS objects stored in a scope variable; inner `x-for` bindings over `row.cells` did not re-evaluate when `stHeatCells` changed; replaced `stHeatRows()` with direct nested `x-for` over `stDayLabels`/`stHourLabels`, each cell's `:style` calling `stHeatCount(d, h)` inline so Alpine tracks the reactive dependency per cell.
- **Shutdown error flood** — on Ctrl+C, PocketBase closed the DB while WhatsApp was still delivering events, causing ~30+ `"database is closed"` ERROR lines in the log. Added `shuttingDown atomic.Bool` (set via `OnTerminate` hook before the DB closes); `saveEvent`, `saveEventFile`, and `saveError` now return `nil` silently when the flag is set, eliminating the noise. Supersedes the previous `pb.DB() == nil` guard which was insufficient because the DB pointer is non-nil even after the connection closes.
- **Receipt event handler blocking whatsmeow (75 s stall)** — large group receipt nodes (200+ participants) triggered one `saveEvent` DB write per participant synchronously inside whatsmeow's node-handler goroutine, causing `"Node handling took 75s"` WARN. Receipt persistence is now dispatched in a goroutine, returning control to whatsmeow immediately.
- **Shutdown crash (nil DB panic)** — background goroutines (event handler, log consumer) accessed PocketBase after DB was closed on shutdown, causing a nil pointer panic; added `pb.DB() == nil` early-exit guards in `saveEvent`, `saveEventFile`, `saveError`, `persistLogEntry`, and a `recover()` deferred in `StartLogConsumer`.
- **Device suffix in Group Overview / Contact Overview names** — `enrichName` in `groupoverview.go` now strips the `:device` suffix before contact name lookup (`123:46@s.whatsapp.net` → `123@s.whatsapp.net`), fixing names displaying as raw JIDs instead of the contact's push name.
- **Group Overview Members tab** — all members now shown (not only active ones); Name/Messages sort; silent members fixed for LID/PN and device-suffix mismatches.
- **Contact Overview — silent group membership** — contacts who are in a group but sent no messages (silent members) now appear in the "Groups" tab of Contact Overview.
- **Contact Overview — group names fallback** — uses `GetGroupInfo()` as final fallback for group names when the local store does not have them.
- **Event Browser — Inspect JSON** — now opens the Event Browser (not Live Events) when navigating from the JSON inspector; added support for `contact`, `poll`, and `enc_reaction` message types.
- **Stats & Heatmap — scroll** — section now scrolls correctly when content overflows.

---

## [v1.0.0-beta.8] — 2026-03-17

### Added
- **Connection Stability Dashboard** — new section for monitoring WhatsApp connection health over time.
  - **Backend**: new PocketBase collection `conn_events` (`event_type`, `reason`, `jid`); migration `1747300000_create_conn_events.go`. Events are recorded asynchronously from the `Connected` and `Disconnected` whatsmeow event handlers in `internal/whatsapp/conntrack.go`.
  - Two new endpoints: `GET /zaplab/api/conn/events` (paginated event log with date filter) and `GET /zaplab/api/conn/stats` (counts + uptime percentage over last 24 h / 7 d / 30 d windows).
  - **Frontend**: timeline card list with connected/disconnected badges, uptime percentage chips, and period selectors.
- **Script Import / Export** — backup and restore the full script library with a single click.
  - `GET /zaplab/api/scripts/export` — returns all scripts as a JSON array bundle.
  - `POST /zaplab/api/scripts/import` — upserts scripts by name; preserves `enabled` and `timeout_secs` on update.
  - **Frontend**: **Export** and **Import** buttons added to the Scripting section header toolbar.
- **WA Health Monitor** — new section exposing the health of internal whatsmeow state.
  - **Pre-key Health** tab: reads `whatsmeow_pre_keys` via `GET /zaplab/api/wa/prekeys`; shows total, uploaded count, and a visual progress bar that turns red when the uploaded supply is critically low.
  - **Message Secrets** tab: reads `whatsmeow_message_secrets` via `GET /zaplab/api/wa/secrets`; paginated table of all stored per-message encryption secrets.
  - Uses the `dbeTableColumns` / `dbeScanRows` helpers from `dbexplorer.go`; custom `rowsToMaps` helper zips column names with `[][]any` rows before JSON serialisation.
- **IQ Node Analyzer** (`frames-iq` section, IQ tab) — filters the persistent `frames` table for entries whose `msg` contains `<iq` and displays them with level and IQ-type (get / set / result / error) filter dropdowns.
- **Binary Node Inspector** (`frames-iq` section, Binary tab) — filters the `frames` table for `module IN ('Noise','Socket')`, exposing the binary protocol layer; filter by level and module; expandable detail per entry.
  - **Backend** for both analyzers: `GET /zaplab/api/frames/iq` and `GET /zaplab/api/frames/binary` in `internal/api/framesiq.go`. Filters use `dbx.NewExp` / `dbx.HashExp` against the `frames` collection. No `direction` or `raw` column — filters on `level` and searches `msg LIKE '%<iq%'`.
- **Group Membership Tracker** — records every `events.GroupInfo` event (join, leave, promote, demote) to a new `group_membership` collection.
  - **Backend**: migration `1747310000_create_group_membership.go`; `recordGroupMembership(*events.GroupInfo)` in `internal/whatsapp/conntrack.go` writes one row per affected member.
  - Two endpoints: `GET /zaplab/api/groups/{jid}/history` (history for a specific group) and `GET /zaplab/api/groups/membership` (all events, paginated).
  - **Frontend**: table view with action badge (join=green, leave=red, promote=blue, demote=yellow), JID filters, and a group-history drill-down mode.
- **Message Secret Inspector** — see *WA Health Monitor* above (tab within the same section).
- **Audit Log** — records every mutating API operation for audit and research purposes.
  - **Backend**: `auditMiddleware()` in `internal/api/auditlog.go` buffers the request body (up to 64 KB), restores it with `io.NopCloser`, and saves a record asynchronously to the new `audit_log` collection (migration `1747320000_create_audit_log.go`). Applied to `POST /sendmessage`, `POST /scripts/{id}/run`, `POST /scripts/run`, and `POST /scripts/import`.
  - `GET /zaplab/api/audit` — paginated, filterable audit log browser.
  - **Frontend**: table with method badge, path, remote IP, timestamp, and expandable request body.

### Fixed
- **Frame Analyzers — `no such column: direction`** — the `frames` PocketBase collection has no `direction` or `raw` columns; fixed `framesiq.go` to filter only on `level` and `msg LIKE '%<iq%'`; fixed frontend dropdowns and display columns to use `fiq.iqLevel` / `fiq.binLevel` instead of removed direction fields.
- **DB Sandbox — `LIMIT` syntax error** — `postDBQuery` was unconditionally appending ` LIMIT 1000` even when the user query already contained a `LIMIT` clause, producing `SELECT ... LIMIT 50 LIMIT 1000`; fixed by checking `strings.Contains(upper, " LIMIT ")` before appending.
- **DB Sandbox — `no such column: version`** — the Sessions quick-access example referenced a non-existent `version` column in `whatsmeow_sessions`; removed it from the example query.

- **Conversation View** — new two-pane section for browsing chat history as a chat bubble interface.
  - **Backend**: `GET /zaplab/api/conversation/chats?limit=N` returns a deduplicated list of chats ordered by most-recent message; `GET /zaplab/api/conversation?chat=...&limit=N&before=RFC3339` returns paginated messages with `has_more` / `next_before` cursor for infinite scroll upward.
  - Messages queried via `json_extract(raw, '$.Info.Chat')` from the `events` table; media type detected from proto fields (text, image, video, audio, document, sticker, location, reaction).
  - **Frontend**: left chat list with filter and last-message/timestamp preview; right bubble area with sent/received alignment, inline media thumbnails, and a "Load older messages" button.
  - Drawer panel for raw event JSON on any message click.
  - Navigable from the Search section via "Open conversation" link.
- **Script Triggers (Event Hooks)** — automatically execute stored scripts when WhatsApp events arrive.
  - **Backend**: new PocketBase collection `script_triggers` (fields: `script_id`, `event_type`, `jid_filter`, `text_pattern`, `enabled`); migration `1743000000_create_script_triggers.go`.
  - Dispatch wired via `whatsapp.TriggerDispatchFunc` callback (set in `api.InitTriggerDispatch()`) to avoid import cycles; triggers fire asynchronously after every saved event with the full event JSON injected as `event` into the goja sandbox.
  - Optional `jid_filter` (substring match on `Info.Chat`) and `text_pattern` (case-insensitive contains on message text) for selective targeting.
  - CRUD: `GET /zaplab/api/script-triggers`, `POST /zaplab/api/script-triggers`, `PATCH /zaplab/api/script-triggers/{id}`, `DELETE /zaplab/api/script-triggers/{id}`.
  - `GET /zaplab/api/script-triggers/event-types` — returns all distinct event types found in the `events` table plus canonical fallbacks.
  - **Frontend**: Triggers section with a list view (event type, script name, filter summary, enabled toggle) and inline edit form; new-trigger form with event type + script dropdowns.
- **Full-text Message Search** — new Search section for querying message content across all chats.
  - `GET /zaplab/api/search?q=...&type=...&chat=...&limit=50&offset=0` — LIKE search across `Conversation`, `ExtendedTextMessage.Text`, and media caption fields; also matches exact `msgID`.
  - Paginated results showing sender JID, chat, message type, text preview, and timestamp.
  - **Frontend**: search bar with optional type and chat filters; result cards with "Open conversation" deep link; raw event drawer for any result; keyboard shortcut (Enter to search).
- **Expanded `wa.*` scripting bindings** — 11 new functions available in the goja sandbox:
  - `wa.jid` — own JID string of the connected account.
  - `wa.sendImage(to, base64Data, mime, caption)` — send an image message.
  - `wa.sendAudio(to, base64Data, mime, ptt)` — send audio (voice note when `ptt=true`).
  - `wa.sendDocument(to, base64Data, mime, filename, caption)` — send a file attachment.
  - `wa.sendLocation(to, lat, lng, name)` — send a location pin.
  - `wa.sendReaction(to, msgId, emoji)` — react to a message.
  - `wa.editMessage(to, msgId, newText)` — edit a previously sent text message.
  - `wa.revokeMessage(to, msgId)` — delete a sent message for everyone.
  - `wa.setTyping(to, typing)` — set composing/paused presence.
  - `wa.getContacts()` — return all stored contacts as an array.
  - `wa.getGroups()` — return all joined groups as an array.
  - `wa.db.query(sql, params)` — query the `whatsapp.db` SQLite database directly.
  - `runScript` now accepts an `env map[string]any` parameter for injecting variables (e.g., `event` for triggers).
- **Media Gallery** — new section for browsing all downloaded media files.
  - `GET /zaplab/api/media/gallery?type=...&chat=...&limit=50&offset=0` queries events with attached files (`file != ''`); media type detected via JSON field presence.
  - Returns `{items: [{id, msgID, chat, sender, is_from_me, media_type, file_url, thumb_url, caption, created}], total, limit, offset}`.
  - **Frontend**: responsive grid (2–5 columns) with type icon overlay and date badge; type/chat filters; pagination; lightbox viewer for images with video and audio inline playback; Escape key closes lightbox.
- **Scripting example scripts** — the ad-hoc console now includes 10 built-in example snippets: List Tables, Recent Messages, Messages per Chat, Top Event Types, Contacts from WA DB, Recent WA Events, Send Text, Get Status, HTTP GET, Sleep & Log.
- **`wa.sendDocument` backend** — new `whatsapp.SendDocumentFile(to, data, filename, caption)` function setting `FileName` and `Caption` on the `DocumentMessage` proto.

### Fixed
- **`db.query` sandbox function** — was failing with `"Invalid variable type: must be a slice of struct or NullStringMap"` when using `pb.DB().NewQuery().All()`. Fixed by using `[]dbx.NullStringMap` and converting `sql.NullString` values to `map[string]any` before returning to the JS sandbox.

- **Plugin System / Scripting Engine** — JavaScript sandbox for automating WhatsApp interactions directly from the browser.
  - **Backend**: new `scripts` PocketBase collection (`name`, `description`, `code`, `enabled`, `timeout_secs`, last-run metadata); 6 REST endpoints: `GET /zaplab/api/scripts`, `POST /zaplab/api/scripts`, `PATCH /zaplab/api/scripts/{id}`, `DELETE /zaplab/api/scripts/{id}`, `POST /zaplab/api/scripts/{id}/run`, `POST /zaplab/api/scripts/run` (ad-hoc).
  - **Sandbox** — goja JavaScript engine (`github.com/dop251/goja`) with configurable per-script timeout; exposes: `console.log/error/warn`, `wa.sendText(jid, text)`, `wa.status()`, `http.get(url)`, `http.post(url, body)`, `db.query(sql)`, `sleep(ms)`.
  - **Frontend**: Scripting section with a three-panel layout — sidebar script list (status icon + last-run indicator), code editor (`<textarea>` with monospace font), output panel (stdout, exit status, duration in ms).
  - **Ad-hoc console** — run JavaScript snippets immediately without saving (useful for quick exploration).
  - Script CRUD: create, edit, enable/disable, set timeout, save, delete — all without leaving the UI.
  - Last-run metadata (`status`, `output`, `error`, `duration_ms`) stored back to PocketBase after each execution.
  - New migration `1742700000_create_scripts.go`.
- **Multi-Session Comparator** — side-by-side comparison of up to 6 Signal Protocol sessions or group SenderKey records.
  - Sessions tab: checkbox selection from the full session list; comparison table with properties as rows and sessions as columns; differences relative to the first selected session highlighted in amber; diff count badge (0 diffs = green, N > 0 = amber).
  - Sender Keys tab: same UX for group SenderKey records (key ID, iteration, signing key, decode error).
  - Compared properties: version, has sender chain, sender counter, receiver chains, previous counter, previous states, raw size, remote/local identity key, decode error.
  - No new backend endpoints — reuses the existing `/zaplab/api/signal/sessions` and `/zaplab/api/signal/senderkeys`.
- **Network Graph** — interactive force-directed contact/group relationship graph built from stored Message events.
  - **Backend**: new `GET /zaplab/api/network/graph?period=<days>` endpoint scans `events WHERE type='Message'`, extracts `Info.Chat` / `Info.Sender` / `Info.IsGroup` from the raw JSON in Go, aggregates nodes (self, contacts, groups, broadcast lists) and edges (device ↔ chat message count; sender ↔ group membership), enriches node labels from `whatsmeow_contacts` push names, returns top-100 nodes by message count with corresponding edges.
  - **Frontend**: pure-JS Verlet force simulation (repulsion + spring attraction + centre gravity + damping); HTML `<canvas>` renderer (dark/light mode aware); node radius scales with `√(msg_count)`; edge opacity and thickness scale with weight.
  - Period selector (7d / 30d / 90d / 365d / all time); Pause/Resume simulation button; stats chips (node count, edge count, message count).
  - Node detail panel (click to inspect): type, label, JID, message count, connection list with per-edge weight.
  - Mouse interaction: drag nodes to pin/reposition; hover shows labels for all nodes; self node always pinned to centre.
  - Legend: self (blue), contact (orange), group (green), broadcast (purple).
- **App State Inspector** — new dashboard section that exposes the three whatsmeow app state SQLite tables for protocol research.
  - **Collections tab**: reads `whatsmeow_app_state_version`; shows every known collection (`critical`, `regular`, `critical_unblock_to_primary`, `critical_block`, `regular_low`) with its current version index, Merkle-tree state hash, and a plain-English description; filterable by name or JID.
  - **Sync Keys tab**: reads `whatsmeow_app_state_sync_keys`; shows key ID (hex), creation timestamp, fingerprint (hex), and key size in bytes for every app state decryption key; raw key bytes are withheld.
  - **Mutations tab**: reads `whatsmeow_app_state_mutation_macs`; shows the per-leaf HMAC integrity codes (index MAC + value MAC) for any selected collection, ordered by version descending; configurable row limit.
  - Three new backend endpoints: `GET /zaplab/api/appstate/collections`, `GET /zaplab/api/appstate/synckeys`, `GET /zaplab/api/appstate/mutations?collection=<name>`.
- **PCAP Export** — export captured frame log entries as a standard libpcap (`.pcap`) file for analysis in Wireshark or tcpdump.
  - New endpoint `GET /zaplab/api/frames/pcap?module=&level=&limit=` queries the `frames` PocketBase collection and writes a valid PCAP binary with a global header (link type `LINKTYPE_ETHERNET`, microsecond timestamps).
  - Each frame log entry is wrapped in a synthetic Ethernet/IPv4/UDP packet (`127.0.0.1:443 → 127.0.0.2:12345`) with the log entry as JSON in the UDP payload, preserving the original timestamp.
  - **Export PCAP button** added to the Frame Capture toolbar, respecting the active module/level/search filters.
- **Advanced Stats & Heatmap** — new dedicated Stats section with activity analytics.
  - **Activity heatmap**: GitHub-style 7×24 grid (day-of-week × hour-of-day) showing message density; 5-level green colour scale; peak hour label; period selector (7d / 30d / 90d / 1y / all time).
  - **Daily sparkline**: SVG bar chart of message volume per day, with min/max/avg labels; fills gaps automatically.
  - **Event type distribution**: horizontal bar chart of top 15 event types with relative widths.
  - **Summary cards**: Total messages, Last 24h, Edited count + rate, Deleted count + rate.
  - Pure HTML/CSS/SVG — no external charting libraries.
  - 5 new backend endpoints powered by raw SQLite queries on PocketBase's own DB:
    `GET /zaplab/api/stats/heatmap`, `GET /zaplab/api/stats/daily`, `GET /zaplab/api/stats/types`,
    `GET /zaplab/api/stats/summary`, `GET /zaplab/api/stats/editchain`.
- **Enhanced Message Diff** — major upgrade to the Edit Diff panel in Message History.
  - **View toggle**: switch between **inline** (additions and removals in a single stream) and **side-by-side** (before / after panels with coloured backgrounds).
  - **Granularity toggle**: switch between **word-level** (default, fast) and **character-level** diff (individual characters as tokens, useful for small edits).
  - **Diff statistics bar**: shows `+N added / −M removed` token counts and a **similarity percentage** computed from the LCS result.
  - **Edit chain timeline**: loads all events that share the same `msgID` from the new `/stats/editchain` endpoint and renders a chronological vertical timeline with kind badges (original / edit / delete) and message content at each step.
- **Signal Session Visualizer** — new dashboard section that decodes all Signal Protocol Double Ratchet sessions and group SenderKey records stored in `whatsapp.db`.
  - **Individual sessions** tab: decodes every row in `whatsmeow_sessions`; shows address, session version (v2/v3/v4), sender chain counter, receiver chain count, previous counter, archived state count, remote identity public key (hex), and raw blob size.
  - **Group sender keys** tab: decodes every row in `whatsmeow_sender_keys`; shows chat ID, sender ID, key ID, chain iteration (messages sent), signing key public (hex), and raw blob size.
  - Health indicator per row: ✓ active / ⚠ no sender chain / ✗ decode error.
  - JID filter for both tabs.
  - `GET /zaplab/api/signal/sessions` — list all decoded session states.
  - `GET /zaplab/api/signal/senderkeys` — list all decoded sender key states.
- **Event Annotations** — new section for attaching research notes and tags to any WhatsApp protocol event.
  - New PocketBase `annotations` collection: `event_id`, `event_type`, `jid`, `note`, `tags` (JSON array), autodate `created`/`updated`; indexed by `event_id` and `jid`.
  - Full CRUD API: `GET /zaplab/api/annotations`, `POST /zaplab/api/annotations`, `PATCH /zaplab/api/annotations/{id}`, `DELETE /zaplab/api/annotations/{id}`.
  - Annotation list page: filterable by event ID or JID, paginated, tag chips with per-tag color, realtime subscription (PocketBase), inline edit/delete.
  - Modal editor: context fields (event ID, JID auto-filled when opened from Event Browser), multi-line note, comma-separated tags input.
  - **Event Browser integration**: each event detail header now shows an **Annotate** button that opens the modal with event ID, type, and sender JID pre-filled.
- **Frame Capture** — real-time log stream browser backed by a custom `waLog.Logger` wrapper (`CapturingLogger`) that intercepts every log call from whatsmeow (Client, Client/Socket, Client/Send, Client/Recv, Database sub-loggers).
  - All levels buffered to an in-memory ring buffer (2 000 entries, thread-safe).
  - INFO/WARN/ERROR entries also persisted to a new PocketBase `frames` collection for historical queries.
  - **Live mode** (ring buffer): shows DEBUG through ERROR; includes XML node frames at DEBUG log level when `--log-level DEBUG` is configured.
  - **DB mode** (PocketBase): persistent INFO+ history with server-side filter by module, level, and text search.
  - Real-time updates via PocketBase subscription (DB mode); module badge, level badge, message, timestamp per row; expandable full-entry detail.
  - `GET /zaplab/api/frames` — paginated DB query; `GET /zaplab/api/frames/ring` — ring buffer snapshot; `GET /zaplab/api/frames/modules` — distinct modules.
- **Noise Handshake Inspector** — annotated visualisation of the WhatsApp `Noise_XX_25519_AESGCM_SHA256` handshake.
  - Step-by-step timeline: Setup → ClientHello → ServerHello → Certificate verification → ClientFinish → Key split, with cryptographic detail per step (HKDF, ECDH, AES-GCM IVs, certificate chain).
  - **Device public key panel**: shows device JID, Noise static public key (Curve25519, hex), Identity public key (Ed25519, hex), registration ID, platform, and push name — no private keys exposed.
  - Live connection events panel: filters ring buffer for Client-module connection/handshake/disconnect log entries.
  - `GET /zaplab/api/wa/keys` — returns public key material only.
- **New PocketBase collection `frames`** — stores INFO+ log entries with `module`, `level`, `seq`, `msg`, and autodate `created`/`updated` fields; indexed by module, level, and created.
- **Protocol Timeline** — new dashboard section with a vertical chronological timeline of all WhatsApp protocol events.
  - Color-coded event type badges (Message, Receipt, Presence, HistorySync, AppStateSync, Connected, Disconnected, QR, PairSuccess, CallOffer, GroupInfo, etc.).
  - Per-event human-readable summary extracted from protocol fields (sender JID, message preview, sync type, disconnect reason, etc.).
  - Real-time updates via PocketBase `events` collection subscription; pause/resume toggle.
  - Filter by event type (dropdown) and free-text search across type and JSON payload.
  - Expandable JSON detail side panel with one-click copy.
  - Limits to last 200 events in memory; thread-safe realtime append.
- **Proto Schema Browser** — new dashboard section exposing the full WhatsApp protobuf schema at runtime.
  - Backend (`GET /zaplab/api/proto/schema`): enumerates all registered proto types via `protoregistry.GlobalTypes` after blank-importing all 56 whatsmeow proto packages at startup.
  - `GET /zaplab/api/proto/message?name=<FullName>`: fetch detail for a single message type (supports nested types not in the top-level index).
  - Frontend: two-panel layout — left sidebar with package filter, search, and scrollable message/enum list; right detail panel with fields table (number, name, type, label, oneof group), nested message navigation (click any `message` or `enum` field type to drill in), breadcrumb back-navigation, nested-message and nested-enum reference chips.
  - Schema is computed once at startup and cached with `sync.Once` for zero-overhead subsequent calls.
  - Full stats: total messages, enums, and packages shown in the header.
- **DB Explorer** — new section in the dashboard to browse, edit, and restore all 12 internal whatsmeow SQLite tables (`whatsmeow_device`, `whatsmeow_identity_keys`, `whatsmeow_pre_keys`, `whatsmeow_sessions`, `whatsmeow_sender_keys`, `whatsmeow_app_state_sync_keys`, `whatsmeow_app_state_version`, `whatsmeow_app_state_mutation_macs`, `whatsmeow_contacts`, `whatsmeow_chat_settings`, `whatsmeow_message_secrets`, `whatsmeow_privacy_tokens`).
  - **Read**: paginated table view with free-text filter; column-level protocol documentation (hover tooltips) explaining each field in Signal, Noise, and WhatsApp protocol context; binary BLOB values displayed as lowercase hex strings.
  - **Write**: edit any cell inline (hex blobs decoded back to bytes automatically); delete rows with confirmation. **Automatic backup (`VACUUM INTO`) is created before every write**.
  - **Backup & Restore**: manual backup creation; backup list with restore and delete; restoring replaces `whatsapp.db`, removes WAL/SHM sidecars, and fully reinitialises the whatsmeow stack without a server restart.
  - **Reconnect controls**: "Reconnect" (disconnect + connect) and "Full Reinit" (teardown + rebuild store + reconnect) buttons to observe protocol behaviour changes after editing.
- **`whatsapp.ForceReconnect()`** — disconnects and immediately reconnects the active client (WebSocket-level).
- **`whatsapp.Reinitialize()`** — closes the whatsmeow client and `sqlstore.Container`, rebuilds from the configured DSN, and reconnects; used after DB file restore.

### Changed
- `storeContainer` promoted to a package-level variable in the `whatsapp` package to enable clean teardown during `Reinitialize()`.

---

## [v1.0.0-beta.7] — 2026-03-13

### Added
- **Embedded Static Files** — `pb_public/` is now compiled into the binary via `//go:embed`; the runtime image no longer needs a separate `pb_public/` directory. Set `ZAPLAB_DEV=1` to serve files from disk instead (hot-reload during development).
- **PocketBase Authentication** — Dashboard now requires a valid PocketBase user login.
- **Unified Auth Middleware** — REST API endpoints re-enabled with support for both PocketBase session (JWT) and static `X-API-Token` header.
- **Login Overlay** — new full-screen login UI for the dashboard.
- **API Token Management** — added a field in the Settings section to manage the local API token used for curl previews and API calls.
- **Force Password Change** — users created on first run or via CLI random password are now forced to choose a new password upon first login.
- **Password Reset CLI** — new `user reset-password` command to help regain access.
- **User Profile Management** — new profile edit section in the sidebar to update display name, email, and manually trigger password changes.
- **URI-based Navigation** — dashboard navigation now reflects in the URL bar via hash (e.g., `/#/dashboard`), enabling deep linking and browser back/forward support.
- **Automatic Redirections** — accessing `/` or `/zaplab` now automatically redirects to `/zaplab/tools/`.
- **Dashboard Quick Actions** — converted action buttons to links supporting "Open in new tab" (Ctrl/Cmd+Click).
- **Dashboard Robustness** — re-fetching data now properly falls back to previous values on partial query failures; prevented double-fetching during initialization.

### Fixed
- **Dashboard Deleted Counter** — replaced unreliable `protocolMessage` JSON search with `Info.Edit` attribute matching (`"7"` = SenderRevoke, `"8"` = AdminRevoke) to correctly detect delete/revoke events; deleted count now shows non-zero values.
- **Dashboard Edited Counter** — replaced broken `IsEdit:true` search (whatsmeow does not set `IsEdit` for protocol-message-style edits received from other clients) with `Info.Edit:"1"` attribute matching, consistent with how delete detection works.
- **Dashboard Sent/Received Counters** — messages sent from the user's own WhatsApp app arrive as `type="Message"` with `IsFromMe:true`, not as `SentMessage`; fixed `fSent` to include them and `fRecv` to exclude them.
- **Message History Section** — aligned delete/edit detection filters and `mhKind` classification with the same `Info.Edit` attribute approach for consistent results.

### Changed
- **Secure Collections** — restricted `events` and `errors` collections to authenticated users only via PocketBase rules.
- **Security Hardening** — `wa/account` endpoint is now protected; Dashboard session is now verified with the server on every page load.

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
[v1.0.0-beta.7]: https://github.com/lichti/zaplab/compare/v1.0.0-beta.6...v1.0.0-beta.7
[v1.0.0-beta.6]: https://github.com/lichti/zaplab/compare/v1.0.0-beta.5...v1.0.0-beta.6
[v1.0.0-beta.5]: https://github.com/lichti/zaplab/compare/v1.0.0-beta.4...v1.0.0-beta.5
[v1.0.0-beta.4]: https://github.com/lichti/zaplab/compare/v1.0.0-beta.3...v1.0.0-beta.4
[v1.0.0-beta.3]: https://github.com/lichti/zaplab/compare/v1.0.0-beta.2...v1.0.0-beta.3
[v1.0.0-beta.2]: https://github.com/lichti/zaplab/compare/v1.0.0-beta.1...v1.0.0-beta.2
[v1.0.0-beta.1]: https://github.com/lichti/zaplab/releases/tag/v1.0.0-beta.1
