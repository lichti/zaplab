package api

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

// auditMiddleware records mutating API requests to the audit_log collection.
func auditMiddleware() *hook.Handler[*core.RequestEvent] {
	return &hook.Handler[*core.RequestEvent]{
		Func: func(e *core.RequestEvent) error {
			method := e.Request.Method
			// Only audit mutating methods
			if method != http.MethodPost && method != http.MethodPut &&
				method != http.MethodPatch && method != http.MethodDelete {
				return e.Next()
			}

			// Buffer request body (up to 64 KB)
			bodyStr := ""
			if e.Request.Body != nil && e.Request.ContentLength > 0 {
				limited := io.LimitReader(e.Request.Body, 64*1024)
				buf, readErr := io.ReadAll(limited)
				if readErr == nil {
					bodyStr = string(buf)
					e.Request.Body = io.NopCloser(bytes.NewReader(buf))
				}
			}

			// Proceed with actual handler
			err := e.Next()

			// Get remote IP
			remoteIP := e.Request.RemoteAddr
			if xff := e.Request.Header.Get("X-Forwarded-For"); xff != "" {
				remoteIP = strings.Split(xff, ",")[0]
			}

			// Save to audit_log asynchronously
			path := e.Request.URL.Path
			body := bodyStr
			go func() {
				col, findErr := pb.FindCollectionByNameOrId("audit_log")
				if findErr != nil {
					return
				}
				record := core.NewRecord(col)
				record.Set("method", method)
				record.Set("path", path)
				record.Set("status_code", "")
				record.Set("remote_ip", strings.TrimSpace(remoteIP))
				if len(body) > 4000 {
					body = body[:4000] + "…"
				}
				record.Set("request_body", body)
				_ = pb.Save(record)
			}()

			return err
		},
	}
}

// getAuditLog returns recent audit log entries.
// GET /zaplab/api/audit?days=7&method=&path=&limit=200
func getAuditLog(e *core.RequestEvent) error {
	days := 7
	limit := 200
	method := e.Request.URL.Query().Get("method")
	pathFilter := e.Request.URL.Query().Get("path")
	if v := e.Request.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}
	if v := e.Request.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	type auditRow struct {
		ID          string `db:"id"           json:"id"`
		Method      string `db:"method"       json:"method"`
		Path        string `db:"path"         json:"path"`
		StatusCode  string `db:"status_code"  json:"status_code"`
		RemoteIP    string `db:"remote_ip"    json:"remote_ip"`
		RequestBody string `db:"request_body" json:"request_body"`
		Created     string `db:"created"      json:"created"`
	}

	var exprs []dbx.Expression
	exprs = append(exprs, dbx.NewExp("created >= datetime('now', {:d})", dbx.Params{
		"d": "-" + strconv.Itoa(days) + " days",
	}))

	if method != "" {
		exprs = append(exprs, dbx.HashExp{"method": strings.ToUpper(method)})
	}
	if pathFilter != "" {
		exprs = append(exprs, dbx.Like("path", pathFilter))
	}

	var rows []auditRow
	err := pb.DB().Select("id", "method", "path", "status_code", "remote_ip", "request_body", "created").
		From("audit_log").
		Where(dbx.And(exprs...)).
		OrderBy("created DESC").
		Limit(int64(limit)).
		All(&rows)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []auditRow{}
	}
	return e.JSON(http.StatusOK, map[string]any{"entries": rows, "total": len(rows)})
}
