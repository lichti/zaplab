package whatsapp

import (
	"encoding/json"
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

func saveEventFile(evtType string, raw interface{}, extra interface{}, fileName string, fileBytes []byte) error {
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
