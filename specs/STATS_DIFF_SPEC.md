# STATS_DIFF_SPEC — Advanced Stats & Heatmap + Enhanced Message Diff

## Overview

Two complementary features shipped together on branch `feature/diff-stats-heatmap`:

1. **Advanced Stats & Heatmap** — aggregate analytics over the `events` table with a GitHub-style activity heatmap, daily sparkline, type distribution bars, and summary KPI cards.
2. **Enhanced Message Diff** — upgraded Edit Diff in Message History: char/word tokenisation toggle, inline/side-by-side layout, diff stats bar, and full edit chain timeline.

---

## 1. Advanced Stats & Heatmap

### Backend — `internal/api/stats.go`

Five read-only endpoints, all protected by `auth` middleware, all using raw SQL via `pb.DB().NewQuery()`.

#### `GET /zaplab/api/stats/summary`

Returns aggregate message statistics across multiple windows.

```json
{
  "total":      12345,
  "last_24h":   42,
  "last_7d":    310,
  "last_30d":   1200,
  "last_event": "2025-03-15 21:04:11",
  "edited":     88,
  "deleted":    17
}
```

- `total` / `last_24h` / `last_7d` / `last_30d`: COUNT(*) on `events WHERE type LIKE '%Message%'`
- `edited`: rows in `events WHERE type='Message'` with `raw LIKE '%"Edit":"1"%'`
- `deleted`: rows with `Edit` = `"7"` or `"8"`

#### `GET /zaplab/api/stats/heatmap?period=30`

Returns cell counts grouped by `(day-of-week, hour)`.

Query params:
- `period` int — days to look back (default 30, max 365; 0 = all time)

Response:
```json
{
  "cells": [{"dow":"1","hour":"09","count":14}, ...],
  "period": 30
}
```

SQL: `strftime('%w', datetime(created))` for DOW (0=Sun…6=Sat), `strftime('%H', ...)` for hour.

#### `GET /zaplab/api/stats/daily?days=30`

Returns per-day message counts for the last N days.

Query params:
- `days` int — days to look back (default 30, max 365; min 1)

Response:
```json
{
  "days": [{"day":"2025-02-14","count":23}, ...],
  "period": 30
}
```

SQL: `strftime('%Y-%m-%d', datetime(created)) AS day` grouped and ordered ascending.

#### `GET /zaplab/api/stats/types?period=30&limit=20`

Returns event counts grouped by event type, ordered by count descending.

Query params:
- `period` int — days to look back (default 30, max 365; 0 = all time)
- `limit` int — max types (default 20, max 50)

Response:
```json
{
  "types": [{"type":"Message","count":900}, {"type":"MessageEdit","count":88}, ...],
  "period": 30
}
```

#### `GET /zaplab/api/stats/editchain?msgid=<id>`

Returns all event rows sharing the same `msgID` (original + edits + deletes), ordered chronologically.

Query params:
- `msgid` string — required; matched against `events.msgID`

Response:
```json
{
  "chain": [
    {"id":"abc","type":"Message","msgID":"3EB0...","raw":"...","created":"2025-03-01 10:00:00"},
    {"id":"def","type":"MessageEdit","msgID":"3EB0...","raw":"...","created":"2025-03-01 10:05:00"}
  ],
  "msgid": "3EB0...",
  "total": 2
}
```

User-supplied `msgid` is sanitised via `sanitizeSQL()` (doubles single quotes) before string interpolation. Max 50 rows.

---

### Frontend — `pb_public/js/sections/stats.js`

Factory function `statsSection()` merged via `Object.assign` in `zaplab()`.

#### State

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `stLoading` | bool | false | Loading indicator |
| `stError` | string | '' | Error message |
| `stPeriod` | int | 30 | Active period (days) |
| `stPeriodOpts` | array | [7,30,90,365,0] | Period selector options |
| `stSummary` | object | {} | Summary KPI data |
| `stHeatCells` | array | [] | Heatmap cells [{dow,hour,count}] |
| `stDaily` | array | [] | Daily counts [{day,count}] |
| `stTypes` | array | [] | Type distribution [{type,count}] |

#### Key Methods

**`stLoad()`** — parallel fetch of all 4 endpoints (`summary`, `heatmap`, `daily`, `types`) with the current `stPeriod`.

**`stHeatCount(dow, hour)`** — finds cell matching string `dow` and zero-padded `hour`; returns 0 if not found.

**`stHeatMax()`** — max count across all cells for normalization.

**`stCellStyle(count)`** — always returns inline style string setting `--heat-light` and `--heat-dark` CSS custom properties (including for count=0; previously returned `''` for zero which left the CSS variables undefined and made empty cells transparent). Five intensity levels:

| Level | Condition | Light bg | Dark bg |
|-------|-----------|----------|---------|
| 0 | count = 0 | `#ebedf0` | `#161b22` |
| 1 | ratio ≤ 0.25 | `#9be9a8` | `#0e4429` |
| 2 | ratio ≤ 0.50 | `#40c463` | `#006d32` |
| 3 | ratio ≤ 0.75 | `#30a14e` | `#26a641` |
| 4 | ratio > 0.75 | `#216e39` | `#39d353` |

**`stSparkSVG()`** — generates SVG string (`viewBox="0 0 800 80"`) with one `<rect>` per day, height proportional to count, 1px gaps between bars. Rendered via `x-html`.

**`stTypeBarWidth(count)`** — `Math.max(2, Math.round((count / max) * 100))` capped at 100.

**`stTypeColor(type)`** — returns Tailwind `bg-*` class based on type name keywords.

**`stPeakCell()` / `stPeakLabel()`** — find the cell with highest count and format as `"Wed 14:00 (42 msgs)"`.

---

### UI Layout

