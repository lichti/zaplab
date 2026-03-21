package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// normalizeLID replaces @lid JIDs in a Message event with the corresponding
// phone-number JID from the whatsmeow LID map.  Called before persisting so
// that all storage is keyed on the canonical PN JID.  No-op when the mapping
// is not yet known — the event is stored as-is and will be fixed by migration.
func normalizeLID(msg *events.Message) {
	if client == nil || msg == nil {
		return
	}
	ctx := context.Background()
	if msg.Info.Sender.Server == types.HiddenUserServer {
		if pn, err := client.Store.LIDs.GetPNForLID(ctx, msg.Info.Sender); err == nil && !pn.IsEmpty() {
			msg.Info.Sender = pn
		}
	}
	// DMs where the chat itself is a LID (rare but possible).
	if msg.Info.Chat.Server == types.HiddenUserServer {
		if pn, err := client.Store.LIDs.GetPNForLID(ctx, msg.Info.Chat); err == nil && !pn.IsEmpty() {
			msg.Info.Chat = pn
		}
	}
}

func saveEventFile(evtType string, raw interface{}, extra interface{}, fileName string, fileBytes []byte) error {
	if shuttingDown.Load() || pb.DB() == nil {
		return nil
	}
	if msg, ok := raw.(*events.Message); ok {
		normalizeLID(msg)
	}
	collection, err := pb.FindCollectionByNameOrId("events")
	if err != nil {
		return err
	}

	record := core.NewRecord(collection)

	var msgID string
	switch rawType := raw.(type) {
	case *events.Message:
		msgID = rawType.Info.ID
	case sentMessagePayload:
		msgID = rawType.Response.ID
	}

	record.Set("type", evtType)
	record.Set("raw", raw)
	record.Set("extra", extra)
	record.Set("msgID", msgID)

	file, err := filesystem.NewFileFromBytes(fileBytes, fileName)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	record.Set("file", file)

	if err := wh.SendToDefault(evtType, raw, nil); err != nil && wh.HasDefaultWebhook() {
		logger.Warnf("Failed to send event to default webhook type=%s error=%v", evtType, err)
	}
	wh.SendToEventWebhooks(evtType, raw, nil)

	if err := pb.Save(record); err != nil {
		return fmt.Errorf("failed to save record: %w", err)
	}
	ssePublish(evtType, raw)
	if TriggerDispatchFunc != nil {
		if rawBytes, err := json.Marshal(raw); err == nil {
			go func() {
				defer func() { recover() }() //nolint:errcheck
				TriggerDispatchFunc(evtType, rawBytes)
			}()
		}
	}
	return nil
}

func saveEvent(evtType string, raw interface{}, extra interface{}) error {
	if shuttingDown.Load() || pb.DB() == nil {
		return nil
	}
	if msg, ok := raw.(*events.Message); ok {
		normalizeLID(msg)
	}
	collection, err := pb.FindCollectionByNameOrId("events")
	if err != nil {
		return err
	}

	var msgID string
	switch rawType := raw.(type) {
	case *events.Message:
		msgID = rawType.Info.ID
	case sentMessagePayload:
		msgID = rawType.Response.ID
	}

	record := core.NewRecord(collection)
	record.Set("type", evtType)
	record.Set("raw", raw)
	record.Set("extra", extra)
	record.Set("msgID", msgID)

	if err := wh.SendToDefault(evtType, raw, nil); err != nil && wh.HasDefaultWebhook() {
		logger.Warnf("Failed to send event to default webhook type=%s error=%v", evtType, err)
	}
	wh.SendToEventWebhooks(evtType, raw, nil)

	if err := pb.Save(record); err != nil {
		return fmt.Errorf("failed to save record: %w", err)
	}
	ssePublish(evtType, raw)
	if TriggerDispatchFunc != nil {
		if rawBytes, err := json.Marshal(raw); err == nil {
			go func() {
				defer func() { recover() }() //nolint:errcheck
				TriggerDispatchFunc(evtType, rawBytes)
			}()
		}
	}
	return nil
}

func saveSentMessage(message *waE2E.Message, response *whatsmeow.SendResponse) error {
	return saveEvent("SentMessage", sentMessagePayload{Message: message, Response: response}, nil)
}

// SaveEvent persists an arbitrary event to the events collection.
// Intended for use by sub-packages (e.g. simulation) that need to record events
// without going through the WhatsApp send path.
func SaveEvent(evtType string, raw interface{}, extra interface{}) error {
	return saveEvent(evtType, raw, extra)
}

func saveError(evtType string, evtError string, raw interface{}) error {
	if shuttingDown.Load() || pb.DB() == nil {
		return nil
	}
	collection, err := pb.FindCollectionByNameOrId("errors")
	if err != nil {
		return err
	}

	record := core.NewRecord(collection)
	record.Set("type", evtType)
	record.Set("raw", raw)
	record.Set("EvtError", evtError)

	if err := wh.SendToError(evtType, raw, evtError); err != nil && wh.HasErrorWebhook() {
		logger.Warnf("Failed to send event to error webhook type=%s error=%v", evtType, err)
	}

	if err := pb.Save(record); err != nil {
		return fmt.Errorf("failed to save record: %w", err)
	}
	return nil
}

func saveSentError(evtError string, message *waE2E.Message, response *whatsmeow.SendResponse, e error) error {
	return saveError("SentMessage", evtError, sentErrorPayload{Message: message, Response: response, Error: e})
}
