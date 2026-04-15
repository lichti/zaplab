package whatsapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"go.mau.fi/whatsmeow/types/events"
)

// autoReplyRule mirrors the auto_reply_rules collection row.
type autoReplyRule struct {
	ID               string  `db:"id"`
	Name             string  `db:"name"`
	Enabled          bool    `db:"enabled"`
	Priority         float64 `db:"priority"`
	StopOnMatch      bool    `db:"stop_on_match"`
	CondFrom         string  `db:"cond_from"`            // all | others | me
	CondChatJID      string  `db:"cond_chat_jid"`        // empty = any
	CondSenderJID    string  `db:"cond_sender_jid"`      // empty = any
	CondTextPattern  string  `db:"cond_text_pattern"`    // empty = any
	CondMatchType    string  `db:"cond_text_match_type"` // prefix|contains|exact|regex
	CondCaseSens     bool    `db:"cond_case_sensitive"`
	CondHourFrom     float64 `db:"cond_hour_from"` // -1 = any
	CondHourTo       float64 `db:"cond_hour_to"`
	ActionType       string  `db:"action_type"` // reply | webhook | script
	ActionReplyText  string  `db:"action_reply_text"`
	ActionWebhookURL string  `db:"action_webhook_url"`
	ActionScriptID   string  `db:"action_script_id"`
}

// EvaluateAutoReplyRules loads enabled rules ordered by priority and runs them
// against the incoming message. Must be called from a goroutine (not the event worker).
func EvaluateAutoReplyRules(evt *events.Message) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("AutoReply: panic recovered: %v", r)
		}
	}()

	if shuttingDown.Load() || pb == nil || pb.DB() == nil || client == nil {
		return
	}

	msgText := getMsg(evt)
	if msgText == "" {
		return // skip non-text messages
	}

	var rules []autoReplyRule
	if err := pb.DB().
		Select("id", "name", "enabled", "priority", "stop_on_match",
			"cond_from", "cond_chat_jid", "cond_sender_jid",
			"cond_text_pattern", "cond_text_match_type", "cond_case_sensitive",
			"cond_hour_from", "cond_hour_to",
			"action_type", "action_reply_text", "action_webhook_url", "action_script_id").
		From("auto_reply_rules").
		Where(dbx.HashExp{"enabled": true}).
		OrderBy("priority ASC").
		All(&rules); err != nil {
		logger.Warnf("AutoReply: failed to load rules: %v", err)
		return
	}

	if len(rules) == 0 {
		return
	}

	chatJID := evt.Info.Chat.String()
	senderJID := evt.Info.Sender.String()
	isFromMe := evt.Info.IsFromMe
	hour := time.Now().Hour()

	logger.Debugf("AutoReply: evaluating %d rule(s) for msg=%q chat=%s fromMe=%v", len(rules), msgText, chatJID, isFromMe)

	for _, rule := range rules {
		if !matchesRule(rule, msgText, chatJID, senderJID, isFromMe, hour) {
			continue
		}
		logger.Infof("AutoReply: rule=%q matched msg=%q action=%s", rule.Name, msgText, rule.ActionType)
		executeRule(rule, evt, msgText, chatJID, senderJID)
		incrementMatchCount(rule.ID)
		if rule.StopOnMatch {
			break
		}
	}
}

// matchesRule returns true when all conditions of the rule are satisfied.
func matchesRule(rule autoReplyRule, text, chatJID, senderJID string, isFromMe bool, hour int) bool {
	// cond_from filter
	switch rule.CondFrom {
	case "me":
		if !isFromMe {
			return false
		}
	case "others", "":
		if isFromMe {
			return false
		}
		// "all" → accept both
	}

	// chat JID filter
	if rule.CondChatJID != "" && rule.CondChatJID != chatJID {
		return false
	}

	// sender JID filter
	if rule.CondSenderJID != "" && rule.CondSenderJID != senderJID {
		return false
	}

	// hour window
	fromH := int(rule.CondHourFrom)
	toH := int(rule.CondHourTo)
	if fromH >= 0 && toH >= 0 {
		if fromH <= toH {
			if hour < fromH || hour > toH {
				return false
			}
		} else {
			// wraps midnight e.g. 22-06
			if hour < fromH && hour > toH {
				return false
			}
		}
	}

	// text pattern
	if rule.CondTextPattern == "" {
		return true
	}
	subject := text
	pattern := rule.CondTextPattern
	if !rule.CondCaseSens {
		subject = strings.ToLower(subject)
		pattern = strings.ToLower(pattern)
	}
	switch rule.CondMatchType {
	case "prefix":
		return strings.HasPrefix(subject, pattern)
	case "exact":
		return subject == pattern
	case "regex":
		re, err := regexp.Compile(rule.CondTextPattern) // use original pattern (regex is case-aware via flags)
		if err != nil {
			return false
		}
		return re.MatchString(text)
	default: // "contains" or empty
		return strings.Contains(subject, pattern)
	}
}

