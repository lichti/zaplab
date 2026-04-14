package whatsapp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"go.mau.fi/util/random"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

var historySyncID int32
var startupTime = time.Now().Unix()

// reconnecting is 1 while a reconnectWithBackoff goroutine is active.
var reconnecting int32

// eventQueue serializes all async event DB writes through a single worker goroutine,
// preventing SQLite write contention from unlimited concurrent goroutines.
var eventQueue = make(chan interface{}, 2000)

// EventQueueLen returns the current number of events waiting in the worker queue.
func EventQueueLen() int { return len(eventQueue) }

// startEventWorker drains eventQueue sequentially. Must be called once during bootstrap.
func startEventWorker() {
	go func() {
		for rawEvt := range eventQueue {
			handleAsync(rawEvt)
		}
	}()
}

// handler is called by whatsmeow synchronously in its node-processing goroutine.
// It must return as fast as possible — all I/O is dispatched to a new goroutine.
func handler(rawEvt interface{}) {
	// Connection lifecycle events update in-memory state synchronously (mutex ops
	// only — no I/O) and kick off goroutines for any blocking follow-up work.
	switch evt := rawEvt.(type) {
	case *events.Connected, *events.PushNameSetting:
		setConnStatus(StatusConnected)
		setLastQR("")
		go handleConnected(rawEvt)
		return

	case *events.Disconnected:
		setConnStatus(StatusDisconnected)
		logger.Warnf("WhatsApp disconnected, reconnecting with backoff... reason=%v", evt)
		go recordConnEvent("disconnected", fmt.Sprintf("%v", evt))
		go reconnectWithBackoff("disconnect")
		return

	case *events.LoggedOut:
		setConnStatus(StatusLoggedOut)
		logger.Warnf("WhatsApp logged out reason=%v", evt.Reason)
		go recordConnEvent("disconnected", fmt.Sprintf("logged out: %v", evt.Reason))
		return

	case *events.StreamReplaced:
		logger.Warnf("Stream replaced by another client, reconnecting with backoff...")
		go func() {
			if err := saveError(getTypeOf(rawEvt), "Stream replaced", rawEvt); err != nil {
				logger.Errorf("Error persisting StreamReplaced error: %v", err)
			}
		}()
		go reconnectWithBackoff("stream replaced")
		return
	}

	// Receipt probe signals must fire immediately — before the event is queued —
	// so the Activity Tracker goroutines get their RTT without queue-induced delay.
	if evt, ok := rawEvt.(*events.Receipt); ok {
		for _, msgID := range evt.MessageIDs {
			NotifyProbeReceipt(msgID)
		}
	}

	// All other processing (DB writes) goes through the serialized worker queue
	// to avoid SQLite write contention from concurrent goroutines.
	select {
	case eventQueue <- rawEvt:
	default:
		logger.Warnf("Event queue full, dropping event type=%s", getTypeOf(rawEvt))
	}
}

// handleConnected performs the post-connection I/O (SendPresence + persist) in a goroutine.
func handleConnected(rawEvt interface{}) {
	evtType := getTypeOf(rawEvt)
	go recordConnEvent("connected", "")
	if len(client.Store.PushName) == 0 {
		if err := saveEvent(evtType, rawEvt, nil); err != nil {
			logger.Errorf("Error persisting event type=%s error=%v", evtType, err)
		}
		return
	}
	if err := client.SendPresence(context.Background(), types.PresenceAvailable); err != nil {
		if err := saveError(evtType, "Failed to send presence", rawEvt); err != nil {
			logger.Errorf("Error persisting error type=%s error=%v", evtType, err)
		}
	} else {
		if err := saveEvent(evtType, rawEvt, nil); err != nil {
			logger.Errorf("Error persisting event type=%s error=%v", evtType, err)
		}
	}
}

