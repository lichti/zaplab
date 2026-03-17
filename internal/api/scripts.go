package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/lichti/zaplab/internal/whatsapp"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// ── Script CRUD + Execution ────────────────────────────────────────────────────
//
// Collection: scripts
// Fields:     name, description, code, enabled, timeout_secs,
//             last_run_status, last_run_output, last_run_duration_ms, last_run_error

// getScripts returns all scripts.
func getScripts(e *core.RequestEvent) error {
	type scriptRow struct {
		ID                string  `db:"id"                   json:"id"`
		Name              string  `db:"name"                 json:"name"`
		Description       string  `db:"description"          json:"description"`
		Code              string  `db:"code"                 json:"code"`
		Enabled           bool    `db:"enabled"              json:"enabled"`
		TimeoutSecs       float64 `db:"timeout_secs"         json:"timeout_secs"`
		LastRunStatus     string  `db:"last_run_status"      json:"last_run_status"`
		LastRunOutput     string  `db:"last_run_output"      json:"last_run_output"`
		LastRunDurationMs float64 `db:"last_run_duration_ms" json:"last_run_duration_ms"`
		LastRunError      string  `db:"last_run_error"       json:"last_run_error"`
		Created           string  `db:"created"              json:"created"`
		Updated           string  `db:"updated"              json:"updated"`
	}
	var rows []scriptRow
	err := pb.DB().Select("id", "name", "description", "code", "enabled", "timeout_secs",
		"last_run_status", "last_run_output", "last_run_duration_ms", "last_run_error",
		"created", "updated").
		From("scripts").OrderBy("name ASC").All(&rows)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []scriptRow{}
	}
	return e.JSON(http.StatusOK, map[string]any{"scripts": rows, "total": len(rows)})
}

// postScript creates a new script.
// Body: {"name", "description", "code", "enabled", "timeout_secs"}
func postScript(e *core.RequestEvent) error {
	var req struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Code        string  `json:"code"`
		Enabled     bool    `json:"enabled"`
		TimeoutSecs float64 `json:"timeout_secs"`
	}
	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return apis.NewBadRequestError("invalid body", err)
	}
	if req.Name == "" {
		return apis.NewBadRequestError("name is required", nil)
	}
	if req.TimeoutSecs <= 0 {
		req.TimeoutSecs = 10
	}

	col, err := pb.FindCollectionByNameOrId("scripts")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": "scripts collection not found"})
	}
	record := core.NewRecord(col)
	record.Set("name", req.Name)
	record.Set("description", req.Description)
	record.Set("code", req.Code)
	record.Set("enabled", req.Enabled)
	record.Set("timeout_secs", req.TimeoutSecs)
	if err := pb.Save(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, scriptRecordToMap(record))
}

// patchScript updates a script.
func patchScript(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	record, err := pb.FindRecordById("scripts", id)
	if err != nil {
		return apis.NewNotFoundError("script not found", nil)
	}

	var req struct {
		Name        *string  `json:"name"`
		Description *string  `json:"description"`
		Code        *string  `json:"code"`
		Enabled     *bool    `json:"enabled"`
		TimeoutSecs *float64 `json:"timeout_secs"`
	}
	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return apis.NewBadRequestError("invalid body", err)
	}
	if req.Name != nil {
		record.Set("name", *req.Name)
	}
	if req.Description != nil {
		record.Set("description", *req.Description)
	}
	if req.Code != nil {
		record.Set("code", *req.Code)
	}
	if req.Enabled != nil {
		record.Set("enabled", *req.Enabled)
	}
	if req.TimeoutSecs != nil {
		record.Set("timeout_secs", *req.TimeoutSecs)
	}
	if err := pb.Save(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, scriptRecordToMap(record))
}

// deleteScript removes a script by id.
func deleteScript(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	record, err := pb.FindRecordById("scripts", id)
	if err != nil {
		return apis.NewNotFoundError("script not found", nil)
	}
	if err := pb.Delete(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "script deleted"})
}

