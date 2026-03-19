package api

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/lichti/zaplab/internal/whatsapp"
	"github.com/pocketbase/pocketbase/core"
	"go.mau.fi/whatsmeow/types"
)

// getGroupOverview returns a rich analytics dashboard for a single group JID.
//
// GET /zaplab/api/groups/{jid}/overview?period=30
//
// period: days to look back (default 30, 0 = all time)
func getGroupOverview(e *core.RequestEvent) error {
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

	// ── Group name ────────────────────────────────────────────────────────────
	groupName := jid
	if at := strings.Index(jid, "@"); at > 0 {
		groupName = jid[:at]
	}
	type nameRow struct {
		N string `db:"group_name"`
	}
	var nr nameRow
	if err := pb.DB().NewQuery(fmt.Sprintf(
		`SELECT group_name FROM group_membership WHERE group_jid='%s' AND group_name!='' ORDER BY created DESC LIMIT 1`,
		jid)).One(&nr); err == nil && nr.N != "" {
		groupName = nr.N
	}

	// ── Summary stats ─────────────────────────────────────────────────────────
	type sumRow struct {
		Total     int `db:"total"`
		Media     int `db:"media"`
		Edited    int `db:"edited"`
		Deleted   int `db:"deleted"`
		Reactions int `db:"reactions"`
	}
	var sum sumRow
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT COUNT(*) AS total,
		       SUM(CASE WHEN file != '' THEN 1 ELSE 0 END) AS media,
		       SUM(CASE WHEN raw LIKE '%%"Edit":"1"%%' THEN 1 ELSE 0 END) AS edited,
		       SUM(CASE WHEN raw LIKE '%%"Edit":"7"%%' OR raw LIKE '%%"Edit":"8"%%' THEN 1 ELSE 0 END) AS deleted,
		       SUM(CASE WHEN raw LIKE '%%"ReactionMessage":{%%' THEN 1 ELSE 0 END) AS reactions
		FROM events
		WHERE type LIKE '%%Message%%'
		  AND json_extract(raw,'$.Info.Chat') = '%s'
		  %s`, jid, periodClause)).One(&sum)

	// ── Load contact names ────────────────────────────────────────────────────
	contactNames := map[string]string{}
	if waDB != nil {
		// Direct contacts (phone-number JIDs and any @lid already in contacts table).
		crows, err := waDB.Query(`SELECT their_jid, COALESCE(NULLIF(push_name,''), NULLIF(full_name,''), their_jid) AS name FROM whatsmeow_contacts`)
		if err == nil {
			defer crows.Close()
			for crows.Next() {
				var j, n string
				if crows.Scan(&j, &n) == nil {
					contactNames[j] = n
				}
			}
		}
		// LID → name: join whatsmeow_lid_map with whatsmeow_contacts via the PN.
		// The table stores only the User part (e.g. "1234567890"), so we append
		// the server domain to reconstruct full JIDs for both the key and the JOIN.
		lrows, err := waDB.Query(`
			SELECT m.lid || '@lid',
			       COALESCE(NULLIF(c.push_name,''), NULLIF(c.full_name,''), m.pn || '@s.whatsapp.net') AS name
			FROM whatsmeow_lid_map m
			JOIN whatsmeow_contacts c ON c.their_jid = m.pn || '@s.whatsapp.net'`)
		if err == nil {
			defer lrows.Close()
			for lrows.Next() {
				var lid, name string
				if lrows.Scan(&lid, &name) == nil {
					contactNames[lid] = name
				}
			}
		}
	}
	enrichName := func(memberJID string) string {
		// Normalise device-suffix JIDs: "123:46@s.whatsapp.net" → "123@s.whatsapp.net"
		normJID := memberJID
		if colon := strings.Index(normJID, ":"); colon > 0 {
			if at := strings.Index(normJID, "@"); at > colon {
				normJID = normJID[:colon] + normJID[at:]
			}
		}
		if name, ok := contactNames[normJID]; ok {
			return name
		}
		if name, ok := contactNames[memberJID]; ok {
			return name
		}
		if at := strings.Index(normJID, "@"); at > 0 {
			return normJID[:at]
		}
		return memberJID
	}

	// ── All sender message counts (for member enrichment) ─────────────────────
	type senderStat struct {
		SenderJID  string `db:"sender_jid"`
		MsgCount   int    `db:"msg_count"`
		MediaCount int    `db:"media_count"`
	}
	var senderStats []senderStat
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT json_extract(raw,'$.Info.Sender') AS sender_jid,
		       COUNT(*) AS msg_count,
		       SUM(CASE WHEN file != '' THEN 1 ELSE 0 END) AS media_count
		FROM events
		WHERE type LIKE '%%Message%%'
		  AND json_extract(raw,'$.Info.Chat') = '%s'
		  AND json_extract(raw,'$.Info.IsFromMe') = 0
		  %s
		GROUP BY sender_jid`, jid, periodClause)).All(&senderStats)

	type msgStat struct{ MsgCount, MediaCount int }
	msgMap := map[string]msgStat{}
	for _, s := range senderStats {
		// Normalize device-suffixed LIDs (e.g. "123:15@lid" → "123@lid")
		// so they match whatsmeow_lid_map keys which have no device part.
		key := s.SenderJID
		if colon := strings.Index(key, ":"); colon > 0 {
			if at := strings.Index(key, "@"); at > colon {
				key = key[:colon] + key[at:] // strip device part
			}
		}
		existing := msgMap[key]
		msgMap[key] = msgStat{existing.MsgCount + s.MsgCount, existing.MediaCount + s.MediaCount}
		// Also keep original key for direct lookups (e.g. GetGroupInfo returns bare user JID)
		if key != s.SenderJID {
			msgMap[s.SenderJID] = msgMap[key]
		}
	}
	// Normalize: add LID↔PN aliases so lookups work regardless of which form
	// was stored in events. Merge counts if both forms have entries.
	for lid, pn := range loadLIDMap() {
		lidStat, hasLID := msgMap[lid]
		pnStat, hasPN := msgMap[pn]
		if hasLID && hasPN {
			merged := msgStat{lidStat.MsgCount + pnStat.MsgCount, lidStat.MediaCount + pnStat.MediaCount}
			msgMap[lid] = merged
			msgMap[pn] = merged
		} else if hasLID {
			msgMap[pn] = lidStat
		} else if hasPN {
			msgMap[lid] = pnStat
		}
	}

	// ── Current members + admins from group_membership ────────────────────────
	type memberRow struct {
		MemberJID  string `db:"member_jid" json:"jid"`
		Action     string `db:"action"     json:"-"`
		Name       string `json:"name"`
		IsAdmin    bool   `json:"is_admin"`
		MsgCount   int    `json:"msg_count"`
		MediaCount int    `json:"media_count"`
	}
	// 'demote' means the member was demoted from admin but is still in the group.
	// Include join + promote + demote; exclude only leave.
	var currentMembers []memberRow
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT m.member_jid, m.action
		FROM group_membership m
		INNER JOIN (
		    SELECT member_jid, MAX(created) AS max_created
		    FROM group_membership WHERE group_jid = '%s'
		    GROUP BY member_jid
		) latest ON m.member_jid = latest.member_jid AND m.created = latest.max_created
		WHERE m.group_jid = '%s'
		  AND m.action NOT IN ('leave')
		ORDER BY m.member_jid ASC`, jid, jid)).All(&currentMembers)

	// Build a set of known current members for quick lookup.
	memberSet := map[string]bool{}
	adminSet := map[string]bool{}
	for i := range currentMembers {
		currentMembers[i].IsAdmin = currentMembers[i].Action == "promote"
		currentMembers[i].Name = enrichName(currentMembers[i].MemberJID)
		if stats, ok := msgMap[currentMembers[i].MemberJID]; ok {
			currentMembers[i].MsgCount = stats.MsgCount
			currentMembers[i].MediaCount = stats.MediaCount
		}
		memberSet[currentMembers[i].MemberJID] = true
		if currentMembers[i].IsAdmin {
			adminSet[currentMembers[i].MemberJID] = true
		}
	}

	// ── Enrich with live participants from WhatsApp ────────────────────────────
	// group_membership only records events observed after the bot joined, so it
	// may be empty for pre-existing groups. GetGroupInfo fills the gaps.
	if groupJID, err := types.ParseJID(jid); err == nil {
		if info, err := whatsapp.GetGroupInfo(groupJID); err == nil {
			if info.Name != "" {
				groupName = info.Name
			}
			for _, p := range info.Participants {
				pJID := p.JID.String()
				if memberSet[pJID] {
					continue // already present — keep DB-derived entry
				}
				m := memberRow{
					MemberJID: pJID,
					Name:      enrichName(pJID),
					IsAdmin:   p.IsAdmin || p.IsSuperAdmin,
				}
				if stats, ok := msgMap[pJID]; ok {
					m.MsgCount = stats.MsgCount
					m.MediaCount = stats.MediaCount
				}
				currentMembers = append(currentMembers, m)
				memberSet[pJID] = true
				if m.IsAdmin {
					adminSet[pJID] = true
				}
			}
		}
	}

	// ── Derived lists ──────────────────────────────────────────────────────────
	// Admins: from membership data (requires recorded events).
	var admins []memberRow
	for _, m := range currentMembers {
		if m.IsAdmin {
			admins = append(admins, m)
		}
	}

	// Top 5 active / least active: derived from senderStats (message events),
	// which is always populated regardless of membership history completeness.
	// Enrich with admin status from membership data where available.
	sortedSenders := make([]senderStat, len(senderStats))
	copy(sortedSenders, senderStats)
	sort.Slice(sortedSenders, func(i, j int) bool { return sortedSenders[i].MsgCount > sortedSenders[j].MsgCount })

	makeRow := func(s senderStat) memberRow {
		return memberRow{
			MemberJID:  s.SenderJID,
			Name:       enrichName(s.SenderJID),
			IsAdmin:    adminSet[s.SenderJID],
			MsgCount:   s.MsgCount,
			MediaCount: s.MediaCount,
		}
	}

	top5Active := make([]memberRow, 0, 5)
	for _, s := range sortedSenders {
		if len(top5Active) >= 5 {
			break
		}
		top5Active = append(top5Active, makeRow(s))
	}

	leastActive := make([]memberRow, 0, 5)
	for i := len(sortedSenders) - 1; i >= 0 && len(leastActive) < 5; i-- {
		leastActive = append(leastActive, makeRow(sortedSenders[i]))
	}

	// Silent members: current members (known via membership events) with 0 messages.
	// Only meaningful when membership history is available.
	var silentList []memberRow
	for _, m := range currentMembers {
		if m.MsgCount == 0 {
			silentList = append(silentList, m)
		}
	}

	// Nil-guard all slices
	if admins == nil {
		admins = []memberRow{}
	}
	if silentList == nil {
		silentList = []memberRow{}
	}
	if top5Active == nil {
		top5Active = []memberRow{}
	}
	if leastActive == nil {
		leastActive = []memberRow{}
	}
	if currentMembers == nil {
		currentMembers = []memberRow{}
	}

	// ── Ranking top 25 ────────────────────────────────────────────────────────
	type rankRow struct {
		SenderJID  string `db:"sender_jid"  json:"jid"`
		MsgCount   int    `db:"msg_count"   json:"msg_count"`
		MediaCount int    `db:"media_count" json:"media_count"`
		Name       string `json:"name"`
	}
	var ranking []rankRow
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT json_extract(raw,'$.Info.Sender') AS sender_jid,
		       COUNT(*) AS msg_count,
		       SUM(CASE WHEN file != '' THEN 1 ELSE 0 END) AS media_count
		FROM events
		WHERE type LIKE '%%Message%%'
		  AND json_extract(raw,'$.Info.Chat') = '%s'
		  AND json_extract(raw,'$.Info.IsFromMe') = 0
		  %s
		GROUP BY sender_jid ORDER BY msg_count DESC LIMIT 25`, jid, periodClause)).All(&ranking)
	for i := range ranking {
		ranking[i].Name = enrichName(ranking[i].SenderJID)
	}
	if ranking == nil {
		ranking = []rankRow{}
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
		  AND json_extract(raw,'$.Info.Chat') = '%s'
		  %s
		GROUP BY dow, hour ORDER BY dow, hour`, jid, periodClause)).All(&heatmap)
	if heatmap == nil {
		heatmap = []heatCell{}
	}

	// ── Peak activity ─────────────────────────────────────────────────────────
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
		  AND json_extract(raw,'$.Info.Chat') = '%s'
		  %s
		GROUP BY dow, hour ORDER BY cnt DESC LIMIT 1`, jid, periodClause)).One(&peak)

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
		  AND json_extract(raw,'$.Info.Chat') = '%s'
		  AND datetime(created) >= datetime('now', '-%d days')
		GROUP BY day ORDER BY day ASC`, jid, dailyDays)).All(&daily)
	if daily == nil {
		daily = []dayRow{}
	}

	// ── Membership evolution ──────────────────────────────────────────────────
	type evolRow struct {
		Day    string `db:"day"    json:"day"`
		Joins  int    `db:"joins"  json:"joins"`
		Leaves int    `db:"leaves" json:"leaves"`
	}
	var evolution []evolRow
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT strftime('%%Y-%%m-%%d', datetime(created)) AS day,
		       SUM(CASE WHEN action='join'  THEN 1 ELSE 0 END) AS joins,
		       SUM(CASE WHEN action='leave' THEN 1 ELSE 0 END) AS leaves
		FROM group_membership WHERE group_jid = '%s'
		GROUP BY day ORDER BY day ASC`, jid)).All(&evolution)
	if evolution == nil {
		evolution = []evolRow{}
	}

	// ── Message type distribution ─────────────────────────────────────────────
	type typeRow struct {
		MsgType string `db:"msg_type" json:"type"`
		Count   int    `db:"cnt"      json:"count"`
	}
	var typeDist []typeRow
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT CASE
		  WHEN raw LIKE '%%"ImageMessage":{%%'    THEN 'image'
		  WHEN raw LIKE '%%"VideoMessage":{%%'    THEN 'video'
		  WHEN raw LIKE '%%"AudioMessage":{%%'    THEN 'audio'
		  WHEN raw LIKE '%%"DocumentMessage":{%%' THEN 'document'
		  WHEN raw LIKE '%%"StickerMessage":{%%'  THEN 'sticker'
		  WHEN raw LIKE '%%"ReactionMessage":{%%' THEN 'reaction'
		  ELSE 'text'
		END AS msg_type, COUNT(*) AS cnt
		FROM events
		WHERE type LIKE '%%Message%%'
		  AND json_extract(raw,'$.Info.Chat') = '%s'
		  %s
		GROUP BY msg_type ORDER BY cnt DESC`, jid, periodClause)).All(&typeDist)
	if typeDist == nil {
		typeDist = []typeRow{}
	}

	// ── First and last message ─────────────────────────────────────────────────
	type flRow struct {
		First string `db:"first_seen"`
		Last  string `db:"last_seen"`
	}
	var fl flRow
	_ = pb.DB().NewQuery(fmt.Sprintf(`
		SELECT MIN(created) AS first_seen, MAX(created) AS last_seen
		FROM events WHERE type LIKE '%%Message%%'
		  AND json_extract(raw,'$.Info.Chat') = '%s'`, jid)).One(&fl)

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

	return e.JSON(http.StatusOK, map[string]any{
		"jid":  jid,
		"name": groupName,
		"summary": map[string]any{
			"total_messages": sum.Total,
			"total_media":    sum.Media,
			"active_members": len(senderStats),
			"known_members":  len(currentMembers),
			"silent_members": len(silentList),
			"edited":         sum.Edited,
			"deleted":        sum.Deleted,
			"reactions":      sum.Reactions,
		},
		"peak_activity":        peakLabel,
		"admins":               admins,
		"current_members":      currentMembers,
		"member_ranking":       ranking,
		"top5_active":          top5Active,
		"top5_least_active":    leastActive,
		"silent_members_list":  silentList,
		"heatmap":              heatmap,
		"daily":                daily,
		"membership_evolution": evolution,
		"type_distribution":    typeDist,
		"first_seen":           fl.First,
		"last_seen":            fl.Last,
		"period":               period,
	})
}
