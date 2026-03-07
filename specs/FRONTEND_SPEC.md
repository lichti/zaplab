# Frontend Spec — ZapLab

## Goal

Rewrite `pb_public/index.html` with a modern layout: collapsible sidebar, dark/light mode support, and a lightweight framework — keeping the file as a **single-file HTML** (no build step, served directly by PocketBase static files).

---

## Stack

- **[Alpine.js](https://alpinejs.dev/)** via CDN — declarative reactivity, no build
- **[Tailwind CSS](https://tailwindcss.com/docs/installation/play-cdn)** via Play CDN — utility CSS, no build
- **[PocketBase JS SDK](https://github.com/pocketbase/js-sdk)** via CDN — realtime + REST

No bundler, no node_modules. Everything via `<script src="...">` / `<link>`.

---

## Layout

```
┌─────────────────────────────────────────────────────────┐
│  [≡] ZapLab                            [◐ dark/light]   │  ← Topbar
├───────────┬─────────────────────────────────────────────┤
│           │                                             │
│  Sidebar  │  Main Content Area                          │
│  (left)   │                                             │
│           │                                             │
│ [collapse]│                                             │
│           │                                             │
└───────────┴─────────────────────────────────────────────┘
```

### Topbar
- App logo/name on the left
- Hamburger button `[≡]` to collapse/expand the sidebar
- Dark/light mode toggle on the right (moon/sun icon)
- WhatsApp connection status indicator (colored dot + text)

### Sidebar (left navigation)
- Expanded width: `240px`
- Collapsed width: `48px` (icons only)
- Smooth transition (`transition-all duration-300`)
- Menu items with icon + label; when collapsed, show icon only with tooltip
- State persisted in `localStorage`

#### Menu items
| Icon     | Label       | Section                         |
|----------|-------------|---------------------------------|
| activity | Live Events | real-time event viewer          |
| send     | Send Message| message send form               |
| settings | Settings    | general settings (future)       |

### Main Content Area
- Fills the remaining screen space
- Renders the active section based on the selected menu item
- Smooth transition when switching sections

---

## Sections

### 1. Live Events (home screen)

Reproduces current functionality with an improved layout:

- Top panel: real-time event list
  - Each row: timestamp | type badge | payload preview
  - Color-coded badge by type (`message`, `receipt`, `connected`, `disconnected`, `history`, `presence`, `sent`)
  - Animated "NEW" badge on recently arrived events
  - Auto-scroll to top on new events (configurable behavior)
- Resizable splitter (vertical drag) between list and detail
- Bottom panel: selected event detail
  - JSON with syntax highlight (keys, strings, numbers, booleans, null in distinct colors)
  - "Copy JSON" button
- Event counter in the panel header
- "Clear" button to clear the local list (does not delete from database)

Connection behavior:
- Connects to PocketBase Realtime via `pb.collection('events').subscribe('*', ...)`
- Loads last 100 events on init (`getList(1, 100, { sort: '-created' })`)
- Automatically reconnects on disconnect (retry with 3s backoff)
- Status indicator: `connecting` (yellow) | `live` (green) | `disconnected` (red)

### 2. Send Message

Form for sending messages via the bot API:

Fields:
- **To** — destination number (e.g. `5511999999999` or `5511999999999@s.whatsapp.net`)
- **Type** — select: `text` | `image` | `video` | `audio` | `document`
- **Message / Caption** — textarea (visible for `text` and as caption for media)
- **File** — file input (visible for media types)
- **PTT** — checkbox (visible for `audio`)

Behavior:
- Calls the appropriate endpoint with the payload matching the selected type
- Shows loading spinner while waiting for response
- Displays success or error toast with the API response
- Conditional fields based on selected type (Alpine.js `x-show`)

See endpoint reference in `internal/api/api.go`.

---

## Dark / Light Mode

- Toggle persisted in `localStorage` (`zaplab-theme`)
- `dark` class applied on `<html>` (compatible with Tailwind `darkMode: 'class'`)
- Tailwind Play CDN: configure via `tailwind.config = { darkMode: 'class' }` in `<script>` before the CDN
- Suggested palette:
  - **Dark**: background `#0d1117`, surface `#161b22`, border `#30363d`, text `#c9d1d9`
  - **Light**: background `#f6f8fa`, surface `#ffffff`, border `#d0d7de`, text `#24292f`

---

## State persistence (localStorage)

| Key                     | Value                          | Description            |
|-------------------------|--------------------------------|------------------------|
| `zaplab-theme`          | `dark` \| `light`              | active theme           |
| `zaplab-sidebar`        | `expanded` \| `collapsed`      | sidebar state          |
| `zaplab-active-section` | `events` \| `send` \| `settings` | active section       |

---

## Technical requirements

- Single file: `pb_public/index.html`
- No local dependencies — everything via CDN
- Compatible with modern browsers (Chrome, Firefox, Safari, Edge — last 2 versions)
- Responsive: on screens `< 768px` the sidebar starts collapsed by default and overlays the content as a drawer
- Basic accessibility: `aria-label` on icon buttons, `title` on collapsed sidebar items

---

## CDN URLs (use fixed versions, not `latest`)

```html
<!-- Tailwind CSS Play CDN -->
<script src="https://cdn.tailwindcss.com"></script>

<!-- Alpine.js -->
<script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>

<!-- PocketBase JS SDK -->
<script src="https://cdn.jsdelivr.net/npm/pocketbase/dist/pocketbase.umd.js"></script>
```

> Replace `3.x.x` with the latest stable Alpine.js version at implementation time.

---

## Alpine.js structure

Use a single global `x-data` on `<body>` or `<div id="app">` with shared state:

```js
{
  theme: localStorage.getItem('zaplab-theme') || 'dark',
  sidebarExpanded: localStorage.getItem('zaplab-sidebar') !== 'collapsed',
  activeSection: localStorage.getItem('zaplab-active-section') || 'events',
  connectionStatus: 'connecting', // 'connecting' | 'connected' | 'disconnected'
  events: [],
  selectedEvent: null,
  // ... send form state
}
```

---

## What NOT to change

- PocketBase Realtime connection logic (`pb.collection('events').subscribe`)
- The send endpoint (`/api/send`) — consume only, do not modify
- The file must remain at `pb_public/index.html`
- No other project files should be modified
