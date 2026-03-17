package api

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"github.com/pocketbase/pocketbase/core"
)

// ── App State Inspector ───────────────────────────────────────────────────────
//
// Three read-only endpoints that expose the three whatsmeow app state tables
// via the `waDB` read-only SQL connection opened by initDBExplorer.
//
// Known WhatsApp app state collection names:
//
//	critical                    — privacy settings, PIN, security config
//	regular                     — contacts, starred messages, chats, labels
//	critical_unblock_to_primary — secondary critical settings → primary device
//	critical_block              — blocked contacts and spam reports
//	regular_low                 — low-priority preferences

// getAppStateCollections returns all rows from whatsmeow_app_state_version,
// showing the current version index and hash for every named collection.
func getAppStateCollections(e *core.RequestEvent) error {
	if waDB == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]any{"error": "DB Explorer not initialised"})
	}

	rows, err := waDB.Query(`SELECT jid, name, version, hash FROM whatsmeow_app_state_version ORDER BY jid, name`)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	defer rows.Close()

	type collOut struct {
		JID     string `json:"jid"`
		Name    string `json:"name"`
		Version int64  `json:"version"`
		HashHex string `json:"hash_hex"`
	}

	var out []collOut
	for rows.Next() {
		var jid, name string
		var version int64
		var hash []byte
		if err := rows.Scan(&jid, &name, &version, &hash); err != nil {
			continue
		}
		out = append(out, collOut{
			JID:     jid,
			Name:    name,
			Version: version,
			HashHex: hex.EncodeToString(hash),
		})
	}
	if out == nil {
		out = []collOut{}
	}
	return e.JSON(http.StatusOK, map[string]any{"collections": out, "total": len(out)})
}

// getAppStateSyncKeys returns all rows from whatsmeow_app_state_sync_keys.
// These symmetric keys decrypt app state patches received from the server.
// The raw key_data bytes are withheld; only metadata is returned.
func getAppStateSyncKeys(e *core.RequestEvent) error {
	if waDB == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]any{"error": "DB Explorer not initialised"})
	}

	rows, err := waDB.Query(`SELECT jid, key_id, key_data, timestamp, fingerprint FROM whatsmeow_app_state_sync_keys ORDER BY jid, key_id`)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	defer rows.Close()

	type keyOut struct {
		JID         string `json:"jid"`
		KeyIDHex    string `json:"key_id_hex"`
		Timestamp   int64  `json:"timestamp"`
		Fingerprint string `json:"fingerprint_hex"`
		DataLen     int    `json:"data_len_bytes"`
	}

	var out []keyOut
	for rows.Next() {
		var jid string
		var keyID, keyData, fingerprint []byte
		var ts int64
		if err := rows.Scan(&jid, &keyID, &keyData, &ts, &fingerprint); err != nil {
			continue
		}
		out = append(out, keyOut{
			JID:         jid,
			KeyIDHex:    hex.EncodeToString(keyID),
			Timestamp:   ts,
			Fingerprint: hex.EncodeToString(fingerprint),
			DataLen:     len(keyData),
		})
	}
	if out == nil {
		out = []keyOut{}
	}
	return e.JSON(http.StatusOK, map[string]any{"sync_keys": out, "total": len(out)})
}

// getAppStateMutations returns mutation MAC rows from
// whatsmeow_app_state_mutation_macs for the requested collection name.
//
// Query params:
//
//	collection  string  required — app state collection name (e.g. "critical")
//	limit       int     max rows (default 100, max 500)
func getAppStateMutations(e *core.RequestEvent) error {
	if waDB == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]any{"error": "DB Explorer not initialised"})
	}

	collection := e.Request.URL.Query().Get("collection")
	if collection == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "collection is required"})
	}
	limit, _ := strconv.Atoi(e.Request.URL.Query().Get("limit"))
	if limit < 1 || limit > 500 {
		limit = 100
	}

	query := fmt.Sprintf(
		`SELECT jid, name, version, index_mac, value_mac FROM whatsmeow_app_state_mutation_macs WHERE name='%s' ORDER BY version DESC LIMIT %d`,
		sanitizeSQL(collection), limit,
	)

	rows, err := waDB.Query(query)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	defer rows.Close()

	type mutOut struct {
		JID      string `json:"jid"`
		Name     string `json:"name"`
		Version  int64  `json:"version"`
		IndexMAC string `json:"index_mac_hex"`
		ValueMAC string `json:"value_mac_hex"`
	}

	var out []mutOut
	for rows.Next() {
		var jid, name string
		var version int64
		var indexMAC, valueMAC []byte
		if err := rows.Scan(&jid, &name, &version, &indexMAC, &valueMAC); err != nil {
			continue
		}
		out = append(out, mutOut{
			JID:      jid,
			Name:     name,
			Version:  version,
			IndexMAC: hex.EncodeToString(indexMAC),
			ValueMAC: hex.EncodeToString(valueMAC),
		})
	}
	if out == nil {
		out = []mutOut{}
	}
	return e.JSON(http.StatusOK, map[string]any{
		"mutations":  out,
		"collection": collection,
		"total":      len(out),
		"limit":      limit,
	})
}
