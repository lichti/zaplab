package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// ── Full-text Message Search ───────────────────────────────────────────────────
//
// GET /zaplab/api/search?q=...&type=...&chat=...&limit=50&offset=0
//
// Searches stored events using SQLite json_extract + LIKE.
// Matches against: Conversation, ExtendedTextMessage.Text,
// ImageMessage.Caption, VideoMessage.Caption, DocumentMessage.Caption, msgID.

func getSearch(e *core.RequestEvent) error {
	q := strings.TrimSpace(e.Request.URL.Query().Get("q"))
	typeFilter := e.Request.URL.Query().Get("type")
	chatFilter := e.Request.URL.Query().Get("chat")

	limit, _ := strconv.Atoi(e.Request.URL.Query().Get("limit"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	offset, _ := strconv.Atoi(e.Request.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	if q == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "q parameter is required"})
	}

	like := "%" + q + "%"

	// Build WHERE clause
	var where []string
	params := map[string]any{"like": like, "q": q}

	where = append(where, `(
		json_extract(raw, '$.Message.Conversation') LIKE {:like}
		OR json_extract(raw, '$.Message.ExtendedTextMessage.Text') LIKE {:like}
		OR json_extract(raw, '$.Message.ImageMessage.Caption') LIKE {:like}
		OR json_extract(raw, '$.Message.VideoMessage.Caption') LIKE {:like}
		OR json_extract(raw, '$.Message.DocumentMessage.Caption') LIKE {:like}
		OR msgID = {:q}
	)`)

	if typeFilter != "" {
		where = append(where, "type = {:type}")
		params["type"] = typeFilter
	} else {
		where = append(where, "type LIKE '%Message%'")
	}

	if chatFilter != "" {
		where = append(where, "json_extract(raw, '$.Info.Chat') = {:chat}")
		params["chat"] = chatFilter
	}

	whereSQL := strings.Join(where, " AND ")

	type resultRow struct {
		ID          string `db:"id"           json:"id"`
		Type        string `db:"type"         json:"type"`
		MsgID       string `db:"msgID"        json:"msgID"`
		Chat        string `db:"chat"         json:"chat"`
		Sender      string `db:"sender"       json:"sender"`
		TextPreview string `db:"text_preview" json:"text_preview"`
		Created     string `db:"created"      json:"created"`
	}

	sqlStr := fmt.Sprintf(`
		SELECT id, type, msgID,
		       COALESCE(json_extract(raw, '$.Info.Chat'), '')   AS chat,
		       COALESCE(json_extract(raw, '$.Info.Sender'), '') AS sender,
		       COALESCE(
		         json_extract(raw, '$.Message.Conversation'),
		         json_extract(raw, '$.Message.ExtendedTextMessage.Text'),
		         json_extract(raw, '$.Message.ImageMessage.Caption'),
		         json_extract(raw, '$.Message.VideoMessage.Caption'),
		         json_extract(raw, '$.Message.DocumentMessage.Caption'),
		         ''
		       ) AS text_preview,
		       created
		FROM events
		WHERE %s
		ORDER BY created DESC
		LIMIT {:limit} OFFSET {:offset}`, whereSQL)

	params["limit"] = limit
	params["offset"] = offset

	var rows []resultRow
	if err := pb.DB().NewQuery(sqlStr).Bind(params).All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []resultRow{}
	}

	// Truncate text preview
	for i := range rows {
		if len(rows[i].TextPreview) > 200 {
			rows[i].TextPreview = rows[i].TextPreview[:200] + "…"
		}
	}

	// Count total
	countSQL := fmt.Sprintf(`SELECT COUNT(*) AS cnt FROM events WHERE %s`, whereSQL)
	type countRow struct {
		Count int `db:"cnt"`
	}
	var cr []countRow
	_ = pb.DB().NewQuery(countSQL).Bind(params).All(&cr)
	total := 0
	if len(cr) > 0 {
		total = cr[0].Count
	}

	return e.JSON(http.StatusOK, map[string]any{
		"results": rows,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"q":       q,
	})
}
