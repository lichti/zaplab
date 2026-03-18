package api

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"
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
	}
	enrichName := func(memberJID string) string {
		if name, ok := contactNames[memberJID]; ok {
			return name
		}
		if at := strings.Index(memberJID, "@"); at > 0 {
			return memberJID[:at]
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
		msgMap[s.SenderJID] = msgStat{s.MsgCount, s.MediaCount}
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
		  AND m.action IN ('join', 'promote')
		ORDER BY m.member_jid ASC`, jid, jid)).All(&currentMembers)

	for i := range currentMembers {
		currentMembers[i].IsAdmin = currentMembers[i].Action == "promote"
		currentMembers[i].Name = enrichName(currentMembers[i].MemberJID)
		if stats, ok := msgMap[currentMembers[i].MemberJID]; ok {
			currentMembers[i].MsgCount = stats.MsgCount
			currentMembers[i].MediaCount = stats.MediaCount
		}
	}

	// ── Derived lists ──────────────────────────────────────────────────────────
	var admins []memberRow
	var silentList []memberRow
	var activeList []memberRow

	for _, m := range currentMembers {
		if m.IsAdmin {
			admins = append(admins, m)
		}
		if m.MsgCount == 0 {
			silentList = append(silentList, m)
		} else {
			activeList = append(activeList, m)
		}
	}

	sort.Slice(activeList, func(i, j int) bool { return activeList[i].MsgCount > activeList[j].MsgCount })
	top5Active := activeList
	if len(top5Active) > 5 {
		top5Active = top5Active[:5]
	}

	leastActive := make([]memberRow, len(activeList))
	copy(leastActive, activeList)
	sort.Slice(leastActive, func(i, j int) bool { return leastActive[i].MsgCount < leastActive[j].MsgCount })
	if len(leastActive) > 5 {
		leastActive = leastActive[:5]
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
