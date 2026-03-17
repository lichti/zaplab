package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// ── Media Gallery ─────────────────────────────────────────────────────────────
//
// GET /zaplab/api/media/gallery?type=image|video|audio|document|sticker&chat=...&limit=50&offset=0
//
// Returns events that have a file attachment (file != ''), with media metadata
// extracted via json_extract. Thumbnail URLs are constructed for PocketBase file serving.

func getMediaGallery(e *core.RequestEvent) error {
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

	params := map[string]any{"limit": limit, "offset": offset}
	var extraWhere []string

	// Media type filter maps to the presence of specific message sub-objects.
	// Proto-generated Go structs use camelCase json tags.
	switch typeFilter {
	case "image":
		extraWhere = append(extraWhere, "json_extract(raw, '$.Message.imageMessage') IS NOT NULL")
	case "video":
		extraWhere = append(extraWhere, "json_extract(raw, '$.Message.videoMessage') IS NOT NULL")
	case "audio":
		extraWhere = append(extraWhere, "json_extract(raw, '$.Message.audioMessage') IS NOT NULL")
	case "document":
		extraWhere = append(extraWhere, "json_extract(raw, '$.Message.documentMessage') IS NOT NULL")
	case "sticker":
		extraWhere = append(extraWhere, "json_extract(raw, '$.Message.stickerMessage') IS NOT NULL")
	}

	if chatFilter != "" {
		extraWhere = append(extraWhere, "json_extract(raw, '$.Info.Chat') = {:chat}")
		params["chat"] = chatFilter
	}

	extraSQL := ""
	if len(extraWhere) > 0 {
		extraSQL = "AND " + strings.Join(extraWhere, " AND ")
	}

	sqlStr := fmt.Sprintf(`
		SELECT id, msgID,
		       COALESCE(json_extract(raw, '$.Info.Chat'), '')     AS chat,
		       COALESCE(json_extract(raw, '$.Info.Sender'), '')   AS sender,
		       COALESCE(json_extract(raw, '$.Info.IsFromMe'), 0)  AS is_from_me,
		       CASE
		         WHEN json_extract(raw, '$.Message.imageMessage')    IS NOT NULL THEN 'image'
		         WHEN json_extract(raw, '$.Message.videoMessage')    IS NOT NULL THEN 'video'
		         WHEN json_extract(raw, '$.Message.audioMessage')    IS NOT NULL THEN 'audio'
		         WHEN json_extract(raw, '$.Message.documentMessage') IS NOT NULL THEN 'document'
		         WHEN json_extract(raw, '$.Message.stickerMessage')  IS NOT NULL THEN 'sticker'
		         ELSE 'unknown'
		       END AS media_type,
		       COALESCE(
		         json_extract(raw, '$.Message.imageMessage.caption'),
		         json_extract(raw, '$.Message.videoMessage.caption'),
		         json_extract(raw, '$.Message.documentMessage.caption'),
		         json_extract(raw, '$.Message.documentMessage.fileName'),
		         ''
		       ) AS caption,
		       file,
		       created
		FROM events
		WHERE type = 'Message'
		  AND file != ''
		  %s
		ORDER BY created DESC
		LIMIT {:limit} OFFSET {:offset}`, extraSQL)

	type itemRow struct {
		ID        string `db:"id"         json:"id"`
		MsgID     string `db:"msgID"      json:"msgID"`
		Chat      string `db:"chat"       json:"chat"`
		Sender    string `db:"sender"     json:"sender"`
		IsFromMe  any    `db:"is_from_me" json:"is_from_me"`
		MediaType string `db:"media_type" json:"media_type"`
		Caption   string `db:"caption"    json:"caption"`
		File      string `db:"file"       json:"-"`
		Created   string `db:"created"    json:"created"`
		FileURL   string `db:"-"          json:"file_url"`
		ThumbURL  string `db:"-"          json:"thumb_url"`
	}

	var rows []itemRow
	if err := pb.DB().NewQuery(sqlStr).Bind(params).All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []itemRow{}
	}

	// Build file URLs
	for i := range rows {
		if rows[i].File != "" {
			rows[i].FileURL = "/api/files/events/" + rows[i].ID + "/" + rows[i].File
			if rows[i].MediaType == "image" || rows[i].MediaType == "sticker" {
				rows[i].ThumbURL = rows[i].FileURL + "?thumb=300x300"
			} else {
				rows[i].ThumbURL = ""
			}
		}
	}

	// Count total
	countSQL := fmt.Sprintf(`SELECT COUNT(*) AS cnt FROM events WHERE type='Message' AND file!='' %s`, extraSQL)
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
		"items":  rows,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
