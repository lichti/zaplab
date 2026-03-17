# SCRIPTING_SPEC — Plugin System & Scripting Engine

## Overview

A JavaScript automation sandbox that lets users write, save, and run scripts directly from the browser to interact with the WhatsApp engine and ZapLab data. Implemented on branch `feature/plugin-scripting`.

Scripts are persisted in the PocketBase `scripts` collection and executed server-side inside an isolated goja VM with a configurable timeout. Last-run metadata is stored back after each execution.

---

## 1. PocketBase Collection — `scripts`

### Migration

`migrations/1742700000_create_scripts.go` — collection ID `sc9p4r2k7mxnb1q`.

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | TextField (required, unique) | Human-readable script name |
| `description` | TextField | Optional description |
| `code` | TextField | JavaScript source |
| `enabled` | BoolField | Whether the script is active |
| `timeout_secs` | NumberField | Execution timeout (default 10, max defined by caller) |
| `last_run_status` | TextField | `"success"` or `"error"` |
| `last_run_output` | TextField | Concatenated `console.log` output from last run |
| `last_run_duration_ms` | NumberField | Wall-clock execution time in milliseconds |
| `last_run_error` | TextField | Error message if last run failed |
| `created` / `updated` | AutodateField | Automatic timestamps |

Rules: all CRUD rules set to `""` (open — guarded by API-token middleware at the route level).

---

