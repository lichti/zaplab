package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		// Restrict events collection
		events, err := app.FindCollectionByNameOrId("events")
		if err == nil {
			authRule := "@request.auth.id != \"\""
			events.ListRule = &authRule
			events.ViewRule = &authRule
			if err := app.Save(events); err != nil {
				return err
			}
		}

		// Restrict errors collection
		errors, err := app.FindCollectionByNameOrId("errors")
		if err == nil {
			authRule := "@request.auth.id != \"\""
			errors.ListRule = &authRule
			errors.ViewRule = &authRule
			if err := app.Save(errors); err != nil {
				return err
			}
		}

		return nil
	}, func(app core.App) error {
		// Rollback: set back to public (empty string means public for List/View in this project's context)
		publicRule := ""

		events, err := app.FindCollectionByNameOrId("events")
		if err == nil {
			events.ListRule = &publicRule
			events.ViewRule = &publicRule
			_ = app.Save(events)
		}

		errors, err := app.FindCollectionByNameOrId("errors")
		if err == nil {
			errors.ListRule = &publicRule
			errors.ViewRule = &publicRule
			_ = app.Save(errors)
		}

		return nil
	})
}
