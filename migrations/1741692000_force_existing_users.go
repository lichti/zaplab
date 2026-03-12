package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		// Set force_password_change to true for all existing users
		records, err := app.FindRecordsByFilter("users", "id != ''", "", 0, 0)
		if err != nil {
			return nil // users collection might not exist yet or be empty
		}

		for _, record := range records {
			record.Set("force_password_change", true)
			if err := app.SaveNoValidate(record); err != nil {
				continue
			}
		}

		return nil
	}, func(app core.App) error {
		return nil
	})
}
