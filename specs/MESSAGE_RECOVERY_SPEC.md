# Message Recovery Specification

## Overview
The Message Recovery feature allows users to monitor and recover content from messages that have been deleted (revoked) or edited by other participants in private chats or groups.

## Functional Requirements
1. **Detection:** The system must listen for `ProtocolMessage` events with types `REVOKE` or `MESSAGE_EDIT`.
2. **Recovery:**
   - Upon detection, the system searches the local `events` collection (SQLite/PocketBase) for the original message using its `msgID`.
   - The original content is extracted from the `raw` JSON field of the stored event.
3. **Notification:**
   - A notification is sent to the user's own WhatsApp JID (self-message).
   - **Deleted messages:** The notification includes the sender's name, the context (chat/group), and the original content.
   - **Edited messages:** The notification includes the sender's name, the context, the content **before** the edit, and the content **after** the edit.
4. **Configuration:**
   - Two independent flags control this feature: `recover_deletes` and `recover_edits`.
   - Settings are persisted in a `config.json` file in the data directory.
   - A UI toggle in the **Settings** section of the dashboard allows easy management.

## Technical Details

### Backend Configuration (`internal/config`)
- File: `config.json`
- Schema:
  ```json
  {
      "recover_edits": true,
      "recover_deletes": true
  }
  ```

### API Endpoints
- `GET /zaplab/api/config`: Returns the current general configuration.
- `PUT /zaplab/api/config`: Updates configuration flags.
  - Body: `{ "recover_edits": bool, "recover_deletes": bool }`

### Data Retrieval Logic
```sql
SELECT type, raw FROM events WHERE msgID = ? LIMIT 1;
```
The `raw` field is unmarshaled into an `events.Message` object to extract the text content using the `getMsg` helper.

### Notification Template
- **Delete:**
  `🚨 *Message deleted*`
  `*From:* {PushName}`
  `*Where:* {Location}`
  `*Original Content:* {Content}`
- **Edit:**
  `🚨 *Message edited*`
  `*From:* {PushName}`
  `*Where:* {Location}`
  `*Before:* {ContentBefore}`
  `*After:* {ContentAfter}`

## Implementation Notes
- A 2-second delay (`time.Sleep`) is used before querying the DB to ensure the original message has been fully persisted in high-concurrency scenarios.
- Only messages from others are processed (`!evt.Info.IsFromMe`).
