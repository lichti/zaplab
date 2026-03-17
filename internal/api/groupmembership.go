package api

import (
	"net/http"
	"strconv"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// getGroupMembershipHistory returns membership change history for a group.
// GET /zaplab/api/groups/{jid}/history?limit=200
func getGroupMembershipHistory(e *core.RequestEvent) error {
	jid := e.Request.PathValue("jid")
	limit := 200
	if v := e.Request.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	type memberRow struct {
		ID        string `db:"id"         json:"id"`
		GroupJID  string `db:"group_jid"  json:"group_jid"`
		GroupName string `db:"group_name" json:"group_name"`
		MemberJID string `db:"member_jid" json:"member_jid"`
		Action    string `db:"action"     json:"action"`
		ActorJID  string `db:"actor_jid"  json:"actor_jid"`
		Created   string `db:"created"    json:"created"`
	}

	var rows []memberRow
	err := pb.DB().Select("id", "group_jid", "group_name", "member_jid", "action", "actor_jid", "created").
		From("group_membership").
		Where(dbx.HashExp{"group_jid": jid}).
		OrderBy("created DESC").
		Limit(int64(limit)).
		All(&rows)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []memberRow{}
	}
	return e.JSON(http.StatusOK, map[string]any{"history": rows, "total": len(rows)})
}

// getGroupMembershipAll returns membership change history across all groups.
// GET /zaplab/api/groups/membership?limit=200&action=
func getGroupMembershipAll(e *core.RequestEvent) error {
	limit := 200
	action := e.Request.URL.Query().Get("action")
	if v := e.Request.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	type memberRow struct {
		ID        string `db:"id"         json:"id"`
		GroupJID  string `db:"group_jid"  json:"group_jid"`
		GroupName string `db:"group_name" json:"group_name"`
		MemberJID string `db:"member_jid" json:"member_jid"`
		Action    string `db:"action"     json:"action"`
		ActorJID  string `db:"actor_jid"  json:"actor_jid"`
		Created   string `db:"created"    json:"created"`
	}

	sel := pb.DB().Select("id", "group_jid", "group_name", "member_jid", "action", "actor_jid", "created").
		From("group_membership").
		OrderBy("created DESC").
		Limit(int64(limit))

	if action != "" {
		sel = sel.Where(dbx.HashExp{"action": action})
	}

	var rows []memberRow
	if err := sel.All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []memberRow{}
	}
	return e.JSON(http.StatusOK, map[string]any{"history": rows, "total": len(rows)})
}
