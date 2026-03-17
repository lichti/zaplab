package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// ── Conversation View ─────────────────────────────────────────────────────────
//
// GET /zaplab/api/conversation/chats?limit=50
//   Returns the list of known chats ordered by most recent message.
//
// GET /zaplab/api/conversation?chat=<jid>&limit=100&before=<RFC3339>
//   Returns messages for a specific chat as parsed bubbles.

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
		WHERE type = 'Message'
		  AND json_extract(raw, '$.Info.Chat') IS NOT NULL
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
		WHERE type = 'Message'
		  AND json_extract(raw, '$.Info.Chat') = {:chat}
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
		ID       string `json:"id"`
		MsgID    string `json:"msgID"`
		Chat     string `json:"chat"`
		Sender   string `json:"sender"`
		IsFromMe bool   `json:"is_from_me"`
		MsgType  string `json:"type"`
		Text     string `json:"text"`
		Caption  string `json:"caption"`
		FileURL  string `json:"file_url"`
		ThumbURL string `json:"thumb_url"`
		Created  string `json:"created"`
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
			out.MsgType, out.Text, out.Caption = detectMsgType(msg)
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

// detectMsgType returns (msgType, text, caption) from a parsed Message map.
func detectMsgType(msg map[string]any) (msgType, text, caption string) {
	if v, ok := msg["Conversation"].(string); ok && v != "" {
		return "text", v, ""
	}
	if ext, ok := msg["ExtendedTextMessage"].(map[string]any); ok {
		t, _ := ext["Text"].(string)
		return "text", t, ""
	}
	if img, ok := msg["ImageMessage"].(map[string]any); ok {
		cap, _ := img["Caption"].(string)
		return "image", "", cap
	}
	if vid, ok := msg["VideoMessage"].(map[string]any); ok {
		cap, _ := vid["Caption"].(string)
		return "video", "", cap
	}
	if _, ok := msg["AudioMessage"]; ok {
		return "audio", "", ""
	}
	if _, ok := msg["DocumentMessage"]; ok {
		doc, _ := msg["DocumentMessage"].(map[string]any)
		cap, _ := doc["Caption"].(string)
		return "document", "", cap
	}
	if _, ok := msg["StickerMessage"]; ok {
		return "sticker", "", ""
	}
	if loc, ok := msg["LocationMessage"].(map[string]any); ok {
		name, _ := loc["Name"].(string)
		return "location", name, ""
	}
	if _, ok := msg["ReactionMessage"]; ok {
		react, _ := msg["ReactionMessage"].(map[string]any)
		emoji, _ := react["Text"].(string)
		return "reaction", emoji, ""
	}
	return "unknown", "", ""
}