- **Period selector**: row of buttons (7d / 30d / 90d / 365d / All), triggers `stLoad()`
- **Summary cards**: 2×4 grid — Total, Last 24h, Last 7d, Last 30d, Last Event, Edited, Deleted, Peak Hour
- **Activity Heatmap**: CSS grid `grid-template-columns: 2.5rem repeat(24, 1fr)` (label column + 24 hour columns); outer `x-for` over `stDayLabels` (7 days), each iteration wrapped in a `<div style="display:contents">` so the day label and 24 cell divs are direct grid children; inner `x-for` over `stHourLabels` (24 hours); each cell calls `stHeatCount(d, h)` inline in `:style="stCellStyle(stHeatCount(d, h))"` so Alpine tracks `stHeatCells` as a reactive dependency per cell and re-renders automatically on period change.

> **Note**: Alpine.js `x-for` requires a single root element per iteration. The heatmap uses `<div style="display:contents">` as the single root for each day row, which causes the day-label div and 24 cell divs to participate directly in the CSS grid without introducing an extra wrapper element in the layout.

- **Daily Sparkline**: `div` with `x-html="stSparkSVG()"`
- **Type Distribution**: list of bars, each `div` with inline width `stTypeBarWidth(count)%` and color class `stTypeColor(type)`

---

### CSS — `pb_public/css/zaplab.css`

```css
.heat-cell { background: var(--heat-light, transparent); }
.dark .heat-cell { background: var(--heat-dark, transparent); }
```

Alpine's `:style` binding sets `--heat-light` and `--heat-dark` per cell; the CSS class reads them. This avoids generating per-cell inline `background` properties and keeps dark mode switching trivial.

---

## 2. Enhanced Message Diff

### Frontend Changes — `pb_public/js/sections/msghistory.js`

#### New State Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `mhDiffMode` | string | `'inline'` | `'inline'` or `'sidebyside'` |
| `mhCharLevel` | bool | false | Token granularity: char vs word |
| `mhShowChain` | bool | false | Edit chain timeline visibility |
| `mhChain` | array | [] | Chain entries from `/stats/editchain` |
| `mhChainLoading` | bool | false | Chain fetch indicator |

#### Tokenisation

`_mhTokenize(text)` branches on `mhCharLevel`:
- **Word mode** (default): splits on `\b` boundary regex, same as original implementation
- **Char mode**: `Array.from(text)` — Unicode-safe character array

#### Diff Rendering

**`mhDiffHtml(item)`** — inline diff; returns `"(identical)"` message when LCS covers all tokens.

**`mhDiffSideA(item)`** — left panel (original); filters out `'ins'` operations from diff output.

**`mhDiffSideB(item)`** — right panel (new text); filters out `'del'` operations.

**`mhDiffStats(item)`** — returns `{added, removed, similarity}`:
- `added`: count of `'ins'` ops
- `removed`: count of `'del'` ops
- `similarity`: `Math.round((lcsLen / Math.max(origLen, newLen)) * 100)` — percentage of tokens in common

#### Edit Chain Timeline

**`mhLoadChain()`** — fetches `/zaplab/api/stats/editchain?msgid=<selected.msgID>`; populates `mhChain`.

**`mhToggleChain()`** — toggles `mhShowChain`; triggers `mhLoadChain()` on first open.

**`mhChainKind(entry)`** — classifies entry by inspecting `raw.Info.Edit` value:
- `"1"` → `'edit'`
- `"7"` or `"8"` → `'delete'`
- absent → `'original'`

**`mhChainKindClass(kind)`** — returns Tailwind border/text class set per kind.

**`mhChainContent(entry)`** — extracts text content from `raw.Message.Conversation` or `raw.Message.ExtendedTextMessage.Text`; falls back to `[no text]`.

---

### UI Layout (Diff Toolbar + Panels)

```
┌─────────────────────────────────────────────┐
│ [Inline] [Side-by-Side]  [Word] [Char]  [Edit Chain ▶] │
├──────────────────────────────────────────────┤
│ Diff Stats Bar: +N  -N  ~N%                  │
├──────────────────────────────────────────────┤
│ Inline Panel  (x-show="mhDiffMode==='inline'")        │
│   <span class="diff-del">…</span> <span class="diff-ins">…</span> │
├──────────────────────────────────────────────┤
│ Side-by-Side (x-show="mhDiffMode==='sidebyside'")     │
│ [Before (red tint)] │ [After (green tint)]            │
├──────────────────────────────────────────────┤
│ Edit Chain Timeline (x-show="mhShowChain")            │
│ ● original  ── ● edit  ── ● edit  ── ● delete         │
└─────────────────────────────────────────────┘
```

The diff stats bar uses `mhDiffStats(mhSelected)` and renders `+added` in green, `-removed` in red, and `~similarity%` in gray.

---

## API Routes Registered (`internal/api/api.go`)

```go
e.Router.GET("/zaplab/api/stats/heatmap",   getStatsHeatmap).Bind(auth)
e.Router.GET("/zaplab/api/stats/daily",     getStatsDaily).Bind(auth)
e.Router.GET("/zaplab/api/stats/types",     getStatsTypes).Bind(auth)
e.Router.GET("/zaplab/api/stats/summary",   getStatsSummary).Bind(auth)
e.Router.GET("/zaplab/api/stats/editchain", getStatsEditChain).Bind(auth)
```

---

## Security Notes

- All stat endpoints are read-only SQL queries against the `events` table.
- `getStatsEditChain` interpolates the `msgid` query parameter directly into SQL; `sanitizeSQL()` escapes single quotes (`'` → `''`) before insertion.
- No user-controlled input affects the period/days/limit paths — they are parsed via `strconv.Atoi` and clamped to safe ranges before being formatted into the SQL string via `%d`.
