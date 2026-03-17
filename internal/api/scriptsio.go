package api

import (
	"encoding/json"
	"net/http"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// getScriptsExport exports all scripts as a JSON array.
// GET /zaplab/api/scripts/export
func getScriptsExport(e *core.RequestEvent) error {
	type exportRow struct {
		Name        string  `db:"name"         json:"name"`
		Description string  `db:"description"  json:"description"`
		Code        string  `db:"code"         json:"code"`
		Enabled     bool    `db:"enabled"      json:"enabled"`
		TimeoutSecs float64 `db:"timeout_secs" json:"timeout_secs"`
		CronExpr    string  `db:"cron_expression" json:"cron_expression,omitempty"`
	}
	var rows []exportRow
	err := pb.DB().Select("name", "description", "code", "enabled", "timeout_secs",
		"COALESCE(cron_expression,'') as cron_expression").
		From("scripts").OrderBy("name ASC").All(&rows)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []exportRow{}
	}
	e.Response.Header().Set("Content-Disposition", "attachment; filename=\"scripts.json\"")
	return e.JSON(http.StatusOK, rows)
}

// postScriptsImport imports scripts from a JSON array, upserting by name.
// POST /zaplab/api/scripts/import
// Body: [{"name","description","code","enabled","timeout_secs","cron_expression"}]
func postScriptsImport(e *core.RequestEvent) error {
	var items []struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Code        string  `json:"code"`
		Enabled     bool    `json:"enabled"`
		TimeoutSecs float64 `json:"timeout_secs"`
		CronExpr    string  `json:"cron_expression"`
	}
	if err := json.NewDecoder(e.Request.Body).Decode(&items); err != nil {
		return apis.NewBadRequestError("invalid JSON array", err)
	}
	if len(items) == 0 {
		return apis.NewBadRequestError("empty import list", nil)
	}

	col, err := pb.FindCollectionByNameOrId("scripts")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": "scripts collection not found"})
	}

	var created, updated int
	for _, item := range items {
		if item.Name == "" {
			continue
		}
		if item.TimeoutSecs <= 0 {
			item.TimeoutSecs = 10
		}

		// Try to find existing script by name
		type idRow struct {
			ID string `db:"id"`
		}
		var existing []idRow
		_ = pb.DB().Select("id").From("scripts").
			Where(dbx.HashExp{"name": item.Name}).
			Limit(1).All(&existing)

		var record *core.Record
		if len(existing) > 0 {
			record, err = pb.FindRecordById("scripts", existing[0].ID)
			if err != nil {
				continue
			}
			updated++
		} else {
			record = core.NewRecord(col)
			created++
		}

		record.Set("name", item.Name)
		record.Set("description", item.Description)
		record.Set("code", item.Code)
		record.Set("enabled", item.Enabled)
		record.Set("timeout_secs", item.TimeoutSecs)
		if item.CronExpr != "" {
			record.Set("cron_expression", item.CronExpr)
		}
		_ = pb.Save(record)
	}

	return e.JSON(http.StatusOK, map[string]any{
		"message": "import complete",
		"created": created,
		"updated": updated,
	})
}
