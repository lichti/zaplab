package api

import (
	"net/http"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// getAutoReplyRules lists all auto-reply rules ordered by priority.
func getAutoReplyRules(e *core.RequestEvent) error {
	type row struct {
		ID               string  `db:"id"                    json:"id"`
		Name             string  `db:"name"                  json:"name"`
		Enabled          bool    `db:"enabled"               json:"enabled"`
		Priority         float64 `db:"priority"              json:"priority"`
		StopOnMatch      bool    `db:"stop_on_match"         json:"stop_on_match"`
		CondFrom         string  `db:"cond_from"             json:"cond_from"`
		CondChatJID      string  `db:"cond_chat_jid"         json:"cond_chat_jid"`
		CondSenderJID    string  `db:"cond_sender_jid"       json:"cond_sender_jid"`
		CondTextPattern  string  `db:"cond_text_pattern"     json:"cond_text_pattern"`
		CondMatchType    string  `db:"cond_text_match_type"  json:"cond_text_match_type"`
		CondCaseSens     bool    `db:"cond_case_sensitive"   json:"cond_case_sensitive"`
		CondHourFrom     float64 `db:"cond_hour_from"        json:"cond_hour_from"`
		CondHourTo       float64 `db:"cond_hour_to"          json:"cond_hour_to"`
		ActionType       string  `db:"action_type"           json:"action_type"`
		ActionReplyText  string  `db:"action_reply_text"     json:"action_reply_text"`
		ActionWebhookURL string  `db:"action_webhook_url"    json:"action_webhook_url"`
		ActionScriptID   string  `db:"action_script_id"      json:"action_script_id"`
		MatchCount       float64 `db:"match_count"           json:"match_count"`
		LastMatchAt      string  `db:"last_match_at"         json:"last_match_at"`
		Created          string  `db:"created"               json:"created"`
	}
	q := pb.DB().
		Select("id", "name", "enabled", "priority", "stop_on_match",
			"cond_from", "cond_chat_jid", "cond_sender_jid",
			"cond_text_pattern", "cond_text_match_type", "cond_case_sensitive",
			"cond_hour_from", "cond_hour_to",
			"action_type", "action_reply_text", "action_webhook_url", "action_script_id",
			"match_count", "last_match_at", "created").
		From("auto_reply_rules").
		OrderBy("priority ASC")
	if v := e.Request.URL.Query().Get("enabled"); v != "" {
		q = q.AndWhere(dbx.HashExp{"enabled": v == "true" || v == "1"})
	}
	var rows []row
	if err := q.All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []row{}
	}
	return e.JSON(http.StatusOK, map[string]any{"rules": rows, "total": len(rows)})
}

// postAutoReplyRule creates a new auto-reply rule.
func postAutoReplyRule(e *core.RequestEvent) error {
	var req struct {
		Name             string  `json:"name"`
		Enabled          bool    `json:"enabled"`
		Priority         float64 `json:"priority"`
		StopOnMatch      bool    `json:"stop_on_match"`
		CondFrom         string  `json:"cond_from"`
		CondChatJID      string  `json:"cond_chat_jid"`
		CondSenderJID    string  `json:"cond_sender_jid"`
		CondTextPattern  string  `json:"cond_text_pattern"`
		CondMatchType    string  `json:"cond_text_match_type"`
		CondCaseSens     bool    `json:"cond_case_sensitive"`
		CondHourFrom     float64 `json:"cond_hour_from"`
		CondHourTo       float64 `json:"cond_hour_to"`
		ActionType       string  `json:"action_type"`
		ActionReplyText  string  `json:"action_reply_text"`
		ActionWebhookURL string  `json:"action_webhook_url"`
		ActionScriptID   string  `json:"action_script_id"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("invalid request body", err)
	}
	if req.Name == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "name is required"})
	}
	if req.ActionType == "" {
		req.ActionType = "reply"
	}
	if req.CondFrom == "" {
		req.CondFrom = "others"
	}
	col, err := pb.FindCollectionByNameOrId("auto_reply_rules")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	r := core.NewRecord(col)
	r.Set("name", req.Name)
	r.Set("enabled", req.Enabled)
	r.Set("priority", req.Priority)
	r.Set("stop_on_match", req.StopOnMatch)
	r.Set("cond_from", req.CondFrom)
	r.Set("cond_chat_jid", req.CondChatJID)
	r.Set("cond_sender_jid", req.CondSenderJID)
	r.Set("cond_text_pattern", req.CondTextPattern)
	r.Set("cond_text_match_type", req.CondMatchType)
	r.Set("cond_case_sensitive", req.CondCaseSens)
	r.Set("cond_hour_from", req.CondHourFrom)
	r.Set("cond_hour_to", req.CondHourTo)
	r.Set("action_type", req.ActionType)
	r.Set("action_reply_text", req.ActionReplyText)
	r.Set("action_webhook_url", req.ActionWebhookURL)
	r.Set("action_script_id", req.ActionScriptID)
	r.Set("match_count", 0)
	if err := pb.Save(r); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"id": r.Id, "name": req.Name})
}

// patchAutoReplyRule updates an existing rule.
func patchAutoReplyRule(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	record, err := pb.FindRecordById("auto_reply_rules", id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	var req map[string]any
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("invalid request body", err)
	}
	allowed := []string{
		"name", "enabled", "priority", "stop_on_match",
		"cond_from", "cond_chat_jid", "cond_sender_jid",
		"cond_text_pattern", "cond_text_match_type", "cond_case_sensitive",
		"cond_hour_from", "cond_hour_to",
		"action_type", "action_reply_text", "action_webhook_url", "action_script_id",
	}
	for _, field := range allowed {
		if v, ok := req[field]; ok {
			record.Set(field, v)
		}
	}
	if err := pb.Save(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"id": record.Id, "updated": true})
}

// deleteAutoReplyRule removes a rule.
func deleteAutoReplyRule(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	record, err := pb.FindRecordById("auto_reply_rules", id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	if err := pb.Delete(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"deleted": true})
}

// postAutoReplyRuleToggle enables or disables a rule.
func postAutoReplyRuleToggle(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	record, err := pb.FindRecordById("auto_reply_rules", id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	record.Set("enabled", !record.GetBool("enabled"))
	if err := pb.Save(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"id": record.Id, "enabled": record.GetBool("enabled")})
}
