# Frontend Architecture

## Overview

The frontend is a single-page Alpine.js 3 application served by PocketBase from `pb_public/`.
JavaScript is split into focused section files — each section is a factory function that returns
an object merged into the main `zaplab()` Alpine component.

---

## File Structure

```
pb_public/
  index.html              ← HTML structure (sidebar, sections, Alpine directives)
  css/
    zaplab.css            ← custom CSS (dark/light variables, scrollbar, syntax highlight)
  js/
    utils.js              ← shared helpers: highlight, escapeHtml, highlightCurl,
                            fmtTime, typeClass, previewText
    sections/
      events.js           ← eventsSection(): Live Events state + methods
      eventbrowser.js     ← eventBrowserSection(): Event Browser search/filter/inspect/replay
      send.js             ← sendSection(): Send Message state + methods
      sendraw.js          ← sendRawSection(): Send Raw state + methods
      ctrl.js             ← ctrlSection(): Message Control state + methods
      spoof.js            ← spoofSection(): Spoof Messages state + methods
      contacts.js         ← contactsSection(): Contacts & Polls (send-only)
      contactsmgmt.js     ← contactsMgmtSection(): Contacts Management (list/check/info)
      groups.js           ← groupsSection(): Groups state + methods
      media.js            ← mediaSection(): Media Download state + methods
      simulation.js       ← simulationSection(): Route Simulation state + methods
      pairing.js          ← pairingSection(): QR pairing + account state + methods
      account.js          ← accountSection(): Account info state + methods
    zaplab.js             ← zaplab() factory: merges all sections, shared
                            state, init(), navigation helpers
```

Each file has a single responsibility and a predictable name.

---

## Section File Pattern

Every section file exports a **factory function** (not a plain object) so each
call to `zaplab()` gets fresh, isolated state — safe if Alpine ever calls it
more than once.

```js
// js/sections/groups.js
function groupsSection() {
  return {
    // ── state ──
    groups: {
      type: 'list',
      jid:  '',
      // ...
    },
    groupsPreviewTab:    localStorage.getItem('zaplab-groups-preview-tab') || 'curl',
    groupsPreviewCopied: false,

    // ── section init (called from main init()) ──
    initGroups() {
      this.$watch('groups.type', () => {
        this.groups.toast  = null;
        this.groups.result = null;
        if (this.groupsPreviewTab === 'response') this.groupsPreviewTab = 'curl';
      });
      this.$watch('groupsPreviewTab', val => {
        localStorage.setItem('zaplab-groups-preview-tab', val);
      });
    },

    // ── methods ──
    groupsEndpoint() { /* ... */ },
    groupsLabel()    { /* ... */ },
    // ...
  };
}
```

---

## Main Factory (`js/zaplab.js`)

```js
const pb = new PocketBase(window.location.origin);

function zaplab() {
  return Object.assign(
    {},
    utilsSection(),
    pairingSection(),
    accountSection(),
    eventsSection(),
    eventBrowserSection(),
    sendSection(),
    sendRawSection(),
    ctrlSection(),
    spoofSection(),
    contactsSection(),
    contactsMgmtSection(),
    groupsSection(),
    mediaSection(),
    simulationSection(),
    {
      // ── shared persistent state ──
      theme:           localStorage.getItem('zaplab-theme')          || 'dark',
      sidebarExpanded: localStorage.getItem('zaplab-sidebar')        !== 'collapsed',
      activeSection:   localStorage.getItem('zaplab-active-section') || 'events',
      apiToken:        localStorage.getItem('zaplab-api-token')      || '',

      // ── shared navigation ──
      toggleTheme()    { /* ... */ },
      toggleSidebar()  { /* ... */ },
      setSection(s)    { /* ... */ },

      // ── init: shared watches + delegates to section inits ──
      async init() {
        this.$watch('theme', val => { /* ... */ });
        this.$watch('sidebarExpanded', val => { /* ... */ });
        this.$watch('activeSection',   val => { /* ... */ });

        this.initPairing();
        this.initAccount();
        this.initEventBrowser();
        this.initSend();
        this.initSendRaw();
        this.initCtrl();
        this.initSpoof();
        this.initContacts();
        this.initContactsMgmt();
        this.initGroups();
        this.initMedia();
        this.initSimulation();

        this.eventsHeight = Math.max(120, Math.floor(window.innerHeight * 0.45));
        if (window.innerWidth < 768) this.sidebarExpanded = false;
        await this.loadInitialEvents();
        this.subscribeEvents();
        this.fetchAccount();
      },
    }
  );
}
```