## 2. Backend — `internal/api/scripts.go`

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/zaplab/api/scripts` | List all scripts ordered by name |
| `POST` | `/zaplab/api/scripts` | Create a new script |
| `PATCH` | `/zaplab/api/scripts/{id}` | Update script fields (partial) |
| `DELETE` | `/zaplab/api/scripts/{id}` | Delete a script |
| `POST` | `/zaplab/api/scripts/{id}/run` | Execute a persisted script and store result |
| `POST` | `/zaplab/api/scripts/run` | Execute ad-hoc code (not persisted) |

All routes require authentication via `requireAuth()` middleware.

### `GET /zaplab/api/scripts`

Response:
```json
{
  "scripts": [
    {
      "id": "...",
      "name": "my-script",
      "description": "...",
      "code": "console.log('hello');",
      "enabled": true,
      "timeout_secs": 10,
      "last_run_status": "success",
      "last_run_output": "hello",
      "last_run_duration_ms": 3,
      "last_run_error": "",
      "created": "...",
      "updated": "..."
    }
  ],
  "total": 1
}
```

### `POST /zaplab/api/scripts`

Body: `{"name", "description", "code", "enabled", "timeout_secs"}`

Returns the created script record.

### `POST /zaplab/api/scripts/{id}/run` and `POST /zaplab/api/scripts/run`

`/run` (ad-hoc) body: `{"code", "timeout_secs"}`

Response:
```json
{
  "status": "success",
  "output": "hello\n1234",
  "duration_ms": 5,
  "error": ""
}
```

After executing a persisted script, `last_run_*` fields are updated in PocketBase.

---

## 3. Sandbox — `runScript(code string, timeout time.Duration)`

Runs inside an isolated `goja.Runtime` (no shared state between calls).

### Timeout

`time.AfterFunc(timeout, vm.Interrupt)` — the VM is interrupted after the configured duration with the message `"script timeout"`.

### Exposed APIs

#### `console`

| Function | Behaviour |
|----------|-----------|
| `console.log(...args)` | Appends space-joined string to output buffer |
| `console.error(...args)` | Alias for `console.log` |
| `console.warn(...args)` | Alias for `console.log` |

Output is returned as a newline-joined string in the response.

#### `wa`

| Function | Behaviour |
|----------|-----------|
| `wa.sendText(jid, text)` | Calls `whatsapp.SendConversationMessage(jid, text, nil)`; panics on invalid JID or send error |
| `wa.status()` | Returns `string(whatsapp.GetConnectionStatus())` |

#### `http`

| Function | Behaviour |
|----------|-----------|
| `http.get(url)` | `GET` with 15 s timeout; returns `{status: int, body: string}` (body capped at 64 KB) |
| `http.post(url, body)` | `POST` `application/json` with body string; same return shape |

#### `db`

| Function | Behaviour |
|----------|-----------|
| `db.query(sql)` | Executes arbitrary SQL on PocketBase's own SQLite via `pb.DB().NewQuery(sql)`; returns JS array of row objects; panics on error |

> **Security note**: `db.query` accepts any SQL including writes. Use only on trusted instances. This is intentional for a research tool.

#### `sleep`

```js
sleep(ms) // max 5000 ms per call
```

---

## 4. Frontend — `pb_public/js/sections/scripting.js`

Factory `scriptingSection()` merged via `Object.assign`.

### State

| Field | Default | Description |
|-------|---------|-------------|
| `scLoading` | false | List fetch in progress |
| `scError` | '' | Last load error |
| `scScripts` | [] | All persisted scripts |
| `scSelected` | null | Script open in the editor panel |
| `scRunning` | false | Persisted-script run in progress |
| `scRunOutput / scRunError / scRunStatus / scRunDurationMs` | — | Result of last persisted-script run |
| `scAdhocCode` | starter snippet | Ad-hoc editor content |
| `scAdhocRunning` | false | Ad-hoc run in progress |
| `scAdhocOutput / scAdhocError / scAdhocStatus / scAdhocDurationMs` | — | Ad-hoc result |
| `scNewName / scNewDesc / scNewCode / scNewTimeout / scShowNew` | — | New-script form fields |

### Key Methods

**`scLoad()`** — fetches `GET /zaplab/api/scripts`.

**`scCreate()`** — posts new script; prepends result to `scScripts`; clears form.

**`scSave(script)`** — patches the script record; updates the `scScripts` array in place.

**`scDelete(script)`** — confirms, then deletes; removes from `scScripts`.

**`scRun(script)`** — posts `/{id}/run`; updates `scRunOutput`, `scRunError`, `scRunStatus`, `scRunDurationMs`, and the inline `last_run_*` fields in the list/editor.

**`scRunAdhoc()`** — posts `POST /zaplab/api/scripts/run` with `scAdhocCode`.

### UI Layout

Three-column layout:
1. **Left sidebar** (fixed 224 px) — scrollable list of saved scripts; each row shows a status icon (✓/✗/—) and script name; clicking opens the editor panel and resets the run output display.
2. **Main panel** — two states:
   - **No selection**: shows the ad-hoc console (textarea + Run button + output panel).
   - **Script selected**: shows an inline editor with name/description/timeout/enabled inputs, Save / Run / Delete buttons, code textarea, and output panel below.
3. **Output panel** (bottom of main) — shows `"ERROR: ..."` if `scRunError` is set, else `scRunOutput`, else placeholder text; displays status badge and duration.

---

## API Routes Registered (`internal/api/api.go`)

```go
e.Router.GET("/zaplab/api/scripts",          getScripts).Bind(auth)
e.Router.POST("/zaplab/api/scripts",         postScript).Bind(auth)
e.Router.PATCH("/zaplab/api/scripts/{id}",   patchScript).Bind(auth)
e.Router.DELETE("/zaplab/api/scripts/{id}",  deleteScript).Bind(auth)
e.Router.POST("/zaplab/api/scripts/{id}/run", postScriptRun).Bind(auth)
e.Router.POST("/zaplab/api/scripts/run",     postScriptRunAdhoc).Bind(auth)
```

---

## Security Notes

- All endpoints are behind `requireAuth()` — API token required.
- `db.query` executes arbitrary SQL on PocketBase's SQLite. This is intentional for a research tool; do not expose to untrusted users.
- `wa.sendText` can send real WhatsApp messages. Scripts should only be run by the device owner.
- `http.get` / `http.post` make outbound HTTP requests from the server. Bodies are capped at 64 KB.
- `sleep` is capped at 5 s per call to prevent easy DoS.
- The VM timeout (`time.AfterFunc`) prevents infinite loops.
