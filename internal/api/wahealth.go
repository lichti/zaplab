package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// rowsToMaps zips column names with row values from dbeScanRows output.
func rowsToMaps(cols []string, rows [][]any) []map[string]any {
	result := make([]map[string]any, len(rows))
	for i, row := range rows {
		m := make(map[string]any, len(cols))
		for j, col := range cols {
			if j < len(row) {
				m[col] = row[j]
			}
		}
		result[i] = m
	}
	return result
}

// getWAPrekeys returns pre-key health information from whatsmeow_pre_keys.
// GET /zaplab/api/wa/prekeys
func getWAPrekeys(e *core.RequestEvent) error {
	if waDB == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]any{"error": "whatsapp DB not available"})
	}

	cols, err := dbeTableColumns("whatsmeow_pre_keys")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}

	sqlRows, err := waDB.Query("SELECT * FROM whatsmeow_pre_keys ORDER BY key_id ASC")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	defer sqlRows.Close()

	rawRows, scanErr := dbeScanRows(sqlRows, len(cols))
	if scanErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": scanErr.Error()})
	}

	keys := rowsToMaps(cols, rawRows)

	// Find the "uploaded" column index for summary stats
	uploadedIdx := -1
	for i, c := range cols {
		if strings.EqualFold(c, "uploaded") {
			uploadedIdx = i
			break
		}
	}

	total := len(rawRows)
	uploaded := 0
	if uploadedIdx >= 0 {
		for _, row := range rawRows {
			if uploadedIdx < len(row) {
				switch v := row[uploadedIdx].(type) {
				case int64:
					if v != 0 {
						uploaded++
					}
				case string:
					if v == "1" || strings.EqualFold(v, "true") {
						uploaded++
					}
				}
			}
		}
	}

	return e.JSON(http.StatusOK, map[string]any{
		"keys":     keys,
		"columns":  cols,
		"total":    total,
		"uploaded": uploaded,
		"pending":  total - uploaded,
	})
}

// getWASecrets returns message secrets from whatsmeow_message_secrets.
// GET /zaplab/api/wa/secrets?limit=100&offset=0&jid=
func getWASecrets(e *core.RequestEvent) error {
	if waDB == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]any{"error": "whatsapp DB not available"})
	}

	limit := 100
	offset := 0
	if v := e.Request.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	if v := e.Request.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	jidFilter := e.Request.URL.Query().Get("jid")

	var query string
	if jidFilter != "" {
		safe := strings.ReplaceAll(jidFilter, "'", "''")
		query = fmt.Sprintf(
			"SELECT * FROM whatsmeow_message_secrets WHERE our_jid = '%s' OR chat_jid = '%s' ORDER BY rowid DESC LIMIT %d OFFSET %d",
			safe, safe, limit, offset,
		)
	} else {
		query = fmt.Sprintf(
			"SELECT * FROM whatsmeow_message_secrets ORDER BY rowid DESC LIMIT %d OFFSET %d",
			limit, offset,
		)
	}

	cols, err := dbeTableColumns("whatsmeow_message_secrets")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}

	sqlRows, err := waDB.Query(query)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	defer sqlRows.Close()

	rawRows, scanErr := dbeScanRows(sqlRows, len(cols))
	if scanErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": scanErr.Error()})
	}

	secrets := rowsToMaps(cols, rawRows)

	// Count total
	countQuery := "SELECT COUNT(*) FROM whatsmeow_message_secrets"
	if jidFilter != "" {
		safe := strings.ReplaceAll(jidFilter, "'", "''")
		countQuery += fmt.Sprintf(" WHERE our_jid = '%s' OR chat_jid = '%s'", safe, safe)
	}
	var total int
	_ = waDB.QueryRow(countQuery).Scan(&total)

	return e.JSON(http.StatusOK, map[string]any{
		"secrets": secrets,
		"columns": cols,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}
