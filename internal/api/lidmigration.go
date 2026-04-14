package api

import (
	"strings"

	"github.com/pocketbase/dbx"
)

// whatsmeow_lid_map stores only the User part of each JID (no @server domain).
// These constants reconstruct the full JID strings that appear in the events table.
const lidServer = "@lid"
const pnServer = "@s.whatsapp.net"

// loadLIDMap returns a map of full-JID lid → full-JID pn ("123@lid" → "555@s.whatsapp.net").
// Returns an empty map (never nil) when waDB is not available.
func loadLIDMap() map[string]string {
	m := map[string]string{}
	if waDB == nil {
		return m
	}
	// The table stores only the User part (e.g. "1234567890" not "1234567890@lid").
	rows, err := waDB.Query(`SELECT lid, pn FROM whatsmeow_lid_map`)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var lidUser, pnUser string
		if rows.Scan(&lidUser, &pnUser) == nil && lidUser != "" && pnUser != "" {
			m[lidUser+lidServer] = pnUser + pnServer
		}
	}
	return m
}

// normalizeLIDJID returns the canonical PN JID for a given jid string,
// falling back to the input unchanged if no mapping is found.
func normalizeLIDJID(jid string, lidMap map[string]string) string {
	if strings.HasSuffix(jid, lidServer) {
		if pn, ok := lidMap[jid]; ok {
			return pn
		}
	}
	return jid
}

// runLIDMigration rewrites existing events in the PocketBase DB so that every
// @lid sender/chat JID is replaced by the canonical phone-number JID.
//
// It reads all known LID→PN mappings from whatsmeow_lid_map (waDB) and for
// each pair issues two UPDATE statements against the events table:
//
//  1. $.Info.Sender — covers group messages sent by a LID participant.
//  2. $.Info.Chat   — covers private-chat events where the chat itself is a LID.
//
// The function is best-effort and runs in a goroutine at startup so it does
// not block the server.  Events whose LID was not yet mapped are left as-is
// and will be normalized on the next run or when new mappings arrive.
func runLIDMigration() {
	if waDB == nil || pb == nil || pb.DB() == nil {
		return
	}

	// The table stores only the User part (e.g. "1234567890" not "1234567890@lid").
	rows, err := waDB.Query(`SELECT lid, pn FROM whatsmeow_lid_map`)
	if err != nil {
		pb.Logger().Warn("LID migration: failed to read whatsmeow_lid_map", "error", err)
		return
	}
	defer rows.Close()

	type mapping struct{ lid, pn string } // full JID strings
	var mappings []mapping
	for rows.Next() {
		var lidUser, pnUser string
		if rows.Scan(&lidUser, &pnUser) == nil && lidUser != "" && pnUser != "" {
			mappings = append(mappings, mapping{
				lid: lidUser + lidServer,
				pn:  pnUser + pnServer,
			})
		}
	}
	if err := rows.Err(); err != nil {
		pb.Logger().Warn("LID migration: row error", "error", err)
		return
	}

	if len(mappings) == 0 {
		return
	}

	updated := 0
	for _, m := range mappings {
		params := dbx.Params{"lid": m.lid, "pn": m.pn}

		// Fix $.Info.Sender (group messages, presence events, receipts…)
		res, err := pb.DB().NewQuery(
			`UPDATE events
			    SET raw = json_replace(raw, '$.Info.Sender', {:pn})
			  WHERE json_extract(raw, '$.Info.Sender') = {:lid}`,
		).Bind(params).Execute()
		if err != nil {
			pb.Logger().Warn("LID migration: update Sender failed", "lid", m.lid, "error", err)
		} else if n, _ := res.RowsAffected(); n > 0 {
			updated += int(n)
		}

		// Fix $.Info.Chat (DM conversations keyed by LID)
		res, err = pb.DB().NewQuery(
			`UPDATE events
			    SET raw = json_replace(raw, '$.Info.Chat', {:pn})
			  WHERE json_extract(raw, '$.Info.Chat') = {:lid}`,
		).Bind(params).Execute()
		if err != nil {
			pb.Logger().Warn("LID migration: update Chat failed", "lid", m.lid, "error", err)
		} else if n, _ := res.RowsAffected(); n > 0 {
			updated += int(n)
		}
	}

	if updated > 0 {
		pb.Logger().Info("LID migration: normalized events", "count", updated)
	}
}
