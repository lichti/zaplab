package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lichti/zaplab/internal/whatsapp"
	"github.com/pocketbase/pocketbase/core"
)

// ── Conversation View ─────────────────────────────────────────────────────────
//
// GET /zaplab/api/conversation/chats?limit=50
//   Returns the list of known chats ordered by most recent message.
//
// GET /zaplab/api/conversation?chat=<jid>&limit=100&before=<RFC3339>
//   Returns messages for a specific chat as parsed bubbles.
//
// Event types that carry message content:
//   type='Message'               — text, reaction, protocolMessage (edit/delete)
//   type='Message.ImageMessage'  — images with downloaded file
//   type='Message.VideoMessage'  — videos
//   type='Message.AudioMessage'  — audio / voice notes
//   type='Message.DocumentMessage' — documents
//   type='Message.StickerMessage'  — stickers
//   type='Message.LocationMessage' — location pins

// contentFilter is appended to WHERE to exclude key-exchange and sync-only events.
// It keeps: all typed media variants (type != 'Message') plus meaningful Message subtypes.
const contentFilter = `AND (
    type != 'Message'
    OR json_extract(raw, '$.Message.conversation')           IS NOT NULL
    OR json_extract(raw, '$.Message.extendedTextMessage')    IS NOT NULL
    OR json_extract(raw, '$.Message.reactionMessage')        IS NOT NULL
    OR json_extract(raw, '$.Message.protocolMessage.type')   = 0
    OR json_extract(raw, '$.Message.protocolMessage.type')   = 14
    OR json_extract(raw, '$.Message.imageMessage')           IS NOT NULL
    OR json_extract(raw, '$.Message.videoMessage')           IS NOT NULL
    OR json_extract(raw, '$.Message.audioMessage')           IS NOT NULL
    OR json_extract(raw, '$.Message.documentMessage')        IS NOT NULL
    OR json_extract(raw, '$.Message.stickerMessage')         IS NOT NULL
    OR json_extract(raw, '$.Message.locationMessage')        IS NOT NULL
)`

func getConversationChats(e *core.RequestEvent) error {
	limit, _ := strconv.Atoi(e.Request.URL.Query().Get("limit"))
	if limit < 1 || limit > 500 {
		limit = 100
	}

	type chatRow struct {
		Chat     string `db:"chat"      json:"chat"`
		MsgCount int    `db:"msg_count" json:"msg_count"`
		LastMsg  string `db:"last_msg"  json:"last_msg"`
	}
	var rows []chatRow
	err := pb.DB().NewQuery(`
		SELECT json_extract(raw, '$.Info.Chat') AS chat,
		       COUNT(*)                          AS msg_count,
		       MAX(created)                      AS last_msg
		FROM events
		WHERE type LIKE 'Message%'
		  AND json_extract(raw, '$.Info.Chat') IS NOT NULL
		  ` + contentFilter + `
		GROUP BY chat
		ORDER BY last_msg DESC
		LIMIT {:limit}`).
		Bind(map[string]any{"limit": limit}).
		All(&rows)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []chatRow{}
	}
	return e.JSON(http.StatusOK, map[string]any{"chats": rows, "total": len(rows)})
}

func getConversation(e *core.RequestEvent) error {
	chat := e.Request.URL.Query().Get("chat")
	if chat == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "chat parameter is required"})
	}

	limit, _ := strconv.Atoi(e.Request.URL.Query().Get("limit"))
	if limit < 1 || limit > 200 {
		limit = 100
	}

	params := map[string]any{"chat": chat, "limit": limit + 1}
	beforeClause := ""
	if beforeStr := e.Request.URL.Query().Get("before"); beforeStr != "" {
		if t, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			beforeClause = "AND created < {:before}"
			params["before"] = t.UTC().Format("2006-01-02 15:04:05.000Z")
		}
	}

	type rawRow struct {
		ID      string `db:"id"`
		RawJSON string `db:"raw"`
		MsgID   string `db:"msgID"`
		File    string `db:"file"`
		Created string `db:"created"`
	}
	var rows []rawRow
	sqlStr := `SELECT id, raw, msgID, COALESCE(file, '') AS file, created
		FROM events
		WHERE type LIKE 'Message%'
		  AND json_extract(raw, '$.Info.Chat') = {:chat}
		  ` + contentFilter + `
		  ` + beforeClause + `
		ORDER BY created DESC
		LIMIT {:limit}`
	if err := pb.DB().NewQuery(sqlStr).Bind(params).All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	type msgOut struct {
		ID          string `json:"id"`
		MsgID       string `json:"msgID"`
		Chat        string `json:"chat"`
		Sender      string `json:"sender"`
		IsFromMe    bool   `json:"is_from_me"`
		MsgType     string `json:"type"`
		Text        string `json:"text"`
		Caption     string `json:"caption"`
		FileURL     string `json:"file_url"`
		ThumbURL    string `json:"thumb_url"`
		ReactTarget string `json:"react_target,omitempty"` // for reactions: msgID of the reacted-to message
		EditTarget  string `json:"edit_target,omitempty"`  // for edited_*/deleted: msgID of the original message
		Created     string `json:"created"`
	}

	messages := make([]msgOut, 0, len(rows))
	var nextBefore string
	for _, row := range rows {
		var raw map[string]any
		if err := json.Unmarshal([]byte(row.RawJSON), &raw); err != nil {
			continue
		}
		info, _ := raw["Info"].(map[string]any)
		msg, _ := raw["Message"].(map[string]any)

		out := msgOut{
			ID:      row.ID,
			MsgID:   row.MsgID,
			Created: row.Created,
		}
		if info != nil {
			out.Chat, _ = info["Chat"].(string)
			out.Sender, _ = info["Sender"].(string)
			out.IsFromMe, _ = info["IsFromMe"].(bool)
		}
		if msg != nil {
			var skip bool
			out.MsgType, out.Text, out.Caption, skip = detectMsgType(msg)
			if skip {
				continue
			}
			// For reactions, extract the target message ID.
			if out.MsgType == "reaction" {
				if react, ok := msg["reactionMessage"].(map[string]any); ok {
					if key, ok := react["key"].(map[string]any); ok {
						out.ReactTarget, _ = key["ID"].(string)
					}
				}
			}
			// For edit/delete protocol messages, extract the original message ID
			// from protocolMessage.key.ID so the frontend can annotate the original bubble.
			if out.MsgType == "deleted" || strings.HasPrefix(out.MsgType, "edited_") {
				if pm, ok := msg["protocolMessage"].(map[string]any); ok {
					if key, ok := pm["key"].(map[string]any); ok {
						out.EditTarget, _ = key["ID"].(string)
					}
				}
			}
		}
		if row.File != "" {
			out.FileURL = "/api/files/events/" + row.ID + "/" + row.File
			out.ThumbURL = out.FileURL + "?thumb=300x300"
		}
		messages = append(messages, out)
		nextBefore = row.Created
	}

	return e.JSON(http.StatusOK, map[string]any{
		"messages":    messages,
		"chat":        chat,
		"has_more":    hasMore,
		"next_before": nextBefore,
	})
}

