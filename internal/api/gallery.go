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
// Media events are stored with a dedicated type column:
//   type='Message.ImageMessage'    — images
//   type='Message.VideoMessage'    — videos
//   type='Message.AudioMessage'    — audio / voice notes
//   type='Message.DocumentMessage' — documents
//   type='Message.StickerMessage'  — stickers
//
// All media events that have a downloaded file (file != '') are returned.

var mediaTypes = []string{
	"Message.ImageMessage",
	"Message.VideoMessage",
	"Message.AudioMessage",
	"Message.DocumentMessage",
	"Message.StickerMessage",
}

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

	// Filter by specific media type using the event type column directly
	switch typeFilter {
	case "image":
		extraWhere = append(extraWhere, "type = 'Message.ImageMessage'")
	case "video":
		extraWhere = append(extraWhere, "type = 'Message.VideoMessage'")
	case "audio":
		extraWhere = append(extraWhere, "type = 'Message.AudioMessage'")
	case "document":
		extraWhere = append(extraWhere, "type = 'Message.DocumentMessage'")
	case "sticker":
		extraWhere = append(extraWhere, "type = 'Message.StickerMessage'")
	}

	if chatFilter != "" {
		extraWhere = append(extraWhere, "json_extract(raw, '$.Info.Chat') = {:chat}")
		params["chat"] = chatFilter
	}

	extraSQL := ""
	if len(extraWhere) > 0 {
		extraSQL = "AND " + strings.Join(extraWhere, " AND ")
	}

	// Build the type IN list for the base WHERE
	typeList := fmt.Sprintf("'%s'", strings.Join(mediaTypes, "','"))

	sqlStr := fmt.Sprintf(`
		SELECT id, msgID,
		       COALESCE(json_extract(raw, '$.Info.Chat'), '')    AS chat,
		       COALESCE(json_extract(raw, '$.Info.Sender'), '')  AS sender,
		       COALESCE(json_extract(raw, '$.Info.IsFromMe'), 0) AS is_from_me,
		       CASE type
		         WHEN 'Message.ImageMessage'    THEN 'image'
		         WHEN 'Message.VideoMessage'    THEN 'video'
		         WHEN 'Message.AudioMessage'    THEN 'audio'
		         WHEN 'Message.DocumentMessage' THEN 'document'
		         WHEN 'Message.StickerMessage'  THEN 'sticker'
		         ELSE 'unknown'
		       END AS media_type,
		       COALESCE(
		         json_extract(raw, '$.Message.imageMessage.caption'),
		         json_extract(raw, '$.Message.videoMessage.caption'),
		         json_extract(raw, '$.Message.documentMessage.caption'),
		         json_extract(raw, '$.Message.documentMessage.fileName'),
		         json_extract(raw, '$.Message.documentMessage.title'),
		         ''
		       ) AS caption,
		       file,
		       created
		FROM events
		WHERE type IN (%s)
		  AND file != ''
		  %s
		ORDER BY created DESC
		LIMIT {:limit} OFFSET {:offset}`, typeList, extraSQL)

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

	// Build file URLs — thumbnails only for image and sticker
	for i := range rows {
		if rows[i].File != "" {
			rows[i].FileURL = "/api/files/events/" + rows[i].ID + "/" + rows[i].File
			if rows[i].MediaType == "image" || rows[i].MediaType == "sticker" {
				rows[i].ThumbURL = rows[i].FileURL + "?thumb=300x300"
			}
		}
	}

	// Count total matching rows
	countSQL := fmt.Sprintf(`SELECT COUNT(*) AS cnt FROM events WHERE type IN (%s) AND file != '' %s`, typeList, extraSQL)
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