// executeRule runs the matched rule's action.
func executeRule(rule autoReplyRule, evt *events.Message, msgText, chatJID, senderJID string) {
	switch rule.ActionType {
	case "reply":
		if rule.ActionReplyText == "" {
			return
		}
		replyText := expandVars(rule.ActionReplyText, evt, msgText, chatJID, senderJID)
		jid, ok := ParseJID(chatJID)
		if !ok {
			logger.Warnf("AutoReply rule=%s: invalid chat JID %q", rule.Name, chatJID)
			return
		}
		reply := &ReplyInfo{
			MessageID: evt.Info.ID,
			Sender:    evt.Info.Sender,
			Text:      msgText,
		}
		if _, _, err := SendConversationMessage(jid, replyText, reply); err != nil {
			logger.Warnf("AutoReply rule=%s: send failed: %v", rule.Name, err)
		}

	case "webhook":
		if rule.ActionWebhookURL == "" {
			return
		}
		payload := map[string]any{
			"rule_id":    rule.ID,
			"rule_name":  rule.Name,
			"chat_jid":   chatJID,
			"sender_jid": senderJID,
			"text":       msgText,
			"msg_id":     evt.Info.ID,
			"timestamp":  evt.Info.Timestamp.UTC().Format(time.RFC3339),
		}
		body, _ := json.Marshal(payload)
		go func() {
			resp, err := http.Post(rule.ActionWebhookURL, "application/json", bytes.NewReader(body)) //nolint:gosec
			if err != nil {
				logger.Warnf("AutoReply webhook rule=%s: %v", rule.Name, err)
				return
			}
			_ = resp.Body.Close()
		}()

	case "script":
		// Script execution is handled by the Sandbox/cmd system via HandleCmd.
		if rule.ActionScriptID == "" {
			return
		}
		out := HandleCmd(rule.ActionScriptID, []string{chatJID, senderJID, msgText}, evt)
		logger.Infof("AutoReply script rule=%s scriptID=%s output=%s", rule.Name, rule.ActionScriptID, out)
	}
}

// expandVars replaces template variables in a reply string.
// Supported: {sender}, {name}, {chat}, {text}, {hour}
func expandVars(template string, evt *events.Message, msgText, chatJID, senderJID string) string {
	name := evt.Info.PushName
	if name == "" {
		name = senderJID
	}
	r := strings.NewReplacer(
		"{sender}", senderJID,
		"{name}", name,
		"{chat}", chatJID,
		"{text}", msgText,
		"{hour}", fmt.Sprintf("%02d", time.Now().Hour()),
	)
	return r.Replace(template)
}

// incrementMatchCount bumps match_count and sets last_match_at for a rule.
func incrementMatchCount(ruleID string) {
	if shuttingDown.Load() {
		return
	}
	col, err := pb.FindCollectionByNameOrId("auto_reply_rules")
	if err != nil {
		return
	}
	rec, err := pb.FindRecordById(col, ruleID)
	if err != nil {
		return
	}
	count := rec.GetFloat("match_count") + 1
	rec.Set("match_count", count)
	rec.Set("last_match_at", time.Now().UTC().Format(time.RFC3339))
	if err := pb.Save(rec); err != nil && !shuttingDown.Load() {
		logger.Warnf("AutoReply: failed to update match_count rule=%s: %v", ruleID, err)
	}
}
