# Phase 3 — Contacts & Polls Spec

> Endpoints: `/sendcontact`, `/sendcontacts`, `/createpoll`, `/votepoll`

---

## New Endpoints

### `POST /sendcontact`

Sends a single vCard contact.

**Body:**
```json
{
  "to":           "5511999999999",
  "display_name": "John Doe",
  "vcard":        "BEGIN:VCARD\nVERSION:3.0\nFN:John Doe\nTEL;TYPE=CELL:+5511999999999\nEND:VCARD",
  "reply_to":     { "message_id": "...", "sender_jid": "...", "quoted_text": "..." }
}
```

**Go (`internal/whatsapp/send.go`):**
```go
func SendContact(to types.JID, displayName, vcard string, reply *ReplyInfo) (*waE2E.Message, *whatsmeow.SendResponse, error)
```

Uses `waE2E.ContactMessage{DisplayName, Vcard, ContextInfo}`.

---

### `POST /sendcontacts`

Sends multiple vCard contacts in a single message bubble.

**Body:**
```json
{
  "to":           "5511999999999",
  "display_name": "2 contacts",
  "contacts": [
    { "name": "Alice", "vcard": "BEGIN:VCARD\nVERSION:3.0\nFN:Alice\nTEL:+5511111111111\nEND:VCARD" },
    { "name": "Bob",   "vcard": "BEGIN:VCARD\nVERSION:3.0\nFN:Bob\nTEL:+5522222222222\nEND:VCARD" }
  ],
  "reply_to": { "message_id": "...", "sender_jid": "...", "quoted_text": "..." }
}
```

**Go (`internal/whatsapp/send.go`):**
```go
func SendContacts(to types.JID, displayName string, contacts []struct{ Name, Vcard string }, reply *ReplyInfo) (*waE2E.Message, *whatsmeow.SendResponse, error)
```

Uses `waE2E.ContactsArrayMessage{DisplayName, Contacts []*ContactMessage, ContextInfo}`.

---

### `POST /createpoll`

Creates a WhatsApp poll. The poll encryption key (`encKey`) is generated and managed internally by `client.BuildPollCreation()` — no manual crypto required.

**Body:**
```json
{
  "to":               "5511999999999",
  "question":         "Favourite colour?",
  "options":          ["Blue", "Green", "Red"],
  "selectable_count": 1
}
```

Allowed `selectable_count` values:
| Value | Meaning |
|-------|---------|
| `1`   | Single choice (default) |
| `2+`  | Voter can pick up to N options |
| `0`   | Unlimited (multiple choice) |

**Go (`internal/whatsapp/send.go`):**
```go
func CreatePoll(to types.JID, question string, options []string, selectableCount int) (*waE2E.Message, *whatsmeow.SendResponse, error) {
    msg := client.BuildPollCreation(question, options, selectableCount)
    return sendMessage(to, msg)
}
```

---

### `POST /votepoll`

Casts a vote on an existing poll. The `poll_message_id` + `poll_sender_jid` pair must match the original poll's `MessageInfo` exactly — WhatsApp's server validates the encrypted vote hash.

**Body:**
```json
{
  "to":               "5511999999999",
  "poll_message_id":  "ABCD1234EFGH5678",
  "poll_sender_jid":  "5511999999999@s.whatsapp.net",
  "selected_options": ["Blue"]
}
```

**Go (`internal/whatsapp/send.go`):**
```go
func VotePoll(chatJID, pollSenderJID types.JID, pollMessageID string, selectedOptions []string) (*waE2E.Message, *whatsmeow.SendResponse, error) {
    pollInfo := &types.MessageInfo{
        MessageSource: types.MessageSource{
            Chat:   chatJID,
            Sender: pollSenderJID,
        },
        ID: pollMessageID,
    }
    msg, err := client.BuildPollVote(context.Background(), pollInfo, selectedOptions)
    if err != nil {
        return &waE2E.Message{}, &whatsmeow.SendResponse{}, fmt.Errorf("failed to build poll vote: %w", err)
    }
    return sendMessage(chatJID, msg)
}
```

`BuildPollVote` uses the poll's `encKey` (stored in the whatsmeow device store by `BuildPollCreation`) to encrypt the selected option hashes before sending.

---

## Frontend changes

### New sidebar section: Contacts & Polls

- Sidebar nav button (user-group icon) → `setSection('contacts')`
- Section `activeSection === 'contacts'` — two-column layout (form left, preview right)
- Action selector: Contact / Contacts / Poll / Vote
- **Contact fields**: to, display_name, vcard (textarea)
- **Contacts fields**: to, display_name, dynamic list of {name, vcard} pairs with Add/Remove buttons
- **Poll fields**: to, question, dynamic options list (min 2, max 12), selectable_count select
- **Vote fields**: to, poll_message_id, poll_sender_jid, selected_options (textarea, one per line)
- cURL + Response preview panel with copy button

### New Alpine state fields

```js
contacts: {
  type:            'contact',
  to:              '',
  displayName:     '',
  vcard:           '',
  contactsList:    [{ name: '', vcard: '' }],
  question:        '',
  options:         ['', ''],
  selectableCount: '1',
  pollMessageId:   '',
  pollSenderJid:   '',
  selectedOptions: '',   // one option per line for votepoll
  loading:         false,
  toast:           null,
  result:          null,
},
contactsPreviewTab:    'curl',
contactsPreviewCopied: false,
```

### New Alpine methods

```js
addContact()            // push { name:'', vcard:'' } to contacts.contactsList
removeContact(i)        // splice at index (min 1 entry)
addPollOption()         // push '' to contacts.options (max 12)
removePollOption(i)     // splice at index (min 2 entries)
contactsEndpoint()      // returns /sendcontact | /sendcontacts | /createpoll | /votepoll
contactsLabel()         // button label for each type
contactsCurlPayload()   // builds request body for current type
contactsCurlPreview()   // formatted cURL command string
contactsResultPreview() // highlighted JSON response
contactsPreviewContent()// delegates to curl or result preview
copyContactsPreview()   // clipboard copy
submitContacts()        // fetch POST to contactsEndpoint
```

---

## Files changed

| File | Change |
|------|--------|
| `internal/whatsapp/send.go` | Added `SendContact`, `SendContacts`, `CreatePoll`, `VotePoll` |
| `internal/api/api.go` | Added `postSendContact`, `postSendContacts`, `postCreatePoll`, `postVotePoll`; registered 4 routes |
| `pb_public/index.html` | New sidebar nav button; new `contacts` section; `contacts` Alpine state; `contactsPreviewTab/Copied`; `$watch` handlers; all new methods |
| `specs/API_SPEC.md` | Documented 4 new endpoints |
| `README.md` | Added 4 new endpoints |
| `README.pt-BR.md` | Same in Portuguese |