// handleAsync processes all non-lifecycle events without blocking whatsmeow.
func handleAsync(rawEvt interface{}) {
	evtType := getTypeOf(rawEvt)

	switch evt := rawEvt.(type) {
	case *events.AppStateSyncComplete:
		if len(client.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
			if err := client.SendPresence(context.Background(), types.PresenceAvailable); err != nil {
				if err := saveError(evtType, "Failed to send presence", rawEvt); err != nil {
					logger.Errorf("Error persisting error type=%s error=%v", evtType, err)
				}
			} else {
				if err := saveEvent(evtType, rawEvt, nil); err != nil {
					logger.Errorf("Error persisting event type=%s error=%v", evtType, err)
				}
			}
		}
		return

	case *events.Message:
		metaParts := []string{
			fmt.Sprintf("pushname: %s", evt.Info.PushName),
			fmt.Sprintf("timestamp: %s", evt.Info.Timestamp),
		}
		if evt.Info.Type != "" {
			metaParts = append(metaParts, fmt.Sprintf("type: %s", evt.Info.Type))
		}
		if evt.Info.Category != "" {
			metaParts = append(metaParts, fmt.Sprintf("category: %s", evt.Info.Category))
		}
		if evt.IsViewOnce {
			metaParts = append(metaParts, "view once")
		}
		if evt.IsEphemeral {
			metaParts = append(metaParts, "ephemeral")
		}
		if evt.IsViewOnceV2 {
			metaParts = append(metaParts, "ephemeral (v2)")
		}
		if evt.IsDocumentWithCaption {
			metaParts = append(metaParts, "document with caption")
		}
		if evt.IsEdit {
			metaParts = append(metaParts, "edit")
		}
		logger.Infof("Received message id=%s from=%s meta=%v", evt.Info.ID, evt.Info.SourceString(), metaParts)

		// Fire text-pattern webhooks for all messages with text content.
		wh.SendToTextWebhooks(getMsg(evt), evt.Info.IsFromMe, rawEvt, nil)

		// Track @mentions.
		if !evt.Info.IsFromMe {
			DetectAndRecordMentions(evt)
		}

		if strings.HasPrefix(getMsg(evt), getIDSecret) && evt.Info.IsFromMe {
			jid, _ := ParseJID(client.Store.ID.User)
			SendConversationMessage(jid, fmt.Sprintf("-> Cmd output: \nChatID %v", evt.Info.Chat), nil)
			return
		}

		if strings.HasPrefix(getMsg(evt), "/setSecrete ") && evt.Info.IsFromMe {
			jid, _ := ParseJID(client.Store.ID.User)
			if evt.Info.Chat.String() == jid.String() {
				words := strings.SplitN(getMsg(evt), " ", 2)
				if len(words) > 1 {
					getIDSecret = words[1]
					SendConversationMessage(jid, fmt.Sprintf("-> Cmd output: \nbSecret set to %s", getIDSecret), nil)
				} else {
					SendConversationMessage(jid, "-> Cmd output: \nYou need to set a secret", nil)
				}
				return
			}
		}

		if strings.HasPrefix(getMsg(evt), "/resetSecrete") && evt.Info.IsFromMe {
			jid, _ := ParseJID(client.Store.ID.User)
			if evt.Info.Chat.String() == jid.String() {
				getIDSecret = strings.ToUpper(hex.EncodeToString(random.Bytes(8)))
				SendConversationMessage(jid, "-> Cmd output: \nbSecret reseted to a random value", nil)
				return
			}
		}

		if strings.HasPrefix(getMsg(evt), "/cmd ") && evt.Info.IsFromMe {
			jid, _ := ParseJID(client.Store.ID.User)
			if evt.Info.Chat.String() == jid.String() {
				words := strings.SplitN(getMsg(evt), " ", 3)
				if len(words) > 1 {
					args := []string{}
					if len(words) > 2 {
						args = strings.Split(words[2], " ")
					}
					out := HandleCmd(words[1], args, evt)
					SendConversationMessage(jid, fmt.Sprintf("-> Cmd output: %s", out), nil)
				} else {
					SendConversationMessage(jid, "-> Cmd output: \nYou need send a valid command", nil)
				}
				return
			}
		}

		strCmd := strings.SplitN(getMsg(evt), " ", 2)[0]
		if wh.CheckCmdExist(strCmd) {
			wh.SendToCmd(strCmd, "Message", rawEvt, nil)
		}

		if evt.Message.GetPollUpdateMessage() != nil {
			decrypted, err := client.DecryptPollVote(context.Background(), evt)
			if err != nil {
				if err := saveError(evtType+"."+getTypeOf(evt.Message.GetPollUpdateMessage()), "Failed to decrypt vote", rawEvt); err != nil {
					logger.Errorf("Error persisting error type=%s error=%v", evtType, err)
				}
			} else {
				if err := saveEvent(evtType+"."+getTypeOf(evt.Message.GetPollUpdateMessage()), rawEvt, decrypted); err != nil {
					logger.Errorf("Error persisting event type=%s error=%v", evtType, err)
				}
			}
			return
		}

		if evt.Message.GetEncReactionMessage() != nil {
			decrypted, err := client.DecryptReaction(context.Background(), evt)
			if err != nil {
				if err := saveError(evtType+"."+getTypeOf(evt.Message.GetEncReactionMessage()), "Failed to decrypt encrypted reaction", rawEvt); err != nil {
					logger.Errorf("Error persisting error type=%s error=%v", evtType, err)
				}
			} else {
				RecordReaction(decrypted, evt)
				if err := saveEvent(evtType+"."+getTypeOf(evt.Message.GetEncReactionMessage()), rawEvt, decrypted); err != nil {
					logger.Errorf("Error persisting event type=%s error=%v", evtType, err)
				}
			}
			return
		}

		// Use explicit typed nil checks to avoid the Go interface-nil gotcha:
		// storing a typed nil (*T)(nil) in interface{} yields a non-nil interface.
		if img := evt.Message.GetImageMessage(); img != nil {
			if err := download(evtType+"."+getTypeOf(img), img, img.GetMimetype(), evt, rawEvt); err == nil {
				return
			}
		} else if audio := evt.Message.GetAudioMessage(); audio != nil {
			if err := download(evtType+"."+getTypeOf(audio), audio, audio.GetMimetype(), evt, rawEvt); err == nil {
				return
			}
		} else if video := evt.Message.GetVideoMessage(); video != nil {
			if err := download(evtType+"."+getTypeOf(video), video, video.GetMimetype(), evt, rawEvt); err == nil {
				return
			}
		} else if doc := evt.Message.GetDocumentMessage(); doc != nil {
			if err := download(evtType+"."+getTypeOf(doc), doc, doc.GetMimetype(), evt, rawEvt); err == nil {
				return
			}
		} else if sticker := evt.Message.GetStickerMessage(); sticker != nil {
			if err := download(evtType+"."+getTypeOf(sticker), sticker, sticker.GetMimetype(), evt, rawEvt); err == nil {
				return
			}
		} else if contact := evt.Message.GetContactMessage(); contact != nil {
			if err := download(evtType+"."+getTypeOf(contact), contact, "text/vcard", evt, rawEvt); err == nil {
				return
			}
		}

		if evt.Message.GetLocationMessage() != nil {
			if err := saveEvent(evtType+".LocationMessage", rawEvt, nil); err != nil {
				logger.Errorf("Error persisting event type=%s error=%v", evtType, err)
			}
			return
		}

		if evt.Message.GetLiveLocationMessage() != nil {
			if err := saveEvent(evtType+".LiveLocationMessage", rawEvt, nil); err != nil {
				logger.Errorf("Error persisting event type=%s error=%v", evtType, err)
			}
			return
		}

		if err := saveEvent(evtType, rawEvt, nil); err != nil {
			logger.Errorf("Error persisting event type=%s error=%v", evtType, err)
		}

		// Message Recovery Logic
		if !evt.Info.IsFromMe {
			if pm := evt.Message.GetProtocolMessage(); pm != nil {
				isRevoke := pm.GetType() == waE2E.ProtocolMessage_REVOKE
				isEdit := pm.GetType() == waE2E.ProtocolMessage_MESSAGE_EDIT

				if (isRevoke && cfg.IsRecoverDeletesEnabled()) || (isEdit && cfg.IsRecoverEditsEnabled()) {
					if targetID := pm.GetKey().GetID(); targetID != "" {
						go recoverAndNotify(evt, targetID, pm.GetType())
					}
				}
			}
		}
		return

	case *events.Receipt:
		receiptType := map[types.ReceiptType]string{
			types.ReceiptTypeRead:      "ReceiptRead",
			types.ReceiptTypeReadSelf:  "ReceiptReadSelf",
			types.ReceiptTypeDelivered: "ReceiptDelivered",
			types.ReceiptTypePlayed:    "ReceiptPlayed",
			types.ReceiptTypeSender:    "ReceiptSender",
			types.ReceiptTypeRetry:     "ReceiptRetry",
		}
		// Persist receipt event (already in serial worker — no extra goroutine needed).
		if name, ok := receiptType[evt.Type]; ok {
			if err := saveEvent(name, rawEvt, nil); err != nil {
				logger.Errorf("Error persisting receipt event type=%s error=%v", name, err)
			}
		}
		// Calculate receipt latency for delivered/read receipts (reads DB, isolate).
		if evt.Type == types.ReceiptTypeDelivered || evt.Type == types.ReceiptTypeRead {
			go recordReceiptLatency(evt)
		}
		// NotifyProbeReceipt is called in handler() before enqueueing — no-op here.
		return

	case *events.Presence:
		presenceType := evtType + ".Online"
		if evt.Unavailable {
			if evt.LastSeen.IsZero() {
				presenceType = evtType + ".Offline"
			} else {
				presenceType = evtType + ".OfflineLastSeen"
			}
		}
		if err := saveEvent(presenceType, rawEvt, nil); err != nil {
			logger.Errorf("Error persisting presence event type=%s error=%v", presenceType, err)
		}
		return

	case *events.ChatPresence:
		presType := "ChatPresence." + string(evt.State)
		if err := saveEvent(presType, rawEvt, nil); err != nil {
			logger.Errorf("Error persisting chat presence event type=%s error=%v", presType, err)
		}
		return

	case *events.GroupInfo:
		go recordGroupMembership(evt)
		if err := saveEvent(evtType, rawEvt, nil); err != nil {
			logger.Errorf("Error persisting group info event type=%s error=%v", evtType, err)
		}
		return

	case *events.HistorySync:
		id := atomic.AddInt32(&historySyncID, 1)
		fileName := fmt.Sprintf("%s/history-%d-%d.json", *historyPath, startupTime, id)
		file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			if err := saveError(evtType, "Failed to open history file", rawEvt); err != nil {
				logger.Errorf("Error persisting error type=%s error=%v", evtType, err)
			}
			return
		}
		enc := json.NewEncoder(file)
		enc.SetIndent("", "  ")
		if err := enc.Encode(evt.Data); err != nil {
			if err := saveError(evtType, "Failed to encode history JSON", rawEvt); err != nil {
				logger.Errorf("Error persisting error type=%s error=%v", evtType, err)
			}
			_ = file.Close()
			return
		}
		_ = file.Close()
		if err := saveEvent(evtType, rawEvt, nil); err != nil {
			logger.Errorf("Error persisting event type=%s error=%v", evtType, err)
		}
		return
	}

	if err := saveEvent(evtType, rawEvt, nil); err != nil {
		logger.Errorf("Error persisting event type=%s error=%v", evtType, err)
	}
}

