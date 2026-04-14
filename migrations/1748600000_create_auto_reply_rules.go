package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const idAutoReplyRules = "arr1a2b3c4d5e6f7"

func init() {
	m.Register(upAutoReplyRules, downAutoReplyRules)
}

func upAutoReplyRules(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idAutoReplyRules); err == nil {
		return nil
	}
	col := core.NewBaseCollection("auto_reply_rules", idAutoReplyRules)
	col.Fields.Add(
		textField("arr01name", "name", true, false),
		&core.BoolField{Id: "arr02enabled", Name: "enabled"},
		&core.NumberField{Id: "arr03prio", Name: "priority"},       // lower = evaluated first
		&core.BoolField{Id: "arr04stop", Name: "stop_on_match"},    // stop chain after match

		// ── Conditions ──
		textField("arr05from", "cond_from", false, false),          // all | others | me
		textField("arr06chat", "cond_chat_jid", false, false),      // empty = any chat
		textField("arr07sender", "cond_sender_jid", false, false),  // empty = any sender
		textField("arr08pattern", "cond_text_pattern", false, false),
		textField("arr09match", "cond_text_match_type", false, false), // prefix|contains|exact|regex
		&core.BoolField{Id: "arr10cs", Name: "cond_case_sensitive"},
		&core.NumberField{Id: "arr11hfrom", Name: "cond_hour_from"}, // 0-23, -1 = any
		&core.NumberField{Id: "arr12hto", Name: "cond_hour_to"},     // 0-23, -1 = any

		// ── Action ──
		textField("arr13atype", "action_type", true, false), // reply | webhook | script
		textField("arr14reply", "action_reply_text", false, false),
		textField("arr15wurl", "action_webhook_url", false, false),
		textField("arr16sid", "action_script_id", false, false),

		// ── Stats ──
		&core.NumberField{Id: "arr17cnt", Name: "match_count"},
		textField("arr18last", "last_match_at", false, false),
	)
	addAutodateFields(col)
	col.AddIndex("idx_arr_enabled", false, "enabled", "")
	col.AddIndex("idx_arr_priority", false, "priority", "")
	col.AddIndex("idx_arr_created", false, "created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	col.CreateRule = &empty
	col.UpdateRule = &empty
	col.DeleteRule = &empty
	return app.Save(col)
}

func downAutoReplyRules(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idAutoReplyRules)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
