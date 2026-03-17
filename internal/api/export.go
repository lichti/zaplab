package api

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// getExportEvents exports events as CSV or JSON.
// Query params:
//
//	format  "csv" | "json" (default "json")
//	type    event type filter (optional)
//	from    ISO date (optional)
//	to      ISO date (optional)
//	limit   max rows (default 1000, max 10000)
func getExportEvents(e *core.RequestEvent) error {
	format := e.Request.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	evtType := e.Request.URL.Query().Get("type")
	from := e.Request.URL.Query().Get("from")
	to := e.Request.URL.Query().Get("to")
	limit, _ := strconv.Atoi(e.Request.URL.Query().Get("limit"))
	if limit < 1 || limit > 10000 {
		limit = 1000
	}

	where := "WHERE 1=1"
	if evtType != "" {
		where += fmt.Sprintf(" AND type = '%s'", sanitizeSQL(evtType))
	}
	if from != "" {
		where += fmt.Sprintf(" AND created >= '%s'", sanitizeSQL(from))
	}
	if to != "" {
		where += fmt.Sprintf(" AND created <= '%s'", sanitizeSQL(to))
	}

	type evtRow struct {
		ID      string `db:"id"      json:"id"`
		Type    string `db:"type"    json:"type"`
		MsgID   string `db:"msgID"   json:"msg_id"`
		Raw     string `db:"raw"     json:"raw"`
		Created string `db:"created" json:"created"`
	}

	sql := fmt.Sprintf(`SELECT id, type, COALESCE(msgID,'') as msgID, raw, created FROM events %s ORDER BY created DESC LIMIT %d`, where, limit)
	var rows []evtRow
	if err := pb.DB().NewQuery(sql).All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []evtRow{}
	}

	switch format {
	case "csv":
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		_ = w.Write([]string{"id", "type", "msg_id", "created", "raw"})
		for _, r := range rows {
			raw := r.Raw
			if len(raw) > 500 {
				raw = raw[:500] + "…"
			}
			_ = w.Write([]string{r.ID, r.Type, r.MsgID, r.Created, raw})
		}
		w.Flush()
		e.Response.Header().Set("Content-Type", "text/csv")
		e.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"zaplab-events-%s.csv\"", time.Now().Format("20060102-150405")))
		_, err := e.Response.Write(buf.Bytes())
		return err
	default: // json
		e.Response.Header().Set("Content-Type", "application/json")
		e.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"zaplab-events-%s.json\"", time.Now().Format("20060102-150405")))
		return json.NewEncoder(e.Response).Encode(map[string]any{"events": rows, "count": len(rows)})
	}
}

// getExportConversation exports a conversation as CSV or JSON.
// Query params: jid (required), format, limit
func getExportConversation(e *core.RequestEvent) error {
	jid := e.Request.URL.Query().Get("jid")
	if jid == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "jid is required"})
	}
	format := e.Request.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	limit, _ := strconv.Atoi(e.Request.URL.Query().Get("limit"))
	if limit < 1 || limit > 10000 {
		limit = 1000
	}

	type msgRow struct {
		ID      string `db:"id"      json:"id"`
		Type    string `db:"type"    json:"type"`
		MsgID   string `db:"msgID"   json:"msg_id"`
		Raw     string `db:"raw"     json:"raw"`
		Created string `db:"created" json:"created"`
	}

	sql := fmt.Sprintf(`
        SELECT id, type, COALESCE(msgID,'') as msgID, raw, created
        FROM events
        WHERE type LIKE '%%Message%%'
          AND (raw LIKE '%%"Chat":{"User":"%s"%%' OR raw LIKE '%%"Chat":"%s"%%')
        ORDER BY created ASC
        LIMIT %d`, sanitizeSQL(jid), sanitizeSQL(jid), limit)

	var rows []msgRow
	if err := pb.DB().NewQuery(sql).All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []msgRow{}
	}

	switch format {
	case "csv":
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		_ = w.Write([]string{"id", "type", "msg_id", "created", "raw"})
		for _, r := range rows {
			raw := r.Raw
			if len(raw) > 500 {
				raw = raw[:500] + "…"
			}
			_ = w.Write([]string{r.ID, r.Type, r.MsgID, r.Created, raw})
		}
		w.Flush()
		e.Response.Header().Set("Content-Type", "text/csv")
		e.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"zaplab-conv-%s-%s.csv\"", sanitizeSQL(jid), time.Now().Format("20060102")))
		_, err := e.Response.Write(buf.Bytes())
		return err
	default:
		e.Response.Header().Set("Content-Type", "application/json")
		e.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"zaplab-conv-%s-%s.json\"", sanitizeSQL(jid), time.Now().Format("20060102")))
		return json.NewEncoder(e.Response).Encode(map[string]any{"jid": jid, "messages": rows, "count": len(rows)})
	}
}