---

## Loading Order in `index.html`

Non-deferred scripts execute before Alpine's deferred script — no module bundler
needed.

```html
<head>
  <!-- styles -->
  <link rel="stylesheet" href="css/zaplab.css" />

  <!-- Alpine (deferred — runs after all sync scripts below) -->
  <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.14.9/dist/cdn.min.js"></script>

  <!-- Section modules (sync, define global functions before Alpine initializes) -->
  <script src="js/utils.js"></script>
  <script src="js/sections/pairing.js"></script>
  <script src="js/sections/account.js"></script>
  <script src="js/sections/events.js"></script>
  <script src="js/sections/eventbrowser.js"></script>
  <script src="js/sections/send.js"></script>
  <script src="js/sections/sendraw.js"></script>
  <script src="js/sections/ctrl.js"></script>
  <script src="js/sections/spoof.js"></script>
  <script src="js/sections/contacts.js"></script>
  <script src="js/sections/contactsmgmt.js"></script>
  <script src="js/sections/groups.js"></script>
  <script src="js/sections/media.js"></script>
  <script src="js/sections/simulation.js"></script>
  <!-- Factory (must come after sections, before Alpine) -->
  <script src="js/zaplab.js"></script>
</head>
```

> Why this order works: browser executes `<script>` (no defer/async) synchronously
> while parsing. By the time `defer`-ed Alpine runs (after `DOMContentLoaded`), all
> section functions and `zaplab()` are already defined in the global scope.

---

## HTML Section Files

The section `<div x-show="...">` blocks remain in `index.html`. They are purely
declarative (Alpine directives reference method names, not implementations), so
splitting them into separate HTML files would require a build step or dynamic
fetch — not worth the complexity.

---

## What Does NOT Change

| Item | Status |
|------|--------|
| PocketBase route `/tools/{path...}` | Unchanged — still serves `pb_public/` |
| Alpine version (3.14.9) | Unchanged |
| All `x-data`, `x-model`, `x-show` directives | Unchanged |
| All method and state names | Unchanged |
| API backend | Unchanged |
| Tailwind CDN | Unchanged |
| PocketBase CDN | Unchanged |

---

## Section Files

| File | Factory Function | Section Name (`activeSection`) |
|------|-----------------|-------------------------------|
| `js/sections/pairing.js` | `pairingSection()` | `pairing` |
| `js/sections/account.js` | `accountSection()` | `account` |
| `js/sections/events.js` | `eventsSection()` | `events` (default) |
| `js/sections/eventbrowser.js` | `eventBrowserSection()` | `eventbrowser` |
| `js/sections/send.js` | `sendSection()` | `send` |
| `js/sections/sendraw.js` | `sendRawSection()` | `sendraw` |
| `js/sections/ctrl.js` | `ctrlSection()` | `ctrl` |
| `js/sections/spoof.js` | `spoofSection()` | `spoof` |
| `js/sections/contacts.js` | `contactsSection()` | `contacts` |
| `js/sections/contactsmgmt.js` | `contactsMgmtSection()` | `contactsmgmt` |
| `js/sections/groups.js` | `groupsSection()` | `groups` |
| `js/sections/media.js` | `mediaSection()` | `media` |
| `js/sections/simulation.js` | `simulationSection()` | `simulation` |

## What Does NOT Change

| Item | Status |
|------|--------|
| PocketBase route `/tools/{path...}` | Unchanged — still serves `pb_public/` |
| Alpine version (3.14.9) | Unchanged |
| All `x-data`, `x-model`, `x-show` directives | Unchanged |
| All method and state names | Unchanged |
| API backend | Unchanged |
| Tailwind CDN | Unchanged |
| PocketBase CDN | Unchanged |
