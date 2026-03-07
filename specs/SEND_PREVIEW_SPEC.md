# Send Message — JSON Preview & cURL Example

## Goal

Add two reactive preview panels to the right side of the Send Message form:
1. **JSON** — exact `*waE2E.Message` structure that Go builds before sending to WhatsApp
2. **cURL** — complete `curl` command to call the bot API, ready to copy

Both update in real time as the form is filled in.

---

## Architecture decision: no new endpoint

The fields generated at runtime by Go (`URL`, `DirectPath`, `MediaKey`, `FileEncSHA256`, `FileSHA256`, `FileLength`) are produced **only after the file is uploaded to WhatsApp servers** — therefore no endpoint can return them before the send.

The preview is built **entirely client-side**, replicating the `waE2E.Message` structure with:
- **Real values** for fields from the form (`conversation`, `caption`, `ptt`, `mimetype`)
- **Placeholder** `"<computed after upload>"` for upload-generated fields
- **Placeholder** `"<unix timestamp>"` for `mediaKeyTimestamp` (AudioMessage)

The `mimetype` is extracted from the selected `File` via `file.type` before the `FileReader`.

---

## Layout

The Send Message section uses two side-by-side panels:

```
┌─────────────────────────────────────────────────────────────────┐
│  Send Message                                                   │
├───────────────────────────┬─────────────────────────────────────┤
│                           │                                     │
│  Form (left)              │  Preview (right)                    │
│                           │                                     │
│  API Token                │  ┌── Request Preview ──────────┐   │
│  To                       │  │  [JSON]  [cURL]   endpoint  │   │
│  Type                     │  │                        Copy  │   │
│  Message / Caption        │  │  <reactive content>         │   │
│  File                     │  │                             │   │
│  PTT                      │  └─────────────────────────────┘   │
│  [Send]                   │                                     │
│                           │                                     │
└───────────────────────────┴─────────────────────────────────────┘
```

- `>= 1024px`: `flex-row`, half/half
- `< 1024px`: `flex-col`, preview below the form
- Preview panel: minimum height `300px`, internal scroll

---

## Content: JSON tab

Shows the `*waE2E.Message` structure (protobuf, serialized as camelCase JSON), as Go builds it in `internal/whatsapp/send.go`.

### `text` → `SendConversationMessage`
```json
{
  "conversation": "<send.message>"
}
```

### `image` → `SendImage`
```json
{
  "imageMessage": {
    "caption":       "<send.message>",
    "url":           "<computed after upload>",
    "directPath":    "<computed after upload>",
    "mediaKey":      "<computed after upload>",
    "mimetype":      "<file.type>",
    "fileEncSha256": "<computed after upload>",
    "fileSha256":    "<computed after upload>",
    "fileLength":    "<computed after upload>"
  }
}
```

### `video` → `SendVideo`
```json
{
  "videoMessage": {
    "caption":       "<send.message>",
    "url":           "<computed after upload>",
    "directPath":    "<computed after upload>",
    "mediaKey":      "<computed after upload>",
    "mimetype":      "<file.type>",
    "fileEncSha256": "<computed after upload>",
    "fileSha256":    "<computed after upload>",
    "fileLength":    "<computed after upload>"
  }
}
```

### `audio` → `SendAudio`
```json
{
  "audioMessage": {
    "url":               "<computed after upload>",
    "directPath":        "<computed after upload>",
    "mediaKey":          "<computed after upload>",
    "mimetype":          "<file.type>; codecs=opus",
    "fileEncSha256":     "<computed after upload>",
    "fileSha256":        "<computed after upload>",
    "fileLength":        "<computed after upload>",
    "ptt":               true,
    "mediaKeyTimestamp": "<unix timestamp>"
  }
}
```

### `document` → `SendDocument`
```json
{
  "documentMessage": {
    "caption":       "<send.message>",
    "url":           "<computed after upload>",
    "directPath":    "<computed after upload>",
    "mediaKey":      "<computed after upload>",
    "mimetype":      "<file.type>",
    "fileEncSha256": "<computed after upload>",
    "fileSha256":    "<computed after upload>",
    "fileLength":    "<computed after upload>"
  }
}
```

