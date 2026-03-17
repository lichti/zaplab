package api

import (
	"net/http"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// getFramesIQ returns frames that contain IQ stanzas (msg contains <iq).
// GET /zaplab/api/frames/iq?level=&iqtype=&limit=200
func getFramesIQ(e *core.RequestEvent) error {
	q := e.Request.URL.Query()
	level := q.Get("level")   // debug / info / warn / error / ""
	iqType := q.Get("iqtype") // get / set / result / error / ""
	limit := 200

	type frameRow struct {
		ID      string `db:"id"      json:"id"`
		Module  string `db:"module"  json:"module"`
		Level   string `db:"level"   json:"level"`
		Seq     string `db:"seq"     json:"seq"`
		Msg     string `db:"msg"     json:"msg"`
		Created string `db:"created" json:"created"`
	}

	var exprs []dbx.Expression
	exprs = append(exprs, dbx.NewExp("msg LIKE {:iq}", dbx.Params{"iq": "%<iq%"}))

	if level != "" {
		exprs = append(exprs, dbx.HashExp{"level": strings.ToLower(level)})
	}
	if iqType != "" {
		t := strings.ToLower(iqType)
		exprs = append(exprs, dbx.NewExp("msg LIKE {:iqt}", dbx.Params{"iqt": "%type=\"" + t + "\"%"}))
	}

	var rows []frameRow
	err := pb.DB().Select("id", "module", "level", "seq", "msg", "created").
		From("frames").
		Where(dbx.And(exprs...)).
		OrderBy("created DESC").
		Limit(int64(limit)).
		All(&rows)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []frameRow{}
	}
	return e.JSON(http.StatusOK, map[string]any{"frames": rows, "total": len(rows)})
}

// getFramesBinary returns frames from Noise/Socket modules (binary protocol layer).
// GET /zaplab/api/frames/binary?level=&module=&limit=200
func getFramesBinary(e *core.RequestEvent) error {
	q := e.Request.URL.Query()
	level := q.Get("level")
	module := q.Get("module")
	limit := 200

	type frameRow struct {
		ID      string `db:"id"      json:"id"`
		Module  string `db:"module"  json:"module"`
		Level   string `db:"level"   json:"level"`
		Seq     string `db:"seq"     json:"seq"`
		Msg     string `db:"msg"     json:"msg"`
		Size    int64  `db:"size"    json:"size"`
		Created string `db:"created" json:"created"`
	}

	var exprs []dbx.Expression
	exprs = append(exprs, dbx.NewExp("module IN ('Noise','Socket','noise','socket')"))

	if level != "" {
		exprs = append(exprs, dbx.HashExp{"level": strings.ToLower(level)})
	}
	if module != "" {
		exprs = append(exprs, dbx.HashExp{"module": module})
	}

	var rows []frameRow
	err := pb.DB().Select("id", "module", "level", "seq",
		"msg", "COALESCE(LENGTH(msg),0) as size", "created").
		From("frames").
		Where(dbx.And(exprs...)).
		OrderBy("created DESC").
		Limit(int64(limit)).
		All(&rows)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []frameRow{}
	}
	return e.JSON(http.StatusOK, map[string]any{"frames": rows, "total": len(rows)})
}
