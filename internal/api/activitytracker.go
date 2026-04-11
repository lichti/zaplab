package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/lichti/zaplab/internal/whatsapp"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	watypes "go.mau.fi/whatsmeow/types"
)

// postActivityTrackerEnable turns the feature flag on.
func postActivityTrackerEnable(e *core.RequestEvent) error {
	if err := cfg.SetActivityTrackerEnabled(true); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"enabled": true})
}

// postActivityTrackerDisable turns the feature flag off and stops all running trackers.
func postActivityTrackerDisable(e *core.RequestEvent) error {
	whatsapp.StopAllTrackers()
	if err := cfg.SetActivityTrackerEnabled(false); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"enabled": false})
}

// postActivityTrackerStart begins tracking a JID.
// Body: {"jid": "5511999999999", "probe_method": "delete"}
func postActivityTrackerStart(e *core.RequestEvent) error {
	if !cfg.IsActivityTrackerEnabled() {
		return e.JSON(http.StatusForbidden, map[string]any{"error": "activity tracker is disabled"})
	}
	var req struct {
		JID         string `json:"jid"`
		ProbeMethod string `json:"probe_method"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.JID == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "jid is required"})
	}
	jid, ok := whatsapp.ParseJID(req.JID)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "invalid jid"})
	}
	if req.ProbeMethod == "" {
		req.ProbeMethod = "delete"
	}
	sessionID, err := whatsapp.StartTracker(jid, req.ProbeMethod)
	if err != nil {
		return e.JSON(http.StatusConflict, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"jid":          jid.String(),
		"session_id":   sessionID,
		"probe_method": req.ProbeMethod,
	})
}

// postActivityTrackerStop stops tracking a JID.
// Body: {"jid": "5511999999999"}
func postActivityTrackerStop(e *core.RequestEvent) error {
	var req struct {
		JID string `json:"jid"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.JID == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "jid is required"})
	}
	jid, ok := whatsapp.ParseJID(req.JID)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "invalid jid"})
	}
	if err := whatsapp.StopTracker(jid); err != nil {
		return e.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"jid": jid.String(), "stopped": true})
}

// getActivityTrackerStatus returns the feature flag state and all active trackers.
func getActivityTrackerStatus(e *core.RequestEvent) error {
	return e.JSON(http.StatusOK, map[string]any{
		"enabled":  cfg.IsActivityTrackerEnabled(),
		"trackers": whatsapp.GetTrackerStatus(),
	})
}

// getActivityTrackerContacts returns the list of known individual contacts enriched
// with their current tracking state. Groups (g.us) are excluded.
func getActivityTrackerContacts(e *core.RequestEvent) error {
	contacts, err := whatsapp.GetAllContacts()
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}

	// Build a set of currently tracked JIDs → state for O(1) lookup.
	trackerStatus := whatsapp.GetTrackerStatus()
	trackedMap := make(map[string]string, len(trackerStatus))
	for _, t := range trackerStatus {
		trackedMap[t["jid"].(string)] = t["state"].(string)
	}

	type contactRow struct {
		JID      string `json:"jid"`
		Phone    string `json:"phone"`
		Name     string `json:"name"`
		Tracking bool   `json:"tracking"`
		State    string `json:"state,omitempty"`
	}

	rows := make([]contactRow, 0, len(contacts))
	for _, c := range contacts {
		if strings.HasSuffix(c.JID, "@g.us") || strings.HasSuffix(c.JID, "@broadcast") {
			continue
		}
		name := c.FullName
		if name == "" {
			name = c.PushName
		}
		if name == "" {
			name = c.BusinessName
		}
		state, tracking := trackedMap[c.JID]
		rows = append(rows, contactRow{
			JID:      c.JID,
			Phone:    c.Phone,
			Name:     name,
			Tracking: tracking,
			State:    state,
		})
	}

	return e.JSON(http.StatusOK, map[string]any{"contacts": rows, "total": len(rows)})
}

// postActivityTrackerStartBulk starts trackers for a list of JIDs at once.
// Body: {"jids": ["5511999...", ...], "probe_method": "delete"}
func postActivityTrackerStartBulk(e *core.RequestEvent) error {
	if !cfg.IsActivityTrackerEnabled() {
		return e.JSON(http.StatusForbidden, map[string]any{"error": "activity tracker is disabled"})
	}
	var req struct {
		JIDs        []string `json:"jids"`
		ProbeMethod string   `json:"probe_method"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if len(req.JIDs) == 0 {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "jids is required"})
	}
	if req.ProbeMethod == "" {
		req.ProbeMethod = "delete"
	}
	var parsed []watypes.JID
	for _, raw := range req.JIDs {
		jid, ok := whatsapp.ParseJID(raw)
		if ok {
			parsed = append(parsed, jid)
		}
	}
	started, skipped, failed := whatsapp.StartTrackersBulk(parsed, req.ProbeMethod)
	return e.JSON(http.StatusOK, map[string]any{
		"started": started,
		"skipped": skipped,
		"failed":  failed,
	})
}

// postActivityTrackerStopAll stops all active trackers without disabling the feature flag.
func postActivityTrackerStopAll(e *core.RequestEvent) error {
	whatsapp.StopAllTrackers()
	return e.JSON(http.StatusOK, map[string]any{"stopped": true})
}

// getActivityTrackerHistory returns persisted probes for a JID.
// Query params: jid (required), limit (default 200, max 2000), days (default 0 = all)
func getActivityTrackerHistory(e *core.RequestEvent) error {
	jidStr := e.Request.URL.Query().Get("jid")
	if jidStr == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "jid is required"})
	}
	limit := 200
	if v := e.Request.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 2000 {
			limit = n
		}
	}
	days := 0
	if v := e.Request.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}

	type probeRow struct {
		ID        string  `db:"id"           json:"id"`
		SessionID string  `db:"session_id"   json:"session_id"`
		JID       string  `db:"jid"          json:"jid"`
		RTTms     float64 `db:"rtt_ms"       json:"rtt_ms"`
		State     string  `db:"state"        json:"state"`
		Median    float64 `db:"median_ms"    json:"median_ms"`
		Threshold float64 `db:"threshold_ms" json:"threshold_ms"`
		Created   string  `db:"created"      json:"created"`
	}

	q := pb.DB().Select("id", "session_id", "jid", "rtt_ms", "state", "median_ms", "threshold_ms", "created").
		From("device_activity_probes").
		Where(dbx.HashExp{"jid": jidStr}).
		OrderBy("created DESC").
		Limit(int64(limit))

	if days > 0 {
		q = q.AndWhere(dbx.NewExp("created >= datetime('now', {:d})",
			dbx.Params{"d": "-" + strconv.Itoa(days) + " days"}))
	}

	var rows []probeRow
	if err := q.All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []probeRow{}
	}
	return e.JSON(http.StatusOK, map[string]any{"probes": rows, "total": len(rows)})
}
