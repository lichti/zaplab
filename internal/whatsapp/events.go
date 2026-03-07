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

	"go.mau.fi/util/random"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

var historySyncID int32
var startupTime = time.Now().Unix()

func handler(rawEvt interface{}) {
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

	case *events.Connected, *events.PushNameSetting:
		setConnStatus(StatusConnected)
		setLastQR("")
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
		return

	case *events.Disconnected:
		setConnStatus(StatusDisconnected)
		logger.Warnf("WhatsApp disconnected, reconnecting with backoff... reason=%v", evt)
		go reconnectWithBackoff("disconnect")
		return

	case *events.StreamReplaced:
		if err := saveError(evtType, "Stream replaced", rawEvt); err != nil {
			logger.Errorf("Error persisting StreamReplaced error: %v", err)
		}
		logger.Warnf("Stream replaced by another client, reconnecting with backoff...")
		go reconnectWithBackoff("stream replaced")
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
				if err := saveEvent(evtType+"."+getTypeOf(evt.Message.GetEncReactionMessage()), rawEvt, decrypted); err != nil {
					logger.Errorf("Error persisting event type=%s error=%v", evtType, err)
				}
			}
			return
		}

		mediaHandlers := []struct {
			msg     interface{}
			mime    string
			subType string
		}{
			{evt.Message.GetImageMessage(), evt.Message.GetImageMessage().GetMimetype(), getTypeOf(evt.Message.GetImageMessage())},
			{evt.Message.GetAudioMessage(), evt.Message.GetAudioMessage().GetMimetype(), getTypeOf(evt.Message.GetAudioMessage())},
			{evt.Message.GetVideoMessage(), evt.Message.GetVideoMessage().GetMimetype(), getTypeOf(evt.Message.GetVideoMessage())},
			{evt.Message.GetDocumentMessage(), evt.Message.GetDocumentMessage().GetMimetype(), getTypeOf(evt.Message.GetDocumentMessage())},
			{evt.Message.GetStickerMessage(), evt.Message.GetStickerMessage().GetMimetype(), getTypeOf(evt.Message.GetStickerMessage())},
			{evt.Message.GetContactMessage(), "text/vcard", getTypeOf(evt.Message.GetContactMessage())},
		}
		for _, m := range mediaHandlers {
			if m.msg == nil {
				continue
			}
			if err := download(evtType+"."+m.subType, m.msg, m.mime, evt, rawEvt); err == nil {
				return
			}
			break
		}

		if evt.Message.GetLocationMessage() != nil {
			if err := saveEvent(evtType, rawEvt, nil); err != nil {
				logger.Errorf("Error persisting event type=%s error=%v", evtType, err)
			}
			return
		}

		if err := saveEvent(evtType, rawEvt, nil); err != nil {
			logger.Errorf("Error persisting event type=%s error=%v", evtType, err)
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
		if name, ok := receiptType[evt.Type]; ok {
			if err := saveEvent(name, rawEvt, nil); err != nil {
				logger.Errorf("Error persisting receipt event type=%s error=%v", name, err)
			}
		}
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

// reconnectWithBackoff retries client.Connect with exponential backoff (5s, 10s, 20s … up to 5min).
func reconnectWithBackoff(reason string) {
	const maxDelay = 5 * time.Minute
	delay := 5 * time.Second
	for attempt := 1; ; attempt++ {
		logger.Infof("Reconnecting after %s attempt=%d delay=%v", reason, attempt, delay)
		time.Sleep(delay)
		if err := client.Connect(); err != nil {
			logger.Errorf("Reconnect failed reason=%s attempt=%d error=%v", reason, attempt, err)
			delay = time.Duration(math.Min(float64(delay*2), float64(maxDelay)))
			continue
		}
		logger.Infof("Reconnected successfully reason=%s attempt=%d", reason, attempt)
		return
	}
}
