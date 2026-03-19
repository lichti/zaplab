package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// ── Network Graph ─────────────────────────────────────────────────────────────
//
// Builds a contact/group relationship graph from stored Message events.
//
// Nodes:
//   - self   — the device owner (JID from whatsmeow_device)
//   - contact — individual chat partners (JIDs ending in @s.whatsapp.net)
//   - group   — group chats (JIDs ending in @g.us)
//   - broadcast — broadcast lists (@broadcast)
//
// Edges:
//   - self ↔ chat: number of messages exchanged
//   - sender ↔ group: member appeared in a group conversation
//
// Contact names are enriched from whatsmeow_contacts when waDB is available.

// getNetworkGraph returns nodes and edges for rendering a force-directed graph.
//
// Query params:
//
//	period        int     days to look back (default 30, max 365; 0 = all time)
//	date_from     string  ISO-8601 date lower bound (overrides period when set)
//	date_to       string  ISO-8601 date upper bound (inclusive)
//	event_types   string  comma-separated event type filters (default "Message")
//	min_msgs      int     minimum message count for a node to be included (default 1)
//	include_groups bool   include group nodes (default true; set "false" to hide)
//	limit         int     max events to scan (default 2000, max 10000)
func getNetworkGraph(e *core.RequestEvent) error {
	q := e.Request.URL.Query()

	period, _ := strconv.Atoi(q.Get("period"))
	if period < 0 || period > 365 {
		period = 30
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 100 || limit > 10000 {
		limit = 2000
	}
	minMsgs, _ := strconv.Atoi(q.Get("min_msgs"))
	if minMsgs < 1 {
		minMsgs = 1
	}
	includeGroups := q.Get("include_groups") != "false"

	// direction: "both" (default) | "sent" | "received"
	direction := q.Get("direction")
	if direction != "sent" && direction != "received" {
		direction = "both"
	}

	dateFrom := sanitizeSQL(q.Get("date_from"))
	dateTo := sanitizeSQL(q.Get("date_to"))

	// Build event-type filter — default to Message only
	rawTypes := q.Get("event_types")
	var typeFilters []string
	if rawTypes != "" {
		for _, t := range strings.Split(rawTypes, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				typeFilters = append(typeFilters, "'"+sanitizeSQL(t)+"'")
			}
		}
	}
	typeClause := "type = 'Message'"
	if len(typeFilters) > 0 {
		typeClause = "type IN (" + strings.Join(typeFilters, ",") + ")"
	}

	// Date range: explicit date_from/date_to take priority over period
	var timeClause string
	if dateFrom != "" && dateTo != "" {
		timeClause = fmt.Sprintf("AND datetime(created) BETWEEN datetime('%s') AND datetime('%s', '+1 day')", dateFrom, dateTo)
	} else if dateFrom != "" {
		timeClause = fmt.Sprintf("AND datetime(created) >= datetime('%s')", dateFrom)
	} else if dateTo != "" {
		timeClause = fmt.Sprintf("AND datetime(created) <= datetime('%s', '+1 day')", dateTo)
	} else if period > 0 {
		timeClause = fmt.Sprintf("AND datetime(created) >= datetime('now', '-%d days')", period)
	}

	rawSQL := fmt.Sprintf(
		`SELECT raw FROM events WHERE %s %s ORDER BY created DESC LIMIT %d`,
		typeClause, timeClause, limit,
	)

	type rawRow struct {
		Raw string `db:"raw"`
	}
	var rows []rawRow
	if err := pb.DB().NewQuery(rawSQL).All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}

	// ── parse raw JSON from each Message event ────────────────────────────────

	type msgInfo struct {
		Info struct {
			Chat     string `json:"Chat"`
			Sender   string `json:"Sender"`
			IsFromMe bool   `json:"IsFromMe"`
			IsGroup  bool   `json:"IsGroup"`
		} `json:"Info"`
	}

	// ── build graph data structures ───────────────────────────────────────────

	type nodeData struct {
		ID       string `json:"id"`
		Label    string `json:"label"`
		NodeType string `json:"node_type"` // "self"|"contact"|"group"|"broadcast"
		MsgCount int    `json:"msg_count"`
	}
	type edgeData struct {
		Source   string `json:"source"`
		Target   string `json:"target"`
		Weight   int    `json:"weight"`
		Sent     int    `json:"sent"`
		Received int    `json:"received"`
	}

	nodeMap := map[string]*nodeData{}
	edgeMap := map[string]*edgeData{} // "a|b" (lexicographic) → edge

	getOrCreate := func(id, ntype string) *nodeData {
		if n, ok := nodeMap[id]; ok {
			return n
		}
		label := id
		if at := strings.Index(id, "@"); at > 0 {
			label = id[:at]
		}
		n := &nodeData{ID: id, Label: label, NodeType: ntype}
		nodeMap[id] = n
		return n
	}

	// addEdge records a directed message: from→to. "sent" is from self's perspective.
	addEdge := func(from, to string, isFromMe bool) {
		a, b := from, to
		if a > b {
			a, b = b, a
		}
		key := a + "|" + b
		ed, ok := edgeMap[key]
		if !ok {
			ed = &edgeData{Source: a, Target: b}
			edgeMap[key] = ed
		}
		ed.Weight++
		if isFromMe {
			ed.Sent++
		} else {
			ed.Received++
		}
	}

	// Get device's own JID (best-effort)
	selfJID := "self"
	if waDB != nil {
		row := waDB.QueryRow(`SELECT jid FROM whatsmeow_device LIMIT 1`)
		_ = row.Scan(&selfJID)
	}
	getOrCreate(selfJID, "self")

	// Load contact names from whatsmeow_contacts for label enrichment
	contactNames := map[string]string{}
	if waDB != nil {
		crows, err := waDB.Query(`SELECT jid, COALESCE(NULLIF(push_name,''), NULLIF(full_name,''), jid) AS name FROM whatsmeow_contacts`)
		if err == nil {
			defer crows.Close()
			for crows.Next() {
				var jid, name string
				if crows.Scan(&jid, &name) == nil {
					contactNames[jid] = name
				}
			}
		}
	}

	// Process events
	for _, r := range rows {
		var msg msgInfo
		if json.Unmarshal([]byte(r.Raw), &msg) != nil {
			continue
		}
		chat := msg.Info.Chat
		sender := msg.Info.Sender
		if chat == "" {
			continue
		}

		// Direction filter
		if direction == "sent" && !msg.Info.IsFromMe {
			continue
		}
		if direction == "received" && msg.Info.IsFromMe {
			continue
		}

		chatType := "contact"
		if strings.HasSuffix(chat, "@g.us") {
			chatType = "group"
		} else if strings.HasSuffix(chat, "@broadcast") {
			chatType = "broadcast"
		}

		// Skip group nodes when include_groups=false
		if chatType == "group" && !includeGroups {
			continue
		}

		chatNode := getOrCreate(chat, chatType)
		chatNode.MsgCount++

		// Enrich label with push name
		if name, ok := contactNames[chat]; ok && name != chat {
			chatNode.Label = name
		}

		// In a group, the sender is the member — add a contact node + member edge
		if chatType == "group" && sender != "" && sender != selfJID {
			senderNode := getOrCreate(sender, "contact")
			senderNode.MsgCount++
			if name, ok := contactNames[sender]; ok && name != sender {
				senderNode.Label = name
			}
			addEdge(sender, chat, false) // member → group (not from self)
		}

		// Device ↔ chat edge (counts every message)
		addEdge(selfJID, chat, msg.Info.IsFromMe)
	}

	// ── apply min_msgs filter then trim to top-N nodes by message count ─────

	const maxNodes = 100
	nodes := make([]*nodeData, 0, len(nodeMap))
	for _, n := range nodeMap {
		if n.NodeType != "self" && n.MsgCount < minMsgs {
			continue
		}
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].NodeType == "self" {
			return true
		}
		if nodes[j].NodeType == "self" {
			return false
		}
		return nodes[i].MsgCount > nodes[j].MsgCount
	})
	if len(nodes) > maxNodes {
		nodes = nodes[:maxNodes]
	}

	kept := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		kept[n.ID] = true
	}

	edges := make([]*edgeData, 0, len(edgeMap))
	for _, ed := range edgeMap {
		if kept[ed.Source] && kept[ed.Target] {
			edges = append(edges, ed)
		}
	}

	return e.JSON(http.StatusOK, map[string]any{
		"nodes":          nodes,
		"edges":          edges,
		"period":         period,
		"date_from":      dateFrom,
		"date_to":        dateTo,
		"min_msgs":       minMsgs,
		"include_groups": includeGroups,
		"total_messages": len(rows),
		"total_nodes":    len(nodes),
		"total_edges":    len(edges),
	})
}