// getExportFramesHAR exports captured frames in HAR (HTTP Archive) format.
// This maps log frames to HAR entries for use with browser dev tools.
// Query params: from, to, limit (default 500)
func getExportFramesHAR(e *core.RequestEvent) error {
	limit, _ := strconv.Atoi(e.Request.URL.Query().Get("limit"))
	if limit < 1 || limit > 5000 {
		limit = 500
	}
	from := e.Request.URL.Query().Get("from")
	to := e.Request.URL.Query().Get("to")

	where := "WHERE 1=1"
	if from != "" {
		where += fmt.Sprintf(" AND created >= '%s'", sanitizeSQL(from))
	}
	if to != "" {
		where += fmt.Sprintf(" AND created <= '%s'", sanitizeSQL(to))
	}

	type frameRow struct {
		ID      string `db:"id"`
		Module  string `db:"module"`
		Level   string `db:"level"`
		Seq     string `db:"seq"`
		Msg     string `db:"msg"`
		Created string `db:"created"`
	}

	sql := fmt.Sprintf(`SELECT id, COALESCE(module,'') as module, COALESCE(level,'') as level, COALESCE(seq,'') as seq, COALESCE(msg,'') as msg, created FROM frames %s ORDER BY created ASC LIMIT %d`, where, limit)
	var frames []frameRow
	if err := pb.DB().NewQuery(sql).All(&frames); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}

	// Build HAR structure
	type harEntry struct {
		StartedDateTime string         `json:"startedDateTime"`
		Time            float64        `json:"time"`
		Request         map[string]any `json:"request"`
		Response        map[string]any `json:"response"`
		Comment         string         `json:"comment"`
	}

	var entries []harEntry
	for _, f := range frames {
		entries = append(entries, harEntry{
			StartedDateTime: strings.Replace(f.Created, " ", "T", 1) + "Z",
			Time:            0,
			Request: map[string]any{
				"method":      "WS",
				"url":         fmt.Sprintf("wss://web.whatsapp.com/ws/%s/%s", f.Module, f.Seq),
				"httpVersion": "WebSocket",
				"headers":     []any{},
				"queryString": []any{},
				"cookies":     []any{},
				"headersSize": -1,
				"bodySize":    len(f.Msg),
				"postData": map[string]any{
					"mimeType": "application/json",
					"text":     f.Msg,
				},
			},
			Response: map[string]any{
				"status":      200,
				"statusText":  f.Level,
				"httpVersion": "WebSocket",
				"headers":     []any{},
				"cookies":     []any{},
				"content": map[string]any{
					"size":     len(f.Msg),
					"mimeType": "application/json",
					"text":     f.Msg,
				},
				"redirectURL": "",
				"headersSize": -1,
				"bodySize":    len(f.Msg),
			},
			Comment: fmt.Sprintf("[%s] %s", f.Module, f.Level),
		})
	}

	if entries == nil {
		entries = []harEntry{}
	}

	har := map[string]any{
		"log": map[string]any{
			"version": "1.2",
			"creator": map[string]any{
				"name":    "ZapLab",
				"version": "1.0",
			},
			"entries": entries,
		},
	}

	e.Response.Header().Set("Content-Type", "application/json")
	e.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"zaplab-frames-%s.har\"", time.Now().Format("20060102-150405")))
	return json.NewEncoder(e.Response).Encode(har)
}
