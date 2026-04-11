package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const (
	idDeviceActivitySessions = "das1a2b3c4d5e6f7"
	idDeviceActivityProbes   = "dap1a2b3c4d5e6f7"
)

func init() {
	m.Register(upDeviceActivity, downDeviceActivity)
}

func upDeviceActivity(app core.App) error {
	if err := upDeviceActivitySessions(app); err != nil {
		return err
	}
	return upDeviceActivityProbes(app)
}

func upDeviceActivitySessions(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idDeviceActivitySessions); err == nil {
		return nil
	}
	col := core.NewBaseCollection("device_activity_sessions", idDeviceActivitySessions)
	col.Fields.Add(
		textField("das1jid", "jid", true, false),
		textField("das2method", "probe_method", true, false),
		textField("das3started", "started_at", false, false),
		textField("das4stopped", "stopped_at", false, false),
	)
	addAutodateFields(col)
	col.AddIndex("idx_das_jid", false, "jid", "")
	col.AddIndex("idx_das_created", false, "created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	col.CreateRule = &empty
	return app.Save(col)
}

func upDeviceActivityProbes(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(idDeviceActivityProbes); err == nil {
		return nil
	}
	col := core.NewBaseCollection("device_activity_probes", idDeviceActivityProbes)
	col.Fields.Add(
		textField("dap1session", "session_id", false, false),
		textField("dap2jid", "jid", true, false),
		&core.NumberField{Id: "dap3rtt", Name: "rtt_ms"},
		textField("dap4state", "state", false, false),
		&core.NumberField{Id: "dap5median", Name: "median_ms"},
		&core.NumberField{Id: "dap6threshold", Name: "threshold_ms"},
	)
	addAutodateFields(col)
	col.AddIndex("idx_dap_jid", false, "jid", "")
	col.AddIndex("idx_dap_session", false, "session_id", "")
	col.AddIndex("idx_dap_created", false, "created", "")
	empty := ""
	col.ListRule = &empty
	col.ViewRule = &empty
	col.CreateRule = &empty
	return app.Save(col)
}

func downDeviceActivity(app core.App) error {
	for _, id := range []string{idDeviceActivityProbes, idDeviceActivitySessions} {
		col, err := app.FindCollectionByNameOrId(id)
		if err != nil {
			continue
		}
		if err := app.Delete(col); err != nil {
			return err
		}
	}
	return nil
}
