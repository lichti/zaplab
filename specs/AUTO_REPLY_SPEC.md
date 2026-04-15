# Auto-Reply Rules Engine — Technical Specification

## Overview

The Auto-Reply Rules Engine evaluates incoming WhatsApp messages against an ordered list of user-defined rules and automatically executes a configured action when a rule matches. Rules are stored in the `auto_reply_rules` PocketBase collection and evaluated in `priority` order (lowest number first) in a goroutine spawned per incoming message.

---

## Data Model

Collection: `auto_reply_rules`  
Migration: `migrations/1748600000_create_auto_reply_rules.go`

| Field | Type | Description |
|---|---|---|
| `name` | text (required) | Human-readable rule label |
| `enabled` | bool | Whether the rule participates in evaluation |
| `priority` | number | Evaluation order — lower = evaluated first |
| `stop_on_match` | bool | Stop processing further rules after this one matches |
| `cond_from` | text | `others` / `me` / `all` — sender direction filter |
| `cond_chat_jid` | text | Restrict to a specific chat JID (empty = any) |
| `cond_sender_jid` | text | Restrict to a specific sender JID (empty = any) |
| `cond_text_pattern` | text | Pattern to match against message text (empty = any) |
| `cond_text_match_type` | text | `prefix` / `contains` / `exact` / `regex` |
| `cond_case_sensitive` | bool | Whether pattern matching is case-sensitive |
| `cond_hour_from` | number | Start of allowed hour window (0–23; -1 = any) |
| `cond_hour_to` | number | End of allowed hour window (0–23; -1 = any; supports midnight-wrap) |
| `action_type` | text (required) | `reply` / `webhook` / `script` |
| `action_reply_text` | text | Reply message body (supports template vars) |
| `action_webhook_url` | text | URL for webhook POST |
| `action_script_id` | text | Script name / ID for script execution |
| `match_count` | number | Cumulative count of successful matches |
| `last_match_at` | text | ISO-8601 timestamp of last match |

---

## Evaluation Flow

```
events.go: handleAsync(*events.Message)
  └── go EvaluateAutoReplyRules(evt)           ← goroutine, avoids blocking event worker
        └── DB: SELECT ... FROM auto_reply_rules WHERE enabled=1 ORDER BY priority ASC
              ↓ for each rule:
              matchesRule(rule, text, chatJID, senderJID, isFromMe, hour)
              │   ├── cond_from check (me/others/all)
              │   ├── cond_chat_jid check (exact match or skip)
              │   ├── cond_sender_jid check (exact match or skip)
              │   ├── cond_hour_from/to window check (midnight-wrap aware)
              │   └── text pattern check (prefix/contains/exact/regex)
              ↓ if matches:
              executeRule(rule, evt, ...)
              │   ├── reply   → SendConversationMessage (quoted reply)
              │   ├── webhook → async http.Post (JSON payload)
              │   └── script  → HandleCmd(scriptID, args, evt)
              incrementMatchCount(rule.ID)      ← updates match_count + last_match_at
              if stop_on_match → break
```

---

## Condition Evaluation

### Sender direction (`cond_from`)

| Value | Behaviour |
|---|---|
| `others` (default) | Only messages from other people (`IsFromMe = false`) |
| `me` | Only messages sent by the bot account (`IsFromMe = true`) |
| `all` | All messages regardless of direction |

### Hour window

When both `cond_hour_from` and `cond_hour_to` are ≥ 0, the current server hour (0–23) must fall within the window. Midnight-wrap is supported:

- `cond_hour_from = 22`, `cond_hour_to = 6` → matches 22:00–23:59 and 00:00–06:00.
- Either field set to `-1` → window check is disabled.

### Text pattern

| Match type | Logic |
|---|---|
| `contains` (default) | `strings.Contains(subject, pattern)` |
| `prefix` | `strings.HasPrefix(subject, pattern)` |
| `exact` | `subject == pattern` |
| `regex` | `regexp.Compile(pattern).MatchString(text)` — uses original pattern (case-awareness via regex flags) |

When `cond_case_sensitive = false`, both subject and pattern are lowercased before comparison (except `regex`, which must use inline flags like `(?i)` if needed).

---

## Actions

### `reply`

Sends a quoted reply to the original message in the same chat using `SendConversationMessage` with `ReplyInfo` set.

Template variables expanded in `action_reply_text`:

| Variable | Replaced with |
|---|---|
| `{name}` | Sender's push name (falls back to JID) |
| `{sender}` | Full sender JID string |
| `{chat}` | Full chat JID string |
| `{text}` | Original message text |
| `{hour}` | Current server hour as `HH` (zero-padded) |

### `webhook`

Fires an async `http.Post` (goroutine) with JSON body:

```json
{
  "rule_id":    "...",
  "rule_name":  "...",
  "chat_jid":   "...",
  "sender_jid": "...",
  "text":       "...",
  "msg_id":     "...",
  "timestamp":  "2026-04-14T20:00:00Z"
}
```

Uses `action_webhook_url` as target. Failures are logged but do not block further rules.

### `script`

Calls `HandleCmd(action_script_id, []string{chatJID, senderJID, msgText}, evt)`. The script receives chat JID, sender JID, and message text as positional arguments. Output is logged at INFO level.

---

## REST API

Base path: `/zaplab/api/auto-reply-rules` — all endpoints require authentication (`🔒`).

| Method | Path | Description |
|---|---|---|
| `GET` | `/auto-reply-rules` | List all rules ordered by priority; `?enabled=true/false` |
| `POST` | `/auto-reply-rules` | Create a new rule |
| `PATCH` | `/auto-reply-rules/{id}` | Update any field of an existing rule |
| `DELETE` | `/auto-reply-rules/{id}` | Delete a rule |
| `POST` | `/auto-reply-rules/{id}/toggle` | Toggle `enabled` boolean |

---

## Frontend

Section: **Auto-Reply Rules** (sidebar icon: chat bubble with dots).

- **Rule list table**: name, priority, match pattern (badge), action type (badge), hit counter, enabled toggle switch, edit/delete buttons.
- **Rule builder form** (create / edit): opens inline above the table.
  - Identity fields: name, priority, enabled checkbox, stop-on-match checkbox.
  - Conditions panel: `cond_from` select, chat JID, sender JID, text pattern + match type select, case-sensitive checkbox, hour from/to inputs.
  - Action panel: action type select; conditionally renders reply textarea, webhook URL input, or script ID input.
- **Toggle switch** on each row calls `POST /{id}/toggle` and mutates the local `rule.enabled` without a full reload.
- Lazy-loads on first section visit via Alpine.js `x-effect`.

---

## Files

| File | Role |
|---|---|
| `migrations/1748600000_create_auto_reply_rules.go` | Collection + indexes |
| `internal/whatsapp/autoreply.go` | `EvaluateAutoReplyRules`, `matchesRule`, `executeRule`, `incrementMatchCount` |
| `internal/whatsapp/events.go` | Hook: `go EvaluateAutoReplyRules(evt)` in Message handler |
| `internal/api/autoreply.go` | CRUD handlers + toggle |
| `internal/api/api.go` | Route registration |
| `pb_public/js/sections/autoreply.js` | Alpine.js section factory |
| `pb_public/index.html` | Nav item + section HTML |