// detectMsgType returns (msgType, text, caption, skip) from a parsed Message map.
// Proto-generated Go structs use camelCase json tags (e.g. "conversation", "imageMessage").
// skip=true means this event has no displayable content (key exchange, sync, etc.).
func detectMsgType(msg map[string]any) (msgType, text, caption string, skip bool) {
	// Plain text
	if v, ok := msg["conversation"].(string); ok && v != "" {
		return "text", v, "", false
	}
	// Rich text / links
	if ext, ok := msg["extendedTextMessage"].(map[string]any); ok {
		t, _ := ext["text"].(string)
		return "text", t, "", false
	}
	// Edited / deleted (protocolMessage)
	if pm, ok := msg["protocolMessage"].(map[string]any); ok {
		pmType, _ := pm["type"].(float64)
		switch int(pmType) {
		case 0: // REVOKE
			return "deleted", "[mensagem apagada]", "", false
		case 14: // MESSAGE_EDIT — editedMessage carries the new content
			if edited, ok := pm["editedMessage"].(map[string]any); ok {
				mt, t, c, _ := detectMsgType(edited)
				if mt != "unknown" {
					return "edited_" + mt, t, c, false
				}
			}
			return "edited_text", "(editado)", "", false
		}
		// Other protocol types (historySyncNotification, appStateSync, etc.) — skip
		return "", "", "", true
	}
	// Reaction
	if react, ok := msg["reactionMessage"].(map[string]any); ok {
		emoji, _ := react["text"].(string)
		return "reaction", emoji, "", false
	}
	// Image
	if img, ok := msg["imageMessage"].(map[string]any); ok {
		cap, _ := img["caption"].(string)
		return "image", "", cap, false
	}
	// Video
	if vid, ok := msg["videoMessage"].(map[string]any); ok {
		cap, _ := vid["caption"].(string)
		return "video", "", cap, false
	}
	// Audio / voice note
	if _, ok := msg["audioMessage"]; ok {
		return "audio", "", "", false
	}
	// Document
	if doc, ok := msg["documentMessage"].(map[string]any); ok {
		name, _ := doc["fileName"].(string)
		if name == "" {
			name, _ = doc["title"].(string)
		}
		return "document", "", name, false
	}
	// Sticker
	if _, ok := msg["stickerMessage"]; ok {
		return "sticker", "", "", false
	}
	// Location
	if loc, ok := msg["locationMessage"].(map[string]any); ok {
		name, _ := loc["name"].(string)
		return "location", name, "", false
	}
	// senderKeyDistributionMessage-only — skip
	return "unknown", "", "", true
}

// ── Name Resolution ────────────────────────────────────────────────────────────
//
// GET /zaplab/api/conversation/names
//   Returns a {jid: displayName} map for all known contacts and groups.
//   Sources:
//     - whatsmeow_contacts (waDB) — covers @s.whatsapp.net and @lid contacts
//     - whatsapp.GetJoinedGroups() — covers @g.us groups (live call, may fail)

func getConversationNames(e *core.RequestEvent) error {
	names := map[string]string{}

	// ── Contacts from whatsapp.db ──────────────────────────────────────────
	if waDB != nil {
		rows, err := waDB.Query(`
			SELECT their_jid,
			       COALESCE(NULLIF(full_name, ''), NULLIF(push_name, '')) AS name
			FROM whatsmeow_contacts
			WHERE full_name != '' OR push_name != ''`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var jid, name string
				if rows.Scan(&jid, &name) == nil && name != "" {
					names[jid] = name
				}
			}
		}
	}

	// ── Groups from whatsapp API (best-effort, fails gracefully if offline) ─
	groups, err := whatsapp.GetJoinedGroups()
	if err == nil {
		for _, g := range groups {
			if g.Name != "" {
				names[g.JID.String()] = g.Name
			}
		}
	}

	return e.JSON(http.StatusOK, map[string]any{"names": names})
}