// postScriptRun executes a script in the goja sandbox and stores the result.
func postScriptRun(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	record, err := pb.FindRecordById("scripts", id)
	if err != nil {
		return apis.NewNotFoundError("script not found", nil)
	}

	code := record.GetString("code")
	timeoutSecs := record.GetFloat("timeout_secs")
	if timeoutSecs <= 0 {
		timeoutSecs = 10
	}
	timeout := time.Duration(timeoutSecs * float64(time.Second))

	output, runErr, duration := runScript(code, timeout)

	status := "success"
	errMsg := ""
	if runErr != nil {
		status = "error"
		errMsg = runErr.Error()
	}

	record.Set("last_run_status", status)
	record.Set("last_run_output", output)
	record.Set("last_run_duration_ms", float64(duration.Milliseconds()))
	record.Set("last_run_error", errMsg)
	_ = pb.Save(record)

	return e.JSON(http.StatusOK, map[string]any{
		"status":      status,
		"output":      output,
		"duration_ms": duration.Milliseconds(),
		"error":       errMsg,
	})
}

// postScriptRunAdhoc executes ad-hoc code (not persisted) and returns the result.
// Body: {"code", "timeout_secs"}
func postScriptRunAdhoc(e *core.RequestEvent) error {
	var req struct {
		Code        string  `json:"code"`
		TimeoutSecs float64 `json:"timeout_secs"`
	}
	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return apis.NewBadRequestError("invalid body", err)
	}
	if req.Code == "" {
		return apis.NewBadRequestError("code is required", nil)
	}
	if req.TimeoutSecs <= 0 {
		req.TimeoutSecs = 10
	}
	timeout := time.Duration(req.TimeoutSecs * float64(time.Second))

	output, runErr, duration := runScript(req.Code, timeout)

	status := "success"
	errMsg := ""
	if runErr != nil {
		status = "error"
		errMsg = runErr.Error()
	}

	return e.JSON(http.StatusOK, map[string]any{
		"status":      status,
		"output":      output,
		"duration_ms": duration.Milliseconds(),
		"error":       errMsg,
	})
}

// ── sandbox ────────────────────────────────────────────────────────────────────

