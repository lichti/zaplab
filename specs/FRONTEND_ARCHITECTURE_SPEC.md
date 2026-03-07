# Frontend Split Proposal

## Motivation

`pb_public/index.html` reached **2 732 lines** across three distinct concerns:
- HTML structure (topbar, sidebar, sections) — 1 692 lines
- Custom CSS — 36 lines
- Alpine.js component — 1 004 lines

Adding Phase 5 would push the file past 3 000 lines. Inline scripts have no syntax
highlighting, no jump-to-definition, and no isolated testability.

---

## Target Structure

```
pb_public/
  index.html              ← HTML only (~400 lines after stripping CSS + JS)
  css/
    zaplab.css            ← extracted <style> block (~36 lines)
  js/
    utils.js              ← shared helpers: highlight, escapeHtml, highlightCurl,
                            fmtTime, typeClass, previewText  (~60 lines)
    sections/
      events.js           ← eventsSection(): state + Live Events methods  (~110 lines)
      send.js             ← sendSection(): state + Send Message methods   (~230 lines)
      sendraw.js          ← sendRawSection(): state + Send Raw methods     (~105 lines)
      ctrl.js             ← ctrlSection(): state + Message Control methods (~110 lines)
      contacts.js         ← contactsSection(): state + Contacts/Polls      (~140 lines)
      groups.js           ← groupsSection(): state + Groups methods        (~145 lines)
    zaplab.js             ← zaplab() factory: merges sections, shared
                            state, init(), navigation helpers              (~90 lines)
```

**Total lines across all files ≈ same as current.** Each file has a single
responsibility and a predictable name.

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
    eventsSection(),
    sendSection(),
    sendRawSection(),
    ctrlSection(),
    contactsSection(),
    groupsSection(),
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

        this.initSend();
        this.initSendRaw();
        this.initCtrl();
        this.initContacts();
        this.initGroups();

        this.eventsHeight = Math.max(120, Math.floor(window.innerHeight * 0.45));
        if (window.innerWidth < 768) this.sidebarExpanded = false;
        await this.loadInitialEvents();
        this.subscribeEvents();
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
  <script src="js/sections/events.js"></script>
  <script src="js/sections/send.js"></script>
  <script src="js/sections/sendraw.js"></script>
  <script src="js/sections/ctrl.js"></script>
  <script src="js/sections/contacts.js"></script>
  <script src="js/sections/groups.js"></script>
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

## Files Changed

| File | Change |
|------|--------|
| `pb_public/index.html` | Remove `<style>` block → `<link>`; remove `<script>` block → `<script src>` tags; HTML section divs unchanged |
| `pb_public/css/zaplab.css` | New — extracted CSS |
| `pb_public/js/utils.js` | New — shared helpers |
| `pb_public/js/sections/events.js` | New — events section |
| `pb_public/js/sections/send.js` | New — send section |
| `pb_public/js/sections/sendraw.js` | New — send raw section |
| `pb_public/js/sections/ctrl.js` | New — message control section |
| `pb_public/js/sections/contacts.js` | New — contacts & polls section |
| `pb_public/js/sections/groups.js` | New — groups section |
| `pb_public/js/zaplab.js` | New — factory + shared state + init |
| `README.md` | Update project structure section |
| `README.pt-BR.md` | Idem em português |

---

## Estimated Line Count per File (after split)

| File | Estimated Lines |
|------|----------------|
| `index.html` | ~1 700 (HTML only, no inline scripts) |
| `css/zaplab.css` | ~40 |
| `js/utils.js` | ~60 |
| `js/sections/events.js` | ~110 |
| `js/sections/send.js` | ~230 |
| `js/sections/sendraw.js` | ~105 |
| `js/sections/ctrl.js` | ~110 |
| `js/sections/contacts.js` | ~140 |
| `js/sections/groups.js` | ~145 |
| `js/zaplab.js` | ~90 |
| **Total** | **~2 730** |

Same total — but each file is focused, navigable, and independently editable.
