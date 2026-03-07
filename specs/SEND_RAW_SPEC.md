# Send Raw Spec — ZapLab

## Goal

Allow sending any arbitrary `waE2E.Message` structure directly to WhatsApp, bypassing the typed send helpers.
This is the primary interface for **protocol study and experimentation**: send any message type that `WAWebProtobufsE2E.pb.go` defines, including types not yet covered by the typed endpoints.

---

## API Endpoint

### `POST /sendraw`

**Body:**
```json
{
  "to":      "5511999999999",
  "message": { ... }
}
```

| Field     | Type   | Required | Description                                                    |
|-----------|--------|----------|----------------------------------------------------------------|
| `to`      | string | yes      | Destination number or JID                                      |
| `message` | object | yes      | Protobuf-JSON encoded `waE2E.Message` (camelCase field names)  |

The `message` field is decoded server-side with `protojson.Unmarshal` into `*waE2E.Message` and sent as-is via `client.SendMessage`.

**Response 200:**
```json
{
  "message":          "Raw message sent",
  "whatsapp_message": { ... },
  "send_response":    { "Timestamp": "...", "ID": "...", "Sender": "...", ... }
}
```

**Response 400 — invalid JSON / unmarshal error:**
```json
{ "message": "invalid message JSON: ..." }
```

**Response 400 — invalid JID:**
```json
{ "message": "To field is not a valid" }
```

---

## Go implementation

### `internal/whatsapp/send.go` — `SendRaw`

```go
func SendRaw(to types.JID, msgJSON []byte) (*waE2E.Message, *whatsmeow.SendResponse, error) {
    var msg waE2E.Message
    if err := protojson.Unmarshal(msgJSON, &msg); err != nil {
        return &waE2E.Message{}, &whatsmeow.SendResponse{}, fmt.Errorf("invalid message JSON: %w", err)
    }
    return sendMessage(to, &msg)
}
```

### `internal/api/api.go` — `postSendRaw`

```go
func postSendRaw(e *core.RequestEvent) error {
    var req struct {
        To      string          `json:"to"`
        Message json.RawMessage `json:"message"`
    }
    // ... parse, validate JID, call SendRaw, return result
}
```

Registered as: `e.Router.POST("/sendraw", postSendRaw)`

---

## Frontend section: Send Raw

Located in `pb_public/index.html`, shown when `activeSection === 'sendraw'`.

### Layout

```
┌─────────────────────────────────────────────────────────────────┐
│  Send Raw                              waE2E.Message JSON       │
├───────────────────────────┬─────────────────────────────────────┤
│                           │                                     │
│  API Token                │  ┌── Preview ──────────────────┐   │
│  To                       │  │  [cURL]  [Response]    Copy  │   │
│  ┌─────────────────────┐  │  │  /sendraw               ...  │   │
│  │ JSON Editor         │  │  │  <reactive content>         │   │
│  │ (syntax highlight   │  │  │                             │   │
│  │  + validation)      │  │  └─────────────────────────────┘   │
│  └─────────────────────┘  │                                     │
│  Valid JSON ✓             │                                     │
│  [Format]    [Send Raw]   │                                     │
│                           │                                     │
└───────────────────────────┴─────────────────────────────────────┘
```

### JSON editor

Implemented as a **textarea + overlay pre** pattern (no external dependencies):

- `<pre>` is positioned `absolute inset-0`, `pointer-events-none`, renders syntax-highlighted HTML
- `<textarea>` overlays the pre with `color: transparent` and a visible caret
- On `scroll` of the textarea, the pre's `scrollTop`/`scrollLeft` are synced via `$refs.rawPre`
- **Tab key** inserts 2 spaces via `insertTab(event)` (prevents browser default focus change)
- Border color: green when JSON is valid, red when invalid
- **Format button** (visible only when valid): pretty-prints the JSON with `JSON.stringify(parsed, null, 2)`

### Validation

`rawJsonHighlight()` runs on every keystroke (Alpine reactive):
1. Calls `JSON.parse(raw.json)` to validate
2. If valid: sets `raw.valid = true`, applies inline syntax highlighting to the escaped raw string
3. If invalid: sets `raw.valid = false`, stores `raw.error = e.message`, returns plain escaped text (no color = visual cue)

The feedback line below the editor shows `"Valid JSON"` (green) or the parse error message (red).

The **Send Raw** button is disabled when `!raw.valid || !raw.to || !raw.json.trim()`.

### Preview panel

| Tab      | Content                                                            |
|----------|--------------------------------------------------------------------|
| cURL     | Formatted `curl` command with endpoint `/sendraw` and full body    |
| Response | Full API response after successful send (with syntax highlight)    |

After a successful send, the preview automatically switches to the Response tab.

cURL body format:
```json
{ "to": "<raw.to>", "message": <parsed raw.json> }
```

### Alpine state

```js
raw: {
  to:      '',                              // destination
  json:    '{\n  "conversation": "Hello!"\n}', // editor content
  loading: false,
  toast:   null,
  result:  null,                            // API response after send
  valid:   true,                            // JSON parse status
  error:   '',                              // parse error message
},
rawPreviewTab:    localStorage.getItem('zaplab-raw-preview-tab') || 'curl',
rawPreviewCopied: false,
```

### Alpine methods

| Method              | Description                                                        |
|---------------------|--------------------------------------------------------------------|
| `rawJsonHighlight()`| Validates + highlights the editor content; updates `raw.valid`     |
| `rawCurlPreview()`  | Builds the `curl` command string for `/sendraw`                    |
| `rawResultPreview()`| Renders the API response with syntax highlight                     |
| `rawPreviewContent()`| Dispatches to cURL or Response tab content                        |
| `insertTab(event)`  | Inserts 2 spaces at the caret position on Tab key                  |
| `copyRawPreview()`  | Copies active tab content to clipboard                             |
| `submitRaw()`       | POSTs `{ to, message }` to `/sendraw`, updates result/toast        |

---

## localStorage keys

| Key                        | Default  | Description              |
|----------------------------|----------|--------------------------|
| `zaplab-raw-preview-tab`   | `'curl'` | Active preview tab       |

---

## Example messages

### Plain text
```json
{ "conversation": "Hello from SendRaw!" }
```

### Extended text (italic)
```json
{
  "extendedTextMessage": {
    "text": "_italic text_",
    "canonicalUrl": "",
    "matchedText": ""
  }
}
```

### Location
```json
{
  "locationMessage": {
    "degreesLatitude": -23.5505,
    "degreesLongitude": -46.6333,
    "name": "São Paulo"
  }
}
```

### Contact
```json
{
  "contactMessage": {
    "displayName": "John Doe",
    "vcard": "BEGIN:VCARD\nVERSION:3.0\nFN:John Doe\nTEL:+5511999999999\nEND:VCARD"
  }
}
```

---

## What NOT to change

- The `sendMessage` internal function — `SendRaw` reuses it unchanged
- Existing typed send endpoints — not affected
- The JSON editor overlay pattern must preserve text alignment (no reformatting of the source)
