package api

import (
	"net/http"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// getFramesIQ returns frames that contain IQ stanzas (raw contains <iq).
// GET /zaplab/api/frames/iq?direction=&iqtype=&limit=200
func getFramesIQ(e *core.RequestEvent) error {
	q := e.Request.URL.Query()
	direction := q.Get("direction") // in / out / ""
	iqType := q.Get("iqtype")       // get / set / result / error / ""
	limit := 200

	type frameRow struct {
		ID        string `db:"id"        json:"id"`
		Module    string `db:"module"    json:"module"`
		Direction string `db:"direction" json:"direction"`
		Raw       string `db:"raw"       json:"raw"`
		Created   string `db:"created"   json:"created"`
	}

	var exprs []dbx.Expression
	exprs = append(exprs, dbx.NewExp("raw LIKE {:iq}", dbx.Params{"iq": "%<iq%"}))

	if direction != "" {
		exprs = append(exprs, dbx.HashExp{"direction": strings.ToLower(direction)})
	}
	if iqType != "" {
		t := strings.ToLower(iqType)
		exprs = append(exprs, dbx.NewExp("raw LIKE {:iqt}", dbx.Params{"iqt": "%type=\"" + t + "\"%"}))
	}

	var rows []frameRow
	err := pb.DB().Select("id", "module", "direction", "raw", "created").
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

// getFramesBinary returns raw binary frames (Noise/Socket modules).
// GET /zaplab/api/frames/binary?direction=&module=&limit=200
func getFramesBinary(e *core.RequestEvent) error {
	q := e.Request.URL.Query()
	direction := q.Get("direction")
	module := q.Get("module")
	limit := 200

	type frameRow struct {
		ID        string `db:"id"        json:"id"`
		Module    string `db:"module"    json:"module"`
		Direction string `db:"direction" json:"direction"`
		Raw       string `db:"raw"       json:"raw"`
		Size      int64  `db:"size"      json:"size"`
		Created   string `db:"created"   json:"created"`
	}

	var exprs []dbx.Expression
	exprs = append(exprs, dbx.NewExp("module IN ('Noise','Socket','noise','socket')"))

	if direction != "" {
		exprs = append(exprs, dbx.HashExp{"direction": strings.ToLower(direction)})
	}
	if module != "" {
		exprs = append(exprs, dbx.HashExp{"module": module})
	}

	var rows []frameRow
	err := pb.DB().Select("id", "module", "direction", "raw",
		"COALESCE(LENGTH(raw),0) as size", "created").
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
