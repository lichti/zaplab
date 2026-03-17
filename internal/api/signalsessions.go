package api

import (
	"encoding/hex"
	"net/http"

	"github.com/pocketbase/pocketbase/core"
	groupRecord "go.mau.fi/libsignal/groups/state/record"
	signalRecord "go.mau.fi/libsignal/state/record"
	whatsmeowStore "go.mau.fi/whatsmeow/store"
)

// ── Signal Session Visualizer ─────────────────────────────────────────────────

// getSignalSessions returns a decoded view of all Double Ratchet session states
// stored in whatsmeow_sessions. Uses the existing waDB read-only connection.
func getSignalSessions(e *core.RequestEvent) error {
	if waDB == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]any{
			"error": "DB Explorer not initialised (sqlite only)",
		})
	}

	rows, err := waDB.Query(`SELECT their_id, session FROM whatsmeow_sessions ORDER BY their_id`)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	defer rows.Close()

	type chainInfo struct {
		Counter uint32 `json:"counter"`
	}
	type sessionSummary struct {
		Address         string `json:"address"`
		Version         int    `json:"version"`
		HasSenderChain  bool   `json:"has_sender_chain"`
		SenderCounter   uint32 `json:"sender_counter"`
		ReceiverChains  int    `json:"receiver_chains"`
		PreviousCounter uint32 `json:"previous_counter"`
		RemoteIdentity  string `json:"remote_identity"`
		LocalIdentity   string `json:"local_identity"`
		PreviousStates  int    `json:"previous_states"`
		RawSize         int    `json:"raw_size_bytes"`
		DecodeError     string `json:"decode_error,omitempty"`
	}

	var sessions []sessionSummary
	for rows.Next() {
		var address string
		var raw []byte
		if err := rows.Scan(&address, &raw); err != nil {
			continue
		}
		s := sessionSummary{Address: address, RawSize: len(raw)}

		sess, err := signalRecord.NewSessionFromBytes(
			raw,
			whatsmeowStore.SignalProtobufSerializer.Session,
			whatsmeowStore.SignalProtobufSerializer.State,
		)
		if err != nil {
			s.DecodeError = err.Error()
			sessions = append(sessions, s)
			continue
		}

		state := sess.SessionState()
		if state != nil {
			s.Version = state.Version()
			s.HasSenderChain = state.HasSenderChain()
			s.PreviousCounter = state.PreviousCounter()
			if state.HasSenderChain() {
				ck := state.SenderChainKey()
				if ck != nil {
					s.SenderCounter = ck.Index()
				}
			}
			if rk := state.RemoteIdentityKey(); rk != nil {
				s.RemoteIdentity = hex.EncodeToString(rk.PublicKey().Serialize())
			}
			if lk := state.LocalIdentityKey(); lk != nil {
				s.LocalIdentity = hex.EncodeToString(lk.PublicKey().Serialize())
			}
		}
		s.PreviousStates = len(sess.PreviousSessionStates())
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if sessions == nil {
		sessions = []sessionSummary{}
	}
	return e.JSON(http.StatusOK, map[string]any{"sessions": sessions, "total": len(sessions)})
}

// getSignalSenderKeys returns a decoded view of all group sender key states
// stored in whatsmeow_sender_keys.
func getSignalSenderKeys(e *core.RequestEvent) error {
	if waDB == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]any{
			"error": "DB Explorer not initialised (sqlite only)",
		})
	}

	rows, err := waDB.Query(`SELECT chat_id, sender_id, sender_key FROM whatsmeow_sender_keys ORDER BY chat_id, sender_id`)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	defer rows.Close()

	type senderKeySummary struct {
		ChatID      string `json:"chat_id"`
		SenderID    string `json:"sender_id"`
		KeyID       uint32 `json:"key_id"`
		Iteration   uint32 `json:"iteration"`
		SigningKey  string `json:"signing_key"`
		RawSize     int    `json:"raw_size_bytes"`
		DecodeError string `json:"decode_error,omitempty"`
	}

	var keys []senderKeySummary
	for rows.Next() {
		var chatID, senderID string
		var raw []byte
		if err := rows.Scan(&chatID, &senderID, &raw); err != nil {
			continue
		}
		s := senderKeySummary{ChatID: chatID, SenderID: senderID, RawSize: len(raw)}

		sk, err := groupRecord.NewSenderKeyFromBytes(
			raw,
			whatsmeowStore.SignalProtobufSerializer.SenderKeyRecord,
			whatsmeowStore.SignalProtobufSerializer.SenderKeyState,
		)
		if err != nil {
			s.DecodeError = err.Error()
			keys = append(keys, s)
			continue
		}

		state, stateErr := sk.SenderKeyState()
		if stateErr == nil && state != nil {
			s.KeyID = state.KeyID()
			ck := state.SenderChainKey()
			if ck != nil {
				s.Iteration = ck.Iteration()
			}
			if sig := state.SigningKey(); sig != nil && sig.PublicKey() != nil {
				s.SigningKey = hex.EncodeToString(sig.PublicKey().Serialize())
			}
		}
		keys = append(keys, s)
	}
	if err := rows.Err(); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if keys == nil {
		keys = []senderKeySummary{}
	}
	return e.JSON(http.StatusOK, map[string]any{"sender_keys": keys, "total": len(keys)})
}
