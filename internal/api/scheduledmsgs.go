package api

import (
	"net/http"
	"time"

	"github.com/lichti/zaplab/internal/whatsapp"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// InitScheduledMessageWorker starts a goroutine that fires pending scheduled messages every minute.
func InitScheduledMessageWorker() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			fireScheduledMessages()
		}
	}()
}

func fireScheduledMessages() {
	if pb == nil || pb.DB() == nil {
		return
	}
	type smRow struct {
		ID          string `db:"id"`
		ChatJID     string `db:"chat_jid"`
		MessageText string `db:"message_text"`
		MsgType     string `db:"msg_type"`
		ReplyToID   string `db:"reply_to_msg_id"`
	}
	var rows []smRow
	if err := pb.DB().
		Select("id", "chat_jid", "message_text", "msg_type", "reply_to_msg_id").
		From("scheduled_messages").
		Where(dbx.HashExp{"status": "pending"}).
		AndWhere(dbx.NewExp("scheduled_at <= datetime('now')")).
		Limit(10). // max 10 per tick to avoid rate-limit
		All(&rows); err != nil || len(rows) == 0 {
		return
	}

	for _, row := range rows {
		var sendErr error
		jid, ok := whatsapp.ParseJID(row.ChatJID)
		if !ok {
			sendErr = &invalidJIDError{row.ChatJID}
		} else {
			// Only text messages supported in this version; media requires URL download.
			_, _, sendErr = whatsapp.SendConversationMessage(jid, row.MessageText, nil)
		}

		record, err := pb.FindRecordById("scheduled_messages", row.ID)
		if err != nil {
			continue
		}
		if sendErr != nil {
			record.Set("status", "failed")
			record.Set("error", sendErr.Error())
		} else {
			record.Set("status", "sent")
			record.Set("sent_at", time.Now().UTC().Format(time.RFC3339))
		}
		_ = pb.Save(record)
	}
}

type invalidJIDError struct{ jid string }

func (e *invalidJIDError) Error() string { return "invalid JID: " + e.jid }

// getScheduledMessages lists scheduled messages.
func getScheduledMessages(e *core.RequestEvent) error {
	type row struct {
		ID          string `db:"id"              json:"id"`
		ChatJID     string `db:"chat_jid"        json:"chat_jid"`
		MessageText string `db:"message_text"    json:"message_text"`
		MsgType     string `db:"msg_type"        json:"msg_type"`
		ScheduledAt string `db:"scheduled_at"    json:"scheduled_at"`
		SentAt      string `db:"sent_at"         json:"sent_at"`
		Status      string `db:"status"          json:"status"`
		Error       string `db:"error"           json:"error"`
		Created     string `db:"created"         json:"created"`
	}
	q := pb.DB().
		Select("id", "chat_jid", "message_text", "msg_type", "scheduled_at", "sent_at", "status", "error", "created").
		From("scheduled_messages").
		OrderBy("scheduled_at ASC").
		Limit(200)
	if s := e.Request.URL.Query().Get("status"); s != "" {
		q = q.AndWhere(dbx.HashExp{"status": s})
	}
	var rows []row
	if err := q.All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []row{}
	}
	return e.JSON(http.StatusOK, map[string]any{"scheduled_messages": rows, "total": len(rows)})
}

// postScheduledMessage creates a new scheduled message.
// Body: {"chat_jid":"...","message_text":"...","scheduled_at":"2026-04-20T15:00:00Z"}
func postScheduledMessage(e *core.RequestEvent) error {
	var req struct {
		ChatJID     string `json:"chat_jid"`
		MessageText string `json:"message_text"`
		ScheduledAt string `json:"scheduled_at"` // ISO-8601 UTC
		MsgType     string `json:"msg_type"`
		ReplyToID   string `json:"reply_to_msg_id"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("invalid request body", err)
	}
	if req.ChatJID == "" || req.MessageText == "" || req.ScheduledAt == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "chat_jid, message_text and scheduled_at are required"})
	}
	if _, err := time.Parse(time.RFC3339, req.ScheduledAt); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "scheduled_at must be ISO-8601 UTC (e.g. 2026-04-20T15:00:00Z)"})
	}
	if req.MsgType == "" {
		req.MsgType = "text"
	}
	col, err := pb.FindCollectionByNameOrId("scheduled_messages")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	r := core.NewRecord(col)
	r.Set("chat_jid", req.ChatJID)
	r.Set("message_text", req.MessageText)
	r.Set("msg_type", req.MsgType)
	r.Set("scheduled_at", req.ScheduledAt)
	r.Set("status", "pending")
	r.Set("reply_to_msg_id", req.ReplyToID)
	if err := pb.Save(r); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"id": r.Id, "status": "pending"})
}

// patchScheduledMessage updates or cancels a scheduled message.
func patchScheduledMessage(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	record, err := pb.FindRecordById("scheduled_messages", id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	if record.GetString("status") != "pending" {
		return e.JSON(http.StatusConflict, map[string]any{"error": "only pending messages can be modified"})
	}
	var req struct {
		MessageText string `json:"message_text"`
		ScheduledAt string `json:"scheduled_at"`
		Status      string `json:"status"` // "cancelled"
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("invalid request body", err)
	}
	if req.MessageText != "" {
		record.Set("message_text", req.MessageText)
	}
	if req.ScheduledAt != "" {
		if _, err := time.Parse(time.RFC3339, req.ScheduledAt); err != nil {
			return e.JSON(http.StatusBadRequest, map[string]any{"error": "scheduled_at must be ISO-8601 UTC"})
		}
		record.Set("scheduled_at", req.ScheduledAt)
	}
	if req.Status == "cancelled" {
		record.Set("status", "cancelled")
	}
	if err := pb.Save(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"id": record.Id, "status": record.GetString("status")})
}

// deleteScheduledMessage removes a pending scheduled message.
func deleteScheduledMessage(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	record, err := pb.FindRecordById("scheduled_messages", id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	if record.GetString("status") != "pending" && record.GetString("status") != "cancelled" {
		return e.JSON(http.StatusConflict, map[string]any{"error": "only pending or cancelled messages can be deleted"})
	}
	if err := pb.Delete(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"deleted": true})
}
