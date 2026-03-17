package api

import (
	"net/http"
	"strconv"

	"github.com/lichti/zaplab/internal/whatsapp"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// getFrames returns paginated log frames from the persistent PocketBase `frames` collection.
// Query params: module, level, search, page (default 1), per_page (default 100, max 500)
func getFrames(e *core.RequestEvent) error {
	q := e.Request.URL.Query()
	module := q.Get("module")
	level := q.Get("level")
	search := q.Get("search")
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if perPage < 1 || perPage > 500 {
		perPage = 100
	}

	col, err := pb.FindCollectionByNameOrId("frames")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "frames collection not found"})
	}

	// Build expressions
	var exprs []dbx.Expression
	if module != "" {
		exprs = append(exprs, dbx.HashExp{"module": module})
	}
	if level != "" {
		exprs = append(exprs, dbx.HashExp{"level": level})
	}
	if search != "" {
		exprs = append(exprs, dbx.Like("msg", search))
	}
	where := dbx.And(exprs...)

	// Count total
	var total int
	countQ := pb.DB().Select("COUNT(*)").From(col.Name)
	if len(exprs) > 0 {
		countQ = countQ.Where(where)
	}
	_ = countQ.Row(&total)

	// Fetch rows
	query := pb.DB().Select("id", "module", "level", "seq", "msg", "created").From(col.Name)
	if len(exprs) > 0 {
		query = query.Where(where)
	}
	offset := (page - 1) * perPage
	query = query.OrderBy("created DESC").Limit(int64(perPage)).Offset(int64(offset))

	type frameRow struct {
		ID      string `db:"id" json:"id"`
		Module  string `db:"module" json:"module"`
		Level   string `db:"level" json:"level"`
		Seq     string `db:"seq" json:"seq"`
		Msg     string `db:"msg" json:"msg"`
		Created string `db:"created" json:"created"`
	}
	var rows []frameRow
	if err := query.All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if rows == nil {
		rows = []frameRow{}
	}

	return e.JSON(http.StatusOK, map[string]any{
		"items":    rows,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// getFramesRing returns the in-memory ring buffer snapshot (includes DEBUG entries).
// Query params: module, level, limit (default 500, max 2000)
func getFramesRing(e *core.RequestEvent) error {
	q := e.Request.URL.Query()
	module := q.Get("module")
	level := q.Get("level")
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 || limit > 2000 {
		limit = 500
	}

	entries := whatsapp.GetLogEntries(module, level, limit)
	if entries == nil {
		entries = []whatsapp.LogEntry{}
	}
	return e.JSON(http.StatusOK, map[string]any{
		"entries": entries,
		"total":   len(entries),
	})
}

// getFramesModules returns the distinct log modules seen in the ring buffer.
func getFramesModules(e *core.RequestEvent) error {
	entries := whatsapp.GetLogEntries("", "", 0)
	seen := make(map[string]struct{})
	for _, e := range entries {
		seen[e.Module] = struct{}{}
	}
	modules := make([]string, 0, len(seen))
	for m := range seen {
		modules = append(modules, m)
	}
	return e.JSON(http.StatusOK, map[string]any{"modules": modules})
}

// getDeviceKeys returns the public key info for the current device (no private keys).
func getDeviceKeys(e *core.RequestEvent) error {
	keys := whatsapp.GetDeviceKeys()
	if keys == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "client not bootstrapped"})
	}
	return e.JSON(http.StatusOK, keys)
}
