package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/lichti/zaplab/internal/whatsapp"
	"github.com/pocketbase/pocketbase/core"
	"go.mau.fi/whatsmeow/types"
)

// getContactOverview returns a rich analytics dashboard for a single contact JID.
//
// GET /zaplab/api/contacts/{jid}/overview?period=30
//
// period: days to look back (default 30, 0 = all time)
func getContactOverview(e *core.RequestEvent) error {
	jid := e.Request.PathValue("jid")
	if jid == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "jid is required"})
	}
	jid = sanitizeSQL(jid)

	period, _ := strconv.Atoi(e.Request.URL.Query().Get("period"))
	if period < 0 || period > 365 {
		period = 30
	}

	periodClause := ""
	if period > 0 {
		periodClause = fmt.Sprintf("AND datetime(created) >= datetime('now', '-%d days')", period)
	}

	// ── Display name + full contact info from whatsmeow_contacts ─────────────
	displayName := jid
	if at := strings.Index(jid, "@"); at > 0 {
		displayName = jid[:at]
	}
	var fullName, pushName, businessName string

	// Resolve the LID↔PN pair. The lid_map table stores only the User part (no @server).
	// primaryJID  = canonical PN (preferred for queries, most events use this after migration)
	// secondaryJID = the other form (LID or old PN) — may be empty if mapping unknown
	primaryJID := jid
	secondaryJID := ""
	if waDB != nil {
		if strings.HasSuffix(jid, "@lid") {
			lidUser := strings.TrimSuffix(jid, "@lid")
			var pnUser string
			if waDB.QueryRow(`SELECT pn FROM whatsmeow_lid_map WHERE lid = ?`, lidUser).Scan(&pnUser) == nil && pnUser != "" {
				primaryJID = pnUser + "@s.whatsapp.net"
				secondaryJID = jid // keep LID as secondary
			}
		} else if strings.Contains(jid, "@s.whatsapp.net") {
			pnUser := strings.TrimSuffix(jid, "@s.whatsapp.net")
			var lidUser string
			if waDB.QueryRow(`SELECT lid FROM whatsmeow_lid_map WHERE pn = ?`, pnUser).Scan(&lidUser) == nil && lidUser != "" {
				secondaryJID = lidUser + "@lid"
			}
		}
	}
	// altJID is the non-primary form, shown in the UI.
	altJID := secondaryJID

	if waDB != nil {
		row := waDB.QueryRow(
			`SELECT COALESCE(NULLIF(push_name,''), NULLIF(full_name,''), their_jid),
			        COALESCE(full_name,''), COALESCE(push_name,''), COALESCE(business_name,'')
			 FROM whatsmeow_contacts WHERE their_jid = ?`, primaryJID)
		if row.Scan(&displayName, &fullName, &pushName, &businessName) != nil {
			// fallback: try original jid in case primaryJID not in contacts
			row2 := waDB.QueryRow(
				`SELECT COALESCE(NULLIF(push_name,''), NULLIF(full_name,''), their_jid),
				        COALESCE(full_name,''), COALESCE(push_name,''), COALESCE(business_name,'')
				 FROM whatsmeow_contacts WHERE their_jid = ?`, jid)
			_ = row2.Scan(&displayName, &fullName, &pushName, &businessName)
		}
	}

	// jidIn builds  = 'primary'  or  IN ('primary','secondary')  for SQL WHERE clauses.
	jidIn := func(col string) string {
		if secondaryJID == "" {
			return fmt.Sprintf("%s = '%s'", col, primaryJID)
		}
		return fmt.Sprintf("%s IN ('%s','%s')", col, primaryJID, secondaryJID)
	}

	// Group data: names map + full list for silent-membership detection.
	// GetJoinedGroups returns full GroupInfo including Participants (with both JID and LID).
	groupNames := map[string]string{}
	var joinedGroups []*types.GroupInfo
	if groups, err := whatsapp.GetJoinedGroups(); err == nil {
		joinedGroups = groups
		for _, g := range groups {
			if g.Name != "" {
				groupNames[g.JID.String()] = g.Name
			}
		}
	}

	// ── DM stats ──────────────────────────────────────────────────────────────
	type dmRow struct {
		Total    int `db:"total"`
		Received int `db:"received"`
		Sent     int `db:"sent"`
		MediaDM  int `db:"media_dm"`
	}
	var dm dmRow
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT COUNT(*) AS total,
		       SUM(CASE WHEN json_extract(raw,'$.Info.IsFromMe')=0 THEN 1 ELSE 0 END) AS received,
		       SUM(CASE WHEN json_extract(raw,'$.Info.IsFromMe')=1 THEN 1 ELSE 0 END) AS sent,
		       SUM(CASE WHEN file != '' THEN 1 ELSE 0 END) AS media_dm
		FROM events
		WHERE type LIKE '%%Message%%'
		  AND %s
		  %s`, jidIn("json_extract(raw,'$.Info.Chat')"), periodClause)).One(&dm)

	// ── Group activity ────────────────────────────────────────────────────────
	type grpRow struct {
		GroupMsgs  int `db:"group_msgs"`
		GroupMedia int `db:"group_media"`
	}
	var grp grpRow
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT COUNT(*) AS group_msgs,
		       SUM(CASE WHEN file != '' THEN 1 ELSE 0 END) AS group_media
		FROM events
		WHERE type LIKE '%%Message%%'
		  AND %s
		  AND json_extract(raw,'$.Info.IsGroup') = 1
		  %s`, jidIn("json_extract(raw,'$.Info.Sender')"), periodClause)).One(&grp)

	// ── Edit / delete / reactions ─────────────────────────────────────────────
	type editReactRow struct {
		Edited            int `db:"edited"`
		Deleted           int `db:"deleted"`
		ReactionsReceived int `db:"reactions_received"`
		ReactionsSent     int `db:"reactions_sent"`
	}
	var er editReactRow
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT SUM(CASE WHEN raw LIKE '%%"Edit":"1"%%' AND json_extract(raw,'$.Info.IsFromMe')=0 THEN 1 ELSE 0 END) AS edited,
		       SUM(CASE WHEN (raw LIKE '%%"Edit":"7"%%' OR raw LIKE '%%"Edit":"8"%%') AND json_extract(raw,'$.Info.IsFromMe')=0 THEN 1 ELSE 0 END) AS deleted,
		       SUM(CASE WHEN raw LIKE '%%"ReactionMessage":{%%' AND json_extract(raw,'$.Info.IsFromMe')=0 THEN 1 ELSE 0 END) AS reactions_received,
		       SUM(CASE WHEN raw LIKE '%%"ReactionMessage":{%%' AND json_extract(raw,'$.Info.IsFromMe')=1 THEN 1 ELSE 0 END) AS reactions_sent
		FROM events
		WHERE type LIKE '%%Message%%'
		  AND %s
		  %s`, jidIn("json_extract(raw,'$.Info.Chat')"), periodClause)).One(&er)

	// ── Common groups ──────────────────────────────────────────────────────────
	type groupRow struct {
		GroupJID string `db:"group_jid" json:"jid"`
		MsgCount int    `db:"msg_count" json:"msg_count"`
		Name     string `json:"name"`
	}
	var commonGroups []groupRow
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT json_extract(raw,'$.Info.Chat') AS group_jid, COUNT(*) AS msg_count
		FROM events
		WHERE type LIKE '%%Message%%'
		  AND %s
		  AND json_extract(raw,'$.Info.Chat') LIKE '%%@g.us'
		  %s
		GROUP BY group_jid
		ORDER BY msg_count DESC
		LIMIT 15`, jidIn("json_extract(raw,'$.Info.Sender')"), periodClause)).All(&commonGroups)
	if commonGroups == nil {
		commonGroups = []groupRow{}
	}
	for i := range commonGroups {
		gJID := commonGroups[i].GroupJID
		// Default: numeric prefix of JID
		name := gJID
		if at := strings.Index(name, "@"); at > 0 {
			name = name[:at]
		}
		// 1st choice: group_membership recorded name
		type nameRow struct {
			N string `db:"group_name"`
		}
		var nr nameRow
		if err := pb.DB().NewQuery(fmt.Sprintf(
			`SELECT group_name FROM group_membership WHERE group_jid='%s' AND group_name!='' ORDER BY created DESC LIMIT 1`,
			sanitizeSQL(gJID))).One(&nr); err == nil && nr.N != "" {
			name = nr.N
		} else if n, ok := groupNames[gJID]; ok {
			// 2nd choice: live joined-groups list
			name = n
		} else if groupJID, err := types.ParseJID(gJID); err == nil {
			// 3rd choice: direct group info API call
			if info, err := whatsapp.GetGroupInfo(groupJID); err == nil && info.Name != "" {
				name = info.Name
			}
		}
		commonGroups[i].Name = name
	}

	// ── Silent memberships: groups where the contact is a member but sent no messages ──
	// GetJoinedGroups already returns full participant lists (including LID).
	// Check both primaryJID and secondaryJID against each participant's JID and LID.
	if len(joinedGroups) > 0 {
		knownGroups := map[string]bool{}
		for _, g := range commonGroups {
			knownGroups[g.GroupJID] = true
		}
		for _, jg := range joinedGroups {
			gjid := jg.JID.String()
			if knownGroups[gjid] {
				continue
			}
			isMember := false
			for _, p := range jg.Participants {
				pjid := p.JID.String()
				if pjid == primaryJID || pjid == secondaryJID {
					isMember = true
					break
				}
				if !p.LID.IsEmpty() {
					plid := p.LID.String()
					if plid == primaryJID || plid == secondaryJID {
						isMember = true
						break
					}
				}
			}
			if !isMember {
				continue
			}
			name := gjid
			if at := strings.Index(name, "@"); at > 0 {
				name = name[:at]
			}
			if jg.Name != "" {
				name = jg.Name
			}
			commonGroups = append(commonGroups, groupRow{GroupJID: gjid, MsgCount: 0, Name: name})
		}
	}

	// ── Heatmap ───────────────────────────────────────────────────────────────
	type heatCell struct {
		DOW   string `db:"dow"  json:"dow"`
		Hour  string `db:"hour" json:"hour"`
		Count int    `db:"cnt"  json:"count"`
	}
	var heatmap []heatCell
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT strftime('%%w', datetime(created)) AS dow,
		       strftime('%%H', datetime(created)) AS hour,
		       COUNT(*) AS cnt
		FROM events
		WHERE type LIKE '%%Message%%'
		  AND (
		    (%s AND json_extract(raw,'$.Info.IsFromMe')=0)
		    OR
		    (%s AND json_extract(raw,'$.Info.IsGroup')=1)
		  )
		  %s
		GROUP BY dow, hour
		ORDER BY dow, hour`,
		jidIn("json_extract(raw,'$.Info.Chat')"),
		jidIn("json_extract(raw,'$.Info.Sender')"),
		periodClause)).All(&heatmap)
	if heatmap == nil {
		heatmap = []heatCell{}
	}

	// ── Peak activity slot ────────────────────────────────────────────────────
	type peakRow struct {
		DOW   string `db:"dow"`
		Hour  string `db:"hour"`
		Count int    `db:"cnt"`
	}
	var peak peakRow
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT strftime('%%w', datetime(created)) AS dow,
		       strftime('%%H', datetime(created)) AS hour,
		       COUNT(*) AS cnt
		FROM events
		WHERE type LIKE '%%Message%%'
		  AND (
		    (%s AND json_extract(raw,'$.Info.IsFromMe')=0)
		    OR
		    (%s AND json_extract(raw,'$.Info.IsGroup')=1)
		  )
		  %s
		GROUP BY dow, hour
		ORDER BY cnt DESC LIMIT 1`,
		jidIn("json_extract(raw,'$.Info.Chat')"),
		jidIn("json_extract(raw,'$.Info.Sender')"),
		periodClause)).One(&peak)

	// ── Daily sparkline ───────────────────────────────────────────────────────
	type dayRow struct {
		Day   string `db:"day" json:"day"`
		Count int    `db:"cnt" json:"count"`
	}
	dailyDays := period
	if dailyDays == 0 || dailyDays > 90 {
		dailyDays = 90
	}
	var daily []dayRow
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT strftime('%%Y-%%m-%%d', datetime(created)) AS day, COUNT(*) AS cnt
		FROM events
		WHERE type LIKE '%%Message%%'
		  AND (
		    (%s AND json_extract(raw,'$.Info.IsFromMe')=0)
		    OR
		    (%s AND json_extract(raw,'$.Info.IsGroup')=1)
		  )
		  AND datetime(created) >= datetime('now', '-%d days')
		GROUP BY day ORDER BY day ASC`,
		jidIn("json_extract(raw,'$.Info.Chat')"),
		jidIn("json_extract(raw,'$.Info.Sender')"),
		dailyDays)).All(&daily)
	if daily == nil {
		daily = []dayRow{}
	}

	// ── Recent presence events ─────────────────────────────────────────────────
	type presRow struct {
		Type    string `db:"type"    json:"type"`
		Created string `db:"created" json:"created"`
	}
	var presence []presRow
	presenceFilter := fmt.Sprintf("raw LIKE '%%%s%%'", primaryJID)
	if secondaryJID != "" {
		presenceFilter = fmt.Sprintf("(raw LIKE '%%%s%%' OR raw LIKE '%%%s%%')", primaryJID, secondaryJID)
	}
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT type, created FROM events
		WHERE (type LIKE 'Presence.%%' OR type LIKE 'ChatPresence.%%')
		  AND %s
		ORDER BY created DESC LIMIT 15`, presenceFilter)).All(&presence)
	if presence == nil {
		presence = []presRow{}
	}

	// ── Last online ────────────────────────────────────────────────────────────
	type lastOnlineRow struct {
		T string `db:"created"`
	}
	var lastOnline lastOnlineRow
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT created FROM events
		WHERE type = 'Presence.Online' AND %s
		ORDER BY created DESC LIMIT 1`, presenceFilter)).One(&lastOnline)

	// ── First and last activity ────────────────────────────────────────────────
	type flRow struct {
		First string `db:"first_seen"`
		Last  string `db:"last_seen"`
	}
	var fl flRow
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT MIN(created) AS first_seen, MAX(created) AS last_seen
		FROM events
		WHERE type LIKE '%%Message%%'
		  AND (%s OR %s)`,
		jidIn("json_extract(raw,'$.Info.Chat')"),
		jidIn("json_extract(raw,'$.Info.Sender')"))).One(&fl)

	// ── Peak label ────────────────────────────────────────────────────────────
	dowNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	peakLabel := ""
	if peak.Count > 0 {
		dowIdx := 0
		fmt.Sscanf(peak.DOW, "%d", &dowIdx)
		if dowIdx >= 0 && dowIdx < 7 {
			peakLabel = fmt.Sprintf("%s at %sh", dowNames[dowIdx], peak.Hour)
		}
	}

	var mostActiveGroup any
	if len(commonGroups) > 0 {
		mostActiveGroup = commonGroups[0]
	}

	return e.JSON(http.StatusOK, map[string]any{
		"jid":           jid,
		"alt_jid":       altJID,
		"display_name":  displayName,
		"full_name":     fullName,
		"push_name":     pushName,
		"business_name": businessName,
		"summary": map[string]any{
			"total_messages":     dm.Total + grp.GroupMsgs,
			"dm_total":           dm.Total,
			"received":           dm.Received,
			"sent":               dm.Sent,
			"group_messages":     grp.GroupMsgs,
			"media_dm":           dm.MediaDM,
			"media_groups":       grp.GroupMedia,
			"media_total":        dm.MediaDM + grp.GroupMedia,
			"edited":             er.Edited,
			"deleted":            er.Deleted,
			"reactions_received": er.ReactionsReceived,
			"reactions_sent":     er.ReactionsSent,
		},
		"peak_activity":     peakLabel,
		"common_groups":     commonGroups,
		"most_active_group": mostActiveGroup,
		"heatmap":           heatmap,
		"daily":             daily,
		"presence":          presence,
		"last_online":       lastOnline.T,
		"first_seen":        fl.First,
		"last_seen":         fl.Last,
		"period":            period,
	})
}
