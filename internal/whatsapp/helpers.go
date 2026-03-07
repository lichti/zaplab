package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// ParseJID converts a string to a JID.
func ParseJID(arg string) (types.JID, bool) {
	if arg == "" {
		logger.Debugf("ParseJID called with empty string")
		return types.JID{}, false
	}
	if arg[0] == '+' {
		arg = arg[1:]
	}
	if !strings.ContainsRune(arg, '@') {
		return types.NewJID(arg, types.DefaultUserServer), true
	}
	recipient, err := types.ParseJID(arg)
	if err != nil {
		logger.Debugf("Invalid JID %s: %v", arg, err)
		return recipient, false
	}
	if recipient.User == "" {
		logger.Debugf("Invalid JID %s: no server specified", arg)
		return recipient, false
	}
	return recipient, true
}

func getMsg(evt *events.Message) string {
	if evt.Message.Conversation != nil {
		return *evt.Message.Conversation
	}
	if evt.Message.ExtendedTextMessage != nil {
		return *evt.Message.ExtendedTextMessage.Text
	}
	return ""
}

func download(evtType string, file interface{}, mimetype string, evt *events.Message, rawEvt interface{}) error {
	if file == nil {
		return errors.New("file is nil")
	}
	exts, _ := mime.ExtensionsByType(mimetype)
	ext := ""
	if len(exts) > 0 {
		ext = exts[0]
	}
	fileName := fmt.Sprintf("%s%s", evt.Info.ID, ext)
	if mimetype == "text/vcard" {
		dataVcard := file.(*waE2E.ContactMessage)
		return saveEventFile(evtType, rawEvt, nil, fileName, []byte(*dataVcard.Vcard))
	}
	data, err := client.Download(context.Background(), file.(whatsmeow.DownloadableMessage))
	if err != nil {
		if serr := saveError(evtType, fmt.Sprintf("%s Failed to download", evtType), rawEvt); serr != nil {
			logger.Errorf("Error persisting download error type=%s error=%v", evtType, serr)
		}
		return err
	}
	if err := saveEventFile(evtType, rawEvt, nil, fileName, data); err != nil {
		if serr := saveError(evtType, fmt.Sprintf("%s Failed to save event", evtType), rawEvt); serr != nil {
			logger.Errorf("Error persisting save error type=%s error=%v", evtType, serr)
		}
		return err
	}
	return nil
}

func getTypeOf(i interface{}) string {
	iType := fmt.Sprintf("%T", i)
	strType := strings.Split(iType, ".")
	return strType[len(strType)-1]
}
