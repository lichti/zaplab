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

		// Add force_password_change field
		users.Fields.Add(&core.BoolField{
			Id:   "force_pwd_chg",
			Name: "force_password_change",
		})

		return app.Save(users)
	}, func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("_pb_users_auth_")
		if err != nil {
			return err
		}

		users.Fields.RemoveByName("force_password_change")

		return app.Save(users)
	})
}
