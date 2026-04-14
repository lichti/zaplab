package api

import (
	"net/http"
	"strconv"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// getMentions returns mention records.
// Query params: mentioned_jid, chat_jid, is_bot (true|false), limit (default 200, max 2000)
func getMentions(e *core.RequestEvent) error {
	limit := 200
	if v := e.Request.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 2000 {
			limit = n
		}
	}

	q := pb.DB().
		Select("id", "message_id", "chat_jid", "sender_jid", "mentioned_jid", "is_bot", "context_text", "created").
		From("mentions").
		OrderBy("created DESC").
		Limit(int64(limit))

	if v := e.Request.URL.Query().Get("mentioned_jid"); v != "" {
		q = q.AndWhere(dbx.HashExp{"mentioned_jid": v})
	}
	if v := e.Request.URL.Query().Get("chat_jid"); v != "" {
		q = q.AndWhere(dbx.HashExp{"chat_jid": v})
	}
	if v := e.Request.URL.Query().Get("is_bot"); v == "true" {
		q = q.AndWhere(dbx.HashExp{"is_bot": true})
	} else if v == "false" {
		q = q.AndWhere(dbx.HashExp{"is_bot": false})
	}

	type row struct {
		ID           string `db:"id"            json:"id"`
		MessageID    string `db:"message_id"    json:"message_id"`
		ChatJID      string `db:"chat_jid"      json:"chat_jid"`
		SenderJID    string `db:"sender_jid"    json:"sender_jid"`
		MentionedJID string `db:"mentioned_jid" json:"mentioned_jid"`
		IsBot        bool   `db:"is_bot"        json:"is_bot"`
		ContextText  string `db:"context_text"  json:"context_text"`
		Created      string `db:"created"       json:"created"`
	}
	var rows []row
	if err := q.All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []row{}
	}
	return e.JSON(http.StatusOK, map[string]any{"mentions": rows, "total": len(rows)})
}

// getMentionStats returns mention frequency by mentioned JID and by chat.
func getMentionStats(e *core.RequestEvent) error {
	type byJID struct {
		MentionedJID string  `db:"mentioned_jid" json:"mentioned_jid"`
		Count        float64 `db:"cnt"           json:"count"`
	}
	var byJIDs []byJID
	_ = pb.DB().NewQuery(
		"SELECT mentioned_jid, COUNT(*) as cnt FROM mentions GROUP BY mentioned_jid ORDER BY cnt DESC LIMIT 30",
	).All(&byJIDs)

	type byChat struct {
		ChatJID string  `db:"chat_jid" json:"chat_jid"`
		Count   float64 `db:"cnt"      json:"count"`
	}
	var byChats []byChat
	_ = pb.DB().NewQuery(
		"SELECT chat_jid, COUNT(*) as cnt FROM mentions WHERE is_bot=1 GROUP BY chat_jid ORDER BY cnt DESC LIMIT 20",
	).All(&byChats)

	if byJIDs == nil {
		byJIDs = []byJID{}
	}
	if byChats == nil {
		byChats = []byChat{}
	}
	return e.JSON(http.StatusOK, map[string]any{
		"top_mentioned": byJIDs,
		"bot_by_chat":   byChats,
	})
}