// recoverAndNotify finds the original message in the DB and notifies the user.
func recoverAndNotify(evt *events.Message, targetID string, protocolType waE2E.ProtocolMessage_Type) {
	// Wait a bit for the original message to be persisted if it just arrived
	time.Sleep(2 * time.Second)

	var originalEvent struct {
		Type string `db:"type"`
		Raw  string `db:"raw"`
	}

	err := pb.DB().Select("type", "raw").
		From("events").
		Where(dbx.HashExp{"msgID": targetID}).
		One(&originalEvent)

	if err != nil {
		logger.Warnf("Message recovery: original message %s not found in DB: %v", targetID, err)
		return
	}

	location := "private chat"
	if evt.Info.IsGroup {
		location = fmt.Sprintf("group %s", evt.Info.Chat)
	}

	// Extract content from original message
	var rawMsg events.Message
	if err := json.Unmarshal([]byte(originalEvent.Raw), &rawMsg); err != nil {
		logger.Warnf("Message recovery: failed to unmarshal original message: %v", err)
		return
	}

	contentBefore := getMsg(&rawMsg)
	if contentBefore == "" {
		contentBefore = fmt.Sprintf("[%s]", originalEvent.Type)
	}

	var notification string
	if protocolType == waE2E.ProtocolMessage_MESSAGE_EDIT {
		contentAfter := ""
		if pm := evt.Message.GetProtocolMessage(); pm != nil && pm.GetEditedMessage() != nil {
			// Create a temporary Message object to use getMsg
			tempMsg := events.Message{Message: pm.GetEditedMessage()}
			contentAfter = getMsg(&tempMsg)
		}
		notification = fmt.Sprintf("🚨 *Message edited*\n\n*From:* %s\n*Where:* %s\n*Before:* %s\n*After:* %s",
			evt.Info.PushName, location, contentBefore, contentAfter)
	} else {
		notification = fmt.Sprintf("🚨 *Message deleted*\n\n*From:* %s\n*Where:* %s\n*Original Content:* %s",
			evt.Info.PushName, location, contentBefore)
	}

	selfJID, _ := ParseJID(client.Store.ID.User)
	_, _, _ = SendConversationMessage(selfJID, notification, nil)
}

