# Phase 4 — Group Management Spec

> Endpoints: `GET /groups`, `GET /groups/{jid}`, `POST /groups`, `POST /groups/{jid}/participants`, `PATCH /groups/{jid}`, `POST /groups/{jid}/leave`, `GET /groups/{jid}/invitelink`, `POST /groups/join`

---

## Endpoints

### `GET /groups`

Returns all groups the bot is currently a member of.

**Response 200:**
```json
{
  "groups": [
    {
      "JID": "123456789-000@g.us",
      "Name": "Group Name",
      "Topic": "Group description",
      "Participants": [...],
      ...
    }
  ]
}
```

**Go (`internal/whatsapp/groups.go`):**
```go
func GetJoinedGroups() ([]*types.GroupInfo, error)
```

---

### `GET /groups/{jid}`

Returns detailed info for a single group. `{jid}` must be URL-encoded (e.g. `123456789-000%40g.us`).

**Response 200:** `*types.GroupInfo` JSON

**Go:**
```go
func GetGroupInfo(jid types.JID) (*types.GroupInfo, error)
```

---

### `POST /groups`

Creates a new WhatsApp group.

**Body:**
```json
{
  "name": "My Group",
  "participants": ["5511999999999", "5511888888888"]
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Group name (max 25 characters) |
| `participants` | array | yes | Phone numbers or JIDs (bot is added automatically) |

**Response 200:**
```json
{ "message": "Group created", "group": { ... } }
```

**Go:**
```go
func CreateGroup(name string, participants []types.JID) (*types.GroupInfo, error)
```

Uses `whatsmeow.ReqCreateGroup{Name, Participants}`.

---

### `POST /groups/{jid}/participants`

Adds, removes, promotes, or demotes group participants.

**Body:**
```json
{
  "action": "add",
  "participants": ["5511999999999"]
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `action` | string | yes | `"add"` \| `"remove"` \| `"promote"` \| `"demote"` |
| `participants` | array | yes | Phone numbers or JIDs |

**Response 200:**
```json
{ "message": "Participants updated", "results": [...] }
```

**Go:**
```go
func UpdateGroupParticipants(jid types.JID, participants []types.JID, action string) ([]types.GroupParticipant, error)
```

---

### `PATCH /groups/{jid}`

Updates one or more group settings in a single call. Only include the fields you want to change.

**Body:**
```json
{
  "name":     "New Name",
  "topic":    "New description",
  "announce": true,
  "locked":   false
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | no | New group name (max 25 chars) |
| `topic` | string | no | New group description |
| `announce` | bool | no | `true` = only admins can send |
| `locked` | bool | no | `true` = only admins can edit group info |

At least one field is required.

**Response 200:**
```json
{ "message": "Group updated" }
```

**Go:**
```go
func SetGroupName(jid types.JID, name string) error
func SetGroupTopic(jid types.JID, topic string) error
func SetGroupAnnounce(jid types.JID, announce bool) error
func SetGroupLocked(jid types.JID, locked bool) error
```

---

### `POST /groups/{jid}/leave`

Makes the bot leave the specified group.

**Response 200:**
```json
{ "message": "Left group" }
```

**Go:**
```go
func LeaveGroup(jid types.JID) error
```

---

### `GET /groups/{jid}/invitelink`

Returns the group invite link. Add `?reset=true` to revoke the current link and generate a new one.

**Query params:**
| Param | Default | Description |
|---|---|---|
| `reset` | `false` | If `"true"`, resets the link (revokes old one) |

**Response 200:**
```json
{ "link": "https://chat.whatsapp.com/AbCdEf123456" }
```

**Go:**
```go
func GetGroupInviteLink(jid types.JID, reset bool) (string, error)
```

---

### `POST /groups/join`

Joins a group using an invite link or link code.

**Body:**
```json
{ "link": "https://chat.whatsapp.com/AbCdEf123456" }
```

The full URL or just the code portion (`AbCdEf123456`) are both accepted.

**Response 200:**
```json
{ "message": "Joined group", "jid": "123456789-000@g.us" }
```

**Go:**
```go
func JoinGroupWithLink(code string) (types.JID, error)
```

---

## Frontend changes

### New sidebar section: Groups

- Sidebar nav button (group/community icon) → `setSection('groups')`
- Section `activeSection === 'groups'` — two-column layout
- Action selector: list / info / create / participants / settings / leave / invitelink / join
- **List**: button only (GET, no body)
- **Info**: group JID input
- **Create**: name (max 25) + participants textarea (one per line)
- **Participants**: group JID + action select (add/remove/promote/demote) + participants textarea
- **Settings**: group JID + optional name/topic inputs + announce/locked checkboxes
- **Leave**: group JID input
- **Invite Link**: group JID + reset checkbox
- **Join**: invite link/code input
- cURL + Response preview panel

### New Alpine state fields

```js
groups: {
  type:              'list',
  jid:               '',
  name:              '',
  participantsList:  '',    // textarea: one phone/JID per line
  participantAction: 'add',
  newName:           '',
  newTopic:          '',
  setAnnounce:       false,
  announce:          false,
  setLocked:         false,
  locked:            false,
  resetInviteLink:   false,
  inviteLink:        '',
  loading:           false,
  toast:             null,
  result:            null,
},
groupsPreviewTab:    'curl',
groupsPreviewCopied: false,
```

### New Alpine methods

```js
groupsEndpoint()       // builds the correct URL including path param and query string
groupsMethod()         // returns GET / POST / PATCH
groupsLabel()          // button label for each action
groupsCurlPayload()    // builds request body (null for GET endpoints)
groupsCurlPreview()    // formatted cURL command string
groupsResultPreview()  // highlighted JSON response
groupsPreviewContent() // delegates to curl or result preview
copyGroupsPreview()    // clipboard copy
submitGroups()         // fetch with dynamic method, endpoint, body
```

---

## Files changed

| File | Change |
|------|--------|
| `internal/whatsapp/groups.go` | New file — `GetJoinedGroups`, `GetGroupInfo`, `CreateGroup`, `UpdateGroupParticipants`, `SetGroupName`, `SetGroupTopic`, `SetGroupAnnounce`, `SetGroupLocked`, `LeaveGroup`, `GetGroupInviteLink`, `JoinGroupWithLink`; `JID` type alias |
| `internal/api/api.go` | Added `getGroups`, `getGroupInfo`, `postCreateGroup`, `postGroupParticipants`, `patchGroup`, `postLeaveGroup`, `getGroupInviteLink`, `postJoinGroup`; registered 8 routes |
| `pb_public/index.html` | New sidebar nav button; new `groups` section; `groups` Alpine state; watches; all new methods |
| `specs/API_SPEC.md` | Documented 8 new endpoints |
| `README.md` | Added group management section |
| `README.pt-BR.md` | Same in Portuguese |
