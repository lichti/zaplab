package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const idGroupMembership = "gm1a2b3c4d5e6f7g"

func init() {
	m.Register(upGroupMembership, downGroupMembership)
}

func upGroupMembership(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idGroupMembership); err == nil {
		return nil
	}
	col := core.NewBaseCollection("group_membership", idGroupMembership)
	col.Fields.Add(
		textField("gm1jid", "group_jid", true, false),
		textField("gm2name", "group_name", false, false),
		textField("gm3member", "member_jid", false, false),
		textField("gm4action", "action", false, false),
		textField("gm5actor", "actor_jid", false, false),
	)
	addAutodateFields(col)
	col.AddIndex("idx_gm_group", false, "group_jid", "")
	col.AddIndex("idx_gm_member", false, "member_jid", "")
	col.AddIndex("idx_gm_action", false, "action", "")
	col.AddIndex("idx_gm_created", false, "created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	col.CreateRule = &empty
	return app.Save(col)
}

func downGroupMembership(app core.App) error {
	col, err := app.FindCollectionByNameOrId(idGroupMembership)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
