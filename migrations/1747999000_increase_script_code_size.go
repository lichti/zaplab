package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(upIncreaseScriptCodeSize, nil)
}

// upIncreaseScriptCodeSize raises the MaxSize of the scripts.code text field
// from the default 5 000 characters to 65 535 so that larger scripts can be stored.
func upIncreaseScriptCodeSize(app core.App) error {
	col, err := app.FindCollectionByNameOrId("scripts")
	if err != nil {
		return nil // collection not yet created — skip
	}
	field := col.Fields.GetByName("code")
	if field == nil {
		return nil
	}
	tf, ok := field.(*core.TextField)
	if !ok {
		return nil
	}
	if tf.Max >= 65535 {
		return nil // already large enough
	}
	tf.Max = 65535
	return app.Save(col)
}