**Syntax highlight**: reuse the existing `highlight()` function in the project.

---

## Content: Response tab

Displays the full API response after a successful send, with syntax highlight.

- Only appears after the first successful send
- The preview automatically switches to this tab after send returns 200
- While there is no response, shows a placeholder message
- Green `●` badge on the tab indicates a response is available
- Reset when the message type is changed (returns to JSON tab)

**Displayed structure** (full API response):
```json
{
  "message": "Message sent",
  "whatsapp_message": { ... },
  "send_response": {
    "Timestamp": "...",
    "ID": "...",
    "ServerID": "...",
    "Sender": "...",
    "DebugTimings": { ... }
  }
}
```

**Alpine state:**
```js
send.result  // null | API response object (data from await res.json())
```

**Method:** `sendResultPreview()` — passes `send.result` to the existing `highlight()`.

**copyPreview():** copies `JSON.stringify(send.result, null, 2)` when on the Response tab.

---

## Content: cURL tab

Shows the `curl` command to call the **bot API** (not the WhatsApp proto).
The `-d` payload is the JSON body of the corresponding endpoint.

```
curl -X POST \
  <window.location.origin>/<endpoint> \
  -H "Content-Type: application/json" \
  -H "X-API-Token: <apiToken>" \
  -d '<json payload>'
```

### Endpoint mapping by type

| Type     | Endpoint      | Body fields                                           |
|----------|---------------|-------------------------------------------------------|
| text     | /sendmessage  | `{ to, message }`                                     |
| image    | /sendimage    | `{ to, message, image: "<base64: filename>" }`        |
| video    | /sendvideo    | `{ to, message, video: "<base64: filename>" }`        |
| audio    | /sendaudio    | `{ to, ptt, audio: "<base64: filename>" }`            |
| document | /senddocument | `{ to, message, document: "<base64: filename>" }`     |

- Real base64 is never shown — replaced by `"<base64: filename>"` or `"<no file selected>"`
- Empty token: display `<your-api-token>`
- Syntax highlight: flags (`-X`, `-H`, `-d`) in blue, URL in green, strings in orange

---

## Additional Alpine state

```js
send: {
  // ... existing fields ...
  mimeType: '',   // extracted from file.type in handleFile()
},
sendPreviewTab:    localStorage.getItem('zaplab-send-preview-tab') || 'json',
sendPreviewCopied: false,
```

---

## Alpine methods

```js
// Returns the waE2E.Message object with real values and placeholders
sendPayload() { ... },

// Returns the API request body object (used for cURL)
sendCurlPayload() { ... },

// Returns the correct endpoint for the current type
sendEndpoint() { ... },

// JSON highlight of the waE2E.Message (uses existing highlight())
sendJsonPreview() { ... },

// Formatted and highlighted cURL command
sendCurlPreview() { ... },

// Dispatches to JSON, cURL, or Response based on active tab
sendPreviewContent() { ... },

// Simple highlight for the cURL output
highlightCurl(str) { ... },

// Safe HTML escape
escapeHtml(str) { ... },

// Copies the active tab content to clipboard
copyPreview() { ... },
```

`sendPayload()` and `sendCurlPayload()` are pure functions that read form state.
Alpine re-executes automatically when any dependency changes — no `$watch` needed.

---

## handleFile — mimeType extraction

```js
handleFile(event) {
  const file = event.target.files[0];
  if (!file) return;
  this.send.fileName = `${file.name} (${(file.size / 1024).toFixed(1)} KB)`;
  this.send.mimeType = file.type || 'application/octet-stream';
  const reader = new FileReader();
  reader.onload = e => {
    this.send.fileData = e.target.result.split(',')[1];
  };
  reader.readAsDataURL(file);
},
```

Reset in `$watch('send.type')`: include `this.send.mimeType = ''`.

---

## copyPreview

- JSON tab: copies `JSON.stringify(sendPayload(), null, 2)` (waE2E.Message, no highlight)
- cURL tab: copies `sendCurlPreview()` as plain text

---

## What NOT to change

- Existing form (fields, validations, send logic) — unchanged
- `highlight()` function — reused without modification
- No other Go files are modified
- No new API endpoints are created