// recordReceiptLatency looks up the original sent message and records delivery/read latency.
func recordReceiptLatency(evt *events.Receipt) {
	defer func() { recover() }() //nolint:errcheck

	col, err := pb.FindCollectionByNameOrId("receipt_latency")
	if err != nil {
		return
	}

	receiptAt := evt.Timestamp
	receiptAtStr := receiptAt.UTC().Format(time.RFC3339)

	for _, msgID := range evt.MessageIDs {
		// Look up when the message was sent (created timestamp in events table)
		type sentRow struct {
			Created string `db:"created"`
		}
		var rows []sentRow
		_ = pb.DB().NewQuery(
			"SELECT created FROM events WHERE msgID = {:msgid} AND type LIKE '%Message%' ORDER BY created ASC LIMIT 1",
		).Bind(dbx.Params{"msgid": msgID}).All(&rows)

		var latencyMs int64
		var sentAt string
		if len(rows) > 0 {
			sentAt = rows[0].Created
			// Parse PocketBase datetime format: "2006-01-02 15:04:05.000Z"
			t, parseErr := time.Parse("2006-01-02 15:04:05.000Z", rows[0].Created)
			if parseErr != nil {
				t, parseErr = time.Parse("2006-01-02 15:04:05.999Z", rows[0].Created)
			}
			if parseErr == nil {
				latencyMs = receiptAt.UnixMilli() - t.UnixMilli()
			}
		}

		record := core.NewRecord(col)
		record.Set("msg_id", msgID)
		record.Set("chat_jid", evt.Chat.String())
		record.Set("receipt_type", string(evt.Type))
		record.Set("latency_ms", latencyMs)
		record.Set("sent_at", sentAt)
		record.Set("receipt_at", receiptAtStr)
		_ = pb.Save(record)
	}
}

// reconnectWithBackoff retries client.Connect with exponential backoff (5s, 10s, 20s … up to 5min).
// Only one goroutine runs at a time; concurrent calls return immediately.
func reconnectWithBackoff(reason string) {
	if !atomic.CompareAndSwapInt32(&reconnecting, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&reconnecting, 0)

	const maxDelay = 5 * time.Minute
	delay := 5 * time.Second
	for attempt := 1; ; attempt++ {
		logger.Infof("Reconnecting after %s attempt=%d delay=%v", reason, attempt, delay)
		time.Sleep(delay)
		if client.IsConnected() {
			logger.Infof("Already connected, stopping reconnect loop reason=%s attempt=%d", reason, attempt)
			return
		}
		if err := client.Connect(); err != nil {
			logger.Errorf("Reconnect failed reason=%s attempt=%d error=%v", reason, attempt, err)
			delay = time.Duration(math.Min(float64(delay*2), float64(maxDelay)))
			continue
		}
		logger.Infof("Reconnected successfully reason=%s attempt=%d", reason, attempt)
		return
	}
}
