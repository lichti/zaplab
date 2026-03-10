# Contacts Management Spec

> Endpoints: `GET /contacts`, `POST /contacts/check`, `GET /contacts/{jid}`

---

## Overview

Contacts management provides read-only access to the WhatsApp device store's contact database and the ability to check whether phone numbers are registered on WhatsApp.

These operations are separate from sending contact messages (`/sendcontact`, `/sendcontacts`) — see `specs/CONTACTS_POLLS_SPEC.md`.

---

## Endpoints

### `GET /contacts`

Returns all contacts stored in the local whatsmeow device store. This reflects the contacts that have been synced to the device.

**Response 200:**
```json
{
  "contacts": [
    {
      "JID":          "5511999999999@s.whatsapp.net",
      "Found":        true,
      "FirstName":    "John",
      "FullName":     "John Doe",
      "PushName":     "Johnny",
      "BusinessName": ""
    }
  ],
  "count": 1
}
```

| Field          | Description                                      |
|----------------|--------------------------------------------------|
| `JID`          | Full WhatsApp JID                                |
| `Found`        | `true` if the contact exists in the store        |
| `FirstName`    | First name from the address book                 |
| `FullName`     | Full name from the address book                  |
| `PushName`     | Display name set by the contact on their device  |
| `BusinessName` | Verified business name (empty for personal)      |

**Response 503 — not connected:**
```json
{ "message": "..." }
```

---

### `POST /contacts/check`

Checks whether a list of phone numbers are registered on WhatsApp (i.e., have a WhatsApp account). Uses whatsmeow's `IsOnWhatsApp()`.

**Body:**
```json
{
  "phones": ["5511999999999", "5522888888888", "+5533777777777"]
}
```

| Field    | Type     | Required | Description                                             |
|----------|----------|----------|---------------------------------------------------------|
| `phones` | string[] | yes      | Phone numbers — digits only, or with `+` prefix        |

**Response 200:**
```json
{
  "results": [
    {
      "Query":        "5511999999999",
      "JID":          "5511999999999@s.whatsapp.net",
      "IsIn":         true,
      "VerifiedName": ""
    },
    {
      "Query":        "5522888888888",
      "JID":          "",
      "IsIn":         false,
      "VerifiedName": ""
    }
  ],
  "count": 2
}
```

| Field          | Description                                                     |
|----------------|-----------------------------------------------------------------|
| `Query`        | The original phone number as supplied                           |
| `JID`          | Full WhatsApp JID if registered, empty otherwise                |
| `IsIn`         | `true` if the number has a WhatsApp account                     |
| `VerifiedName` | Verified business name if the account is a verified business    |

**Response 400:**
```json
{ "message": "phones array is required" }
```

---

### `GET /contacts/{jid}`

Returns stored info for a specific contact by their JID. `{jid}` must be URL-encoded.

**Example:** `GET /contacts/5511999999999%40s.whatsapp.net`

**Response 200:**
```json
{
  "JID":          "5511999999999@s.whatsapp.net",
  "Found":        true,
  "FirstName":    "John",
  "FullName":     "John Doe",
  "PushName":     "Johnny",
  "BusinessName": ""
}
```

**Response 400 — invalid JID:**
```json
{ "message": "Invalid JID" }
```

**Response 500 — lookup error:**
```json
{ "message": "Error fetching contact info: ..." }
```

---

## Go Implementation

**File:** `internal/whatsapp/contacts.go` (or similar)

Functions called:
- `whatsapp.GetAllContacts()` → `[]store.Contact`
- `whatsapp.CheckOnWhatsApp(phones []string)` → `[]types.IsOnWhatsAppResponse`
- `whatsapp.GetContactInfo(jid types.JID)` → `store.Contact`

---

## Frontend

Section: **Contacts Management** (`activeSection === 'contactsmgmt'`)

File: `pb_public/js/sections/contactsmgmt.js` — `contactsMgmtSection()` factory

State prefix: `mgmt.*`

Supported actions and their behavior:

| Action | HTTP | Endpoint | Description |
|---|---|---|---|
| `list` | GET | `/contacts` | Load all contacts, display in table, filter, export CSV |
| `check` | POST | `/contacts/check` | Check phone numbers, display IsIn results |
| `info` | GET | `/contacts/{jid}` | Fetch and display a single contact card |

Additional UI features:
- **Contact picker**: loads from `/contacts` list, allows selecting a JID for `info` action
- **Filter**: client-side filter on the contacts table (name / JID)
- **Export CSV**: downloads the contacts table as `contacts.csv`

---

## Files Changed

| File | Change |
|---|---|
| `internal/api/api.go` | Added `getContacts`, `postContactsCheck`, `getContactInfo` handlers; registered 3 routes |
| `pb_public/js/sections/contactsmgmt.js` | New — `contactsMgmtSection()` factory |
| `pb_public/js/sections/contacts.js` | Removed management types (`list`, `check`, `info`) — now send-only |
| `pb_public/js/zaplab.js` | Added `contactsMgmtSection()` to `Object.assign`, `this.initContactsMgmt()` to `init()` |
| `pb_public/index.html` | Added Contacts Management nav button and section HTML; removed management fields from Contacts & Polls section |
| `specs/API_SPEC.md` | Documented 3 new endpoints |
| `specs/CONTACTS_POLLS_SPEC.md` | Added note that management actions moved to this section |