// runScript executes JS code in an isolated goja VM and returns (output, error, duration).
// The sandbox exposes: console.log, wa.sendText, wa.status, http.get, http.post, db.query, sleep.
func runScript(code string, timeout time.Duration) (output string, runErr error, duration time.Duration) {
	vm := goja.New()

	// Interrupt after timeout
	done := make(chan struct{})
	timer := time.AfterFunc(timeout, func() {
		vm.Interrupt("script timeout")
		close(done)
	})
	defer func() {
		timer.Stop()
		select {
		case <-done:
		default:
		}
	}()

	// ── console ──────────────────────────────────────────────────────────────
	var logs []string
	consoleObj := vm.NewObject()
	_ = consoleObj.Set("log", func(call goja.FunctionCall) goja.Value {
		parts := make([]string, len(call.Arguments))
		for i, a := range call.Arguments {
			parts[i] = a.String()
		}
		line := ""
		for i, p := range parts {
			if i > 0 {
				line += " "
			}
			line += p
		}
		logs = append(logs, line)
		return goja.Undefined()
	})
	_ = consoleObj.Set("error", consoleObj.Get("log"))
	_ = consoleObj.Set("warn", consoleObj.Get("log"))
	_ = vm.Set("console", consoleObj)

	// ── wa ────────────────────────────────────────────────────────────────────
	waObj := vm.NewObject()
	_ = waObj.Set("sendText", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.ToValue("wa.sendText(jid, text) requires 2 arguments"))
		}
		jidStr := call.Arguments[0].String()
		text := call.Arguments[1].String()
		jid, ok := whatsapp.ParseJID(jidStr)
		if !ok {
			panic(vm.ToValue(fmt.Sprintf("wa.sendText: invalid JID %q", jidStr)))
		}
		_, _, err := whatsapp.SendConversationMessage(jid, text, nil)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("wa.sendText error: %v", err)))
		}
		return goja.Undefined()
	})
	_ = waObj.Set("status", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(string(whatsapp.GetConnectionStatus()))
	})
	_ = vm.Set("wa", waObj)

	// ── http ─────────────────────────────────────────────────────────────────
	httpObj := vm.NewObject()
	_ = httpObj.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.ToValue("http.get(url) requires 1 argument"))
		}
		url := call.Arguments[0].String()
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Get(url) //nolint:noctx
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("http.get error: %v", err)))
		}
		defer resp.Body.Close()
		var buf [1 << 16]byte // 64 KB cap
		n, _ := resp.Body.Read(buf[:])
		result := vm.NewObject()
		_ = result.Set("status", resp.StatusCode)
		_ = result.Set("body", string(buf[:n]))
		return result
	})
	_ = httpObj.Set("post", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.ToValue("http.post(url, body) requires 2 arguments"))
		}
		postURL := call.Arguments[0].String()
		bodyStr := call.Arguments[1].String()
		client := &http.Client{Timeout: 15 * time.Second}
		req, reqErr := http.NewRequest(http.MethodPost, postURL, strings.NewReader(bodyStr))
		if reqErr != nil {
			panic(vm.ToValue(fmt.Sprintf("http.post error: %v", reqErr)))
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("http.post error: %v", err)))
		}
		defer resp.Body.Close()
		var buf [1 << 16]byte
		n, _ := resp.Body.Read(buf[:])
		result := vm.NewObject()
		_ = result.Set("status", resp.StatusCode)
		_ = result.Set("body", string(buf[:n]))
		return result
	})
	_ = vm.Set("http", httpObj)

	// ── db ────────────────────────────────────────────────────────────────────
	dbObj := vm.NewObject()
	_ = dbObj.Set("query", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.ToValue("db.query(sql) requires 1 argument"))
		}
		sql := call.Arguments[0].String()
		type anyRow map[string]any
		var rows []anyRow
		if err := pb.DB().NewQuery(sql).All(&rows); err != nil {
			panic(vm.ToValue(fmt.Sprintf("db.query error: %v", err)))
		}
		val, _ := json.Marshal(rows)
		var out any
		_ = json.Unmarshal(val, &out)
		return vm.ToValue(out)
	})
	_ = vm.Set("db", dbObj)

	// ── sleep ─────────────────────────────────────────────────────────────────
	_ = vm.Set("sleep", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		ms := call.Arguments[0].ToFloat()
		if ms > 5000 {
			ms = 5000 // cap at 5 s per sleep call
		}
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return goja.Undefined()
	})

	start := time.Now()
	_, runErr = vm.RunString(code)
	duration = time.Since(start)

	if len(logs) > 0 {
		for _, l := range logs {
			if output != "" {
				output += "\n"
			}
			output += l
		}
	}
	return output, runErr, duration
}

// ── helpers ────────────────────────────────────────────────────────────────────

func scriptRecordToMap(r *core.Record) map[string]any {
	return map[string]any{
		"id":                   r.Id,
		"name":                 r.GetString("name"),
		"description":          r.GetString("description"),
		"code":                 r.GetString("code"),
		"enabled":              r.GetBool("enabled"),
		"timeout_secs":         r.GetFloat("timeout_secs"),
		"last_run_status":      r.GetString("last_run_status"),
		"last_run_output":      r.GetString("last_run_output"),
		"last_run_duration_ms": r.GetFloat("last_run_duration_ms"),
		"last_run_error":       r.GetString("last_run_error"),
		"created":              r.GetDateTime("created").String(),
		"updated":              r.GetDateTime("updated").String(),
	}
}
