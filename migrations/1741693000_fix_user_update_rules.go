package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("_pb_users_auth_")
		if err != nil {
			return err
		}

		// Ensure users can update their own record
		// This is usually the default, but we'll make it explicit.
		updateRule := "id = @request.auth.id"
		users.UpdateRule = &updateRule

		return app.Save(users)
	}, func(app core.App) error {
		return nil
	})
}
