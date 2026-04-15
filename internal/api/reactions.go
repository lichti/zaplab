package api

import (
	"net/http"
	"strconv"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// getReactions returns reactions from the message_reactions collection.
// Query params: message_id, chat_jid, sender_jid, limit (default 200, max 2000)
func getReactions(e *core.RequestEvent) error {
	limit := 200
	if v := e.Request.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 2000 {
			limit = n
		}
	}

	q := pb.DB().
		Select("id", "message_id", "chat_jid", "sender_jid", "emoji", "removed", "react_at", "created").
		From("message_reactions").
		OrderBy("created DESC").
		Limit(int64(limit))

	if v := e.Request.URL.Query().Get("message_id"); v != "" {
		q = q.AndWhere(dbx.HashExp{"message_id": v})
	}
	if v := e.Request.URL.Query().Get("chat_jid"); v != "" {
		q = q.AndWhere(dbx.HashExp{"chat_jid": v})
	}
	if v := e.Request.URL.Query().Get("sender_jid"); v != "" {
		q = q.AndWhere(dbx.HashExp{"sender_jid": v})
	}

	type row struct {
		ID        string `db:"id"         json:"id"`
		MessageID string `db:"message_id"  json:"message_id"`
		ChatJID   string `db:"chat_jid"    json:"chat_jid"`
		SenderJID string `db:"sender_jid"  json:"sender_jid"`
		Emoji     string `db:"emoji"       json:"emoji"`
		Removed   bool   `db:"removed"     json:"removed"`
		ReactAt   string `db:"react_at"    json:"react_at"`
		Created   string `db:"created"     json:"created"`
	}
	var rows []row
	if err := q.All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []row{}
	}
	return e.JSON(http.StatusOK, map[string]any{"reactions": rows, "total": len(rows)})
}

// getReactionStats returns emoji frequency and top reactors for a chat.
// Query param: chat_jid (required)
func getReactionStats(e *core.RequestEvent) error {
	chatJID := e.Request.URL.Query().Get("chat_jid")
	if chatJID == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "chat_jid is required"})
	}

	type emojiRow struct {
		Emoji string  `db:"emoji" json:"emoji"`
		Count float64 `db:"cnt"   json:"count"`
	}
	var emojis []emojiRow
	_ = pb.DB().NewQuery(
		"SELECT emoji, COUNT(*) as cnt FROM message_reactions WHERE chat_jid={:jid} AND removed=0 AND emoji!='' GROUP BY emoji ORDER BY cnt DESC LIMIT 20",
	).Bind(dbx.Params{"jid": chatJID}).All(&emojis)

	type senderRow struct {
		SenderJID string  `db:"sender_jid" json:"sender_jid"`
		Count     float64 `db:"cnt"        json:"count"`
	}
	var senders []senderRow
	_ = pb.DB().NewQuery(
		"SELECT sender_jid, COUNT(*) as cnt FROM message_reactions WHERE chat_jid={:jid} AND removed=0 GROUP BY sender_jid ORDER BY cnt DESC LIMIT 20",
	).Bind(dbx.Params{"jid": chatJID}).All(&senders)

	if emojis == nil {
		emojis = []emojiRow{}
	}
	if senders == nil {
		senders = []senderRow{}
	}
	return e.JSON(http.StatusOK, map[string]any{
		"chat_jid":    chatJID,
		"top_emojis":  emojis,
		"top_senders": senders,
	})
}
