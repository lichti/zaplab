package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// ── Annotations CRUD ──────────────────────────────────────────────────────────

// getAnnotations returns annotations, optionally filtered by event_id or jid.
// Query params: event_id, jid, page, per_page.
func getAnnotations(e *core.RequestEvent) error {
	q := e.Request.URL.Query()
	eventID := q.Get("event_id")
	jid := q.Get("jid")
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if perPage < 1 || perPage > 200 {
		perPage = 50
	}

	var exprs []dbx.Expression
	if eventID != "" {
		exprs = append(exprs, dbx.HashExp{"event_id": eventID})
	}
	if jid != "" {
		exprs = append(exprs, dbx.HashExp{"jid": jid})
	}

	// Count
	var total int
	countQ := pb.DB().Select("COUNT(*)").From("annotations")
	if len(exprs) > 0 {
		countQ = countQ.Where(dbx.And(exprs...))
	}
	_ = countQ.Row(&total)

	// Fetch
	type annotationRow struct {
		ID        string `db:"id"         json:"id"`
		EventID   string `db:"event_id"   json:"event_id"`
		EventType string `db:"event_type" json:"event_type"`
		JID       string `db:"jid"        json:"jid"`
		Note      string `db:"note"       json:"note"`
		Tags      string `db:"tags"       json:"tags_raw"`
		Created   string `db:"created"    json:"created"`
		Updated   string `db:"updated"    json:"updated"`
	}
	query := pb.DB().Select("id", "event_id", "event_type", "jid", "note", "tags", "created", "updated").
		From("annotations")
	if len(exprs) > 0 {
		query = query.Where(dbx.And(exprs...))
	}
	offset := (page - 1) * perPage
	query = query.OrderBy("created DESC").Limit(int64(perPage)).Offset(int64(offset))

	var rows []annotationRow
	if err := query.All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}

	// Parse tags JSON for each row
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		var tags []string
		_ = json.Unmarshal([]byte(r.Tags), &tags)
		if tags == nil {
			tags = []string{}
		}
		items = append(items, map[string]any{
			"id":         r.ID,
			"event_id":   r.EventID,
			"event_type": r.EventType,
			"jid":        r.JID,
			"note":       r.Note,
			"tags":       tags,
			"created":    r.Created,
			"updated":    r.Updated,
		})
	}

	return e.JSON(http.StatusOK, map[string]any{
		"items":    items,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// postAnnotation creates a new annotation.
// Body: {"event_id", "event_type", "jid", "note", "tags": [...]}
func postAnnotation(e *core.RequestEvent) error {
	var req struct {
		EventID   string   `json:"event_id"`
		EventType string   `json:"event_type"`
		JID       string   `json:"jid"`
		Note      string   `json:"note"`
		Tags      []string `json:"tags"`
	}
	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return apis.NewBadRequestError("invalid body", err)
	}
	if req.Note == "" {
		return apis.NewBadRequestError("note is required", nil)
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}

	col, err := pb.FindCollectionByNameOrId("annotations")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": "annotations collection not found"})
	}
	record := core.NewRecord(col)
	record.Set("event_id", req.EventID)
	record.Set("event_type", req.EventType)
	record.Set("jid", req.JID)
	record.Set("note", req.Note)
	record.Set("tags", req.Tags)
	if err := pb.Save(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"id":         record.Id,
		"event_id":   record.GetString("event_id"),
		"event_type": record.GetString("event_type"),
		"jid":        record.GetString("jid"),
		"note":       record.GetString("note"),
		"tags":       record.Get("tags"),
		"created":    record.GetDateTime("created").String(),
		"updated":    record.GetDateTime("updated").String(),
	})
}

// patchAnnotation updates an existing annotation's note and/or tags.
// Body: {"note", "tags": [...]}
func patchAnnotation(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	record, err := pb.FindRecordById("annotations", id)
	if err != nil {
		return apis.NewNotFoundError("annotation not found", nil)
	}

	var req struct {
		Note *string  `json:"note"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return apis.NewBadRequestError("invalid body", err)
	}
	if req.Note != nil {
		record.Set("note", *req.Note)
	}
	if req.Tags != nil {
		record.Set("tags", req.Tags)
	}
	if err := pb.Save(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"id":         record.Id,
		"event_id":   record.GetString("event_id"),
		"event_type": record.GetString("event_type"),
		"jid":        record.GetString("jid"),
		"note":       record.GetString("note"),
		"tags":       record.Get("tags"),
		"created":    record.GetDateTime("created").String(),
		"updated":    record.GetDateTime("updated").String(),
	})
}

// deleteAnnotation removes an annotation by id.
func deleteAnnotation(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	record, err := pb.FindRecordById("annotations", id)
	if err != nil {
		return apis.NewNotFoundError("annotation not found", nil)
	}
	if err := pb.Delete(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "annotation deleted"})
}
