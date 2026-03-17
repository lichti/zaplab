package whatsapp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// ReplyInfo holds the data needed to build a quoted-message reply ContextInfo.
type ReplyInfo struct {
	MessageID     string
	Sender        types.JID
	Text          string   // optional: text shown inside the reply bubble
	MentionedJIDs []string // optional: JID strings for @user mentions
}

// buildContextInfo builds a ContextInfo for a reply and/or mentions.
// Returns nil when there is nothing to encode (no reply, no mentions).
func buildContextInfo(r *ReplyInfo) *waE2E.ContextInfo {
	if r == nil {
		return nil
	}
	hasReply := r.MessageID != ""
	hasMentions := len(r.MentionedJIDs) > 0
	if !hasReply && !hasMentions {
		return nil
	}
	ctx := &waE2E.ContextInfo{}
	if hasMentions {
		ctx.MentionedJID = r.MentionedJIDs
	}
	if hasReply {
		ctx.StanzaID = proto.String(r.MessageID)
		ctx.Participant = proto.String(r.Sender.String())
		ctx.QuotedMessage = &waE2E.Message{
			Conversation: proto.String(r.Text),
		}
	}
	return ctx
}

func sendMessage(to types.JID, msg *waE2E.Message) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	resp, err := client.SendMessage(context.Background(), to, msg)
	if err != nil {
		logger.Errorf("Error sending message: %v", err)
		saveSentError("Error sending message", msg, &resp, err)
		return msg, &resp, err
	}
	logger.Infof("Message sent server_timestamp=%v", resp.Timestamp)
	saveSentMessage(msg, &resp)
	return msg, &resp, err
}

func SendConversationMessage(to types.JID, text string, reply *ReplyInfo) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	var msg *waE2E.Message
	if ctx := buildContextInfo(reply); ctx != nil {
		// reply or mentions require ExtendedTextMessage — Conversation has no ContextInfo field
		msg = &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text:        proto.String(text),
				ContextInfo: ctx,
			},
		}
	} else {
		msg = &waE2E.Message{Conversation: proto.String(text)}
	}
	return sendMessage(to, msg)
}

func SendImage(to types.JID, data []byte, caption string, reply *ReplyInfo, viewOnce bool) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	uploaded, err := client.Upload(context.Background(), data, whatsmeow.MediaImage)
	if err != nil {
		saveError("SendImage", "Failed to upload file", &sentErrorPayload{Message: nil, Response: nil, Error: err})
		logger.Debugf("Failed to upload file: %v", err)
		return &waE2E.Message{}, &whatsmeow.SendResponse{}, fmt.Errorf("failed to upload file: %v", err)
	}
	inner := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption:       proto.String(caption),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(http.DetectContentType(data)),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
			ContextInfo:   buildContextInfo(reply),
			ViewOnce:      proto.Bool(viewOnce),
		},
	}
	if viewOnce {
		return sendMessage(to, &waE2E.Message{ViewOnceMessage: &waE2E.FutureProofMessage{Message: inner}})
	}
	return sendMessage(to, inner)
}

func SendAudio(to types.JID, data []byte, isPtt bool, reply *ReplyInfo) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	uploaded, err := client.Upload(context.Background(), data, whatsmeow.MediaAudio)
	if err != nil {
		saveError("SendAudio", "Failed to upload file", &sentErrorPayload{Message: nil, Response: nil, Error: err})
		logger.Debugf("Failed to upload file: %v", err)
		return &waE2E.Message{}, &whatsmeow.SendResponse{}, fmt.Errorf("failed to upload file: %v", err)
	}
	msg := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
			URL:               proto.String(uploaded.URL),
			DirectPath:        proto.String(uploaded.DirectPath),
			MediaKey:          uploaded.MediaKey,
			Mimetype:          proto.String(fmt.Sprint(mimetype.Detect(data).String(), "; codecs=opus")),
			FileEncSHA256:     uploaded.FileEncSHA256,
			FileSHA256:        uploaded.FileSHA256,
			FileLength:        proto.Uint64(uint64(len(data))),
			PTT:               proto.Bool(isPtt),
			MediaKeyTimestamp: proto.Int64(time.Now().Unix()),
			ContextInfo:       buildContextInfo(reply),
		},
	}
	return sendMessage(to, msg)
}

func SendVideo(to types.JID, data []byte, caption string, reply *ReplyInfo, viewOnce bool) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	uploaded, err := client.Upload(context.Background(), data, whatsmeow.MediaVideo)
	if err != nil {
		saveError("SendVideo", "Failed to upload file", &sentErrorPayload{Message: nil, Response: nil, Error: err})
		logger.Debugf("Failed to upload file: %v", err)
		return &waE2E.Message{}, &whatsmeow.SendResponse{}, fmt.Errorf("failed to upload file: %v", err)
	}
	inner := &waE2E.Message{
		VideoMessage: &waE2E.VideoMessage{
			Caption:       proto.String(caption),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(http.DetectContentType(data)),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
			ContextInfo:   buildContextInfo(reply),
			ViewOnce:      proto.Bool(viewOnce),
		},
	}
	if viewOnce {
		return sendMessage(to, &waE2E.Message{ViewOnceMessage: &waE2E.FutureProofMessage{Message: inner}})
	}
	return sendMessage(to, inner)
}

// SendRaw unmarshals msgJSON (a protobuf-JSON encoded waE2E.Message) and sends it as-is.
// This allows sending any arbitrary Message structure supported by the WhatsApp protocol.
func SendRaw(to types.JID, msgJSON []byte) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	var msg waE2E.Message
	if err := protojson.Unmarshal(msgJSON, &msg); err != nil {
		return &waE2E.Message{}, &whatsmeow.SendResponse{}, fmt.Errorf("invalid message JSON: %w", err)
	}
	return sendMessage(to, &msg)
}

// SendReaction sends an emoji reaction to a specific message.
// To remove a reaction, pass an empty string as emoji.
func SendReaction(chat, sender types.JID, messageID, emoji string) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	msg := client.BuildReaction(chat, sender, messageID, emoji)
	return sendMessage(chat, msg)
}

// RevokeMessage deletes a previously sent message for everyone.
// sender is the JID of the original message author.
func RevokeMessage(chat, sender types.JID, messageID string) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	msg := client.BuildRevoke(chat, sender, messageID)
	return sendMessage(chat, msg)
}

// EditMessage edits a previously sent text message.
// Only messages sent by the bot itself can be edited (whatsmeow sets FromMe=true internally).
func EditMessage(chat types.JID, messageID, newText string) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	newContent := &waE2E.Message{Conversation: proto.String(newText)}
	msg := client.BuildEdit(chat, messageID, newContent)
	return sendMessage(chat, msg)
}

// SendDocumentFile sends a document with both a display filename and optional caption.
func SendDocumentFile(to types.JID, data []byte, filename, caption string, reply *ReplyInfo) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	uploaded, err := client.Upload(context.Background(), data, whatsmeow.MediaDocument)
	if err != nil {
		saveError("SendDocumentFile", "Failed to upload file", &sentErrorPayload{Message: nil, Response: nil, Error: err})
		logger.Debugf("Failed to upload file: %v", err)
		return &waE2E.Message{}, &whatsmeow.SendResponse{}, fmt.Errorf("failed to upload file: %v", err)
	}
	msg := &waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			FileName:      proto.String(filename),
			Caption:       proto.String(caption),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(http.DetectContentType(data)),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
			ContextInfo:   buildContextInfo(reply),
		},
	}
	return sendMessage(to, msg)
}

func SendDocument(to types.JID, data []byte, caption string, reply *ReplyInfo) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	uploaded, err := client.Upload(context.Background(), data, whatsmeow.MediaDocument)
	if err != nil {
		saveError("SendDocument", "Failed to upload file", &sentErrorPayload{Message: nil, Response: nil, Error: err})
		logger.Debugf("Failed to upload file: %v", err)
		return &waE2E.Message{}, &whatsmeow.SendResponse{}, fmt.Errorf("failed to upload file: %v", err)
	}
	msg := &waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			Caption:       proto.String(caption),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(http.DetectContentType(data)),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
			ContextInfo:   buildContextInfo(reply),
		},
	}
	return sendMessage(to, msg)
}

// SendLocation sends a static GPS location pin.
func SendLocation(to types.JID, lat, lon float64, name, address string, reply *ReplyInfo) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	msg := &waE2E.Message{
		LocationMessage: &waE2E.LocationMessage{
			DegreesLatitude:  proto.Float64(lat),
			DegreesLongitude: proto.Float64(lon),
			Name:             proto.String(name),
			Address:          proto.String(address),
			ContextInfo:      buildContextInfo(reply),
		},
	}
	return sendMessage(to, msg)
}

// SendContact sends a single vCard contact.
func SendContact(to types.JID, displayName, vcard string, reply *ReplyInfo) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	msg := &waE2E.Message{
		ContactMessage: &waE2E.ContactMessage{
			DisplayName: proto.String(displayName),
			Vcard:       proto.String(vcard),
			ContextInfo: buildContextInfo(reply),
		},
	}
	return sendMessage(to, msg)
}

// SendContacts sends multiple vCard contacts in a single message.
func SendContacts(to types.JID, displayName string, contacts []struct{ Name, Vcard string }, reply *ReplyInfo) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	protoContacts := make([]*waE2E.ContactMessage, len(contacts))
	for i, c := range contacts {
		c := c
		protoContacts[i] = &waE2E.ContactMessage{
			DisplayName: proto.String(c.Name),
			Vcard:       proto.String(c.Vcard),
		}
	}
	msg := &waE2E.Message{
		ContactsArrayMessage: &waE2E.ContactsArrayMessage{
			DisplayName: proto.String(displayName),
			Contacts:    protoContacts,
			ContextInfo: buildContextInfo(reply),
		},
	}
	return sendMessage(to, msg)
}

// CreatePoll creates a WhatsApp poll. selectableCount is the max number of options a voter can choose (0 = unlimited).
func CreatePoll(to types.JID, question string, options []string, selectableCount int) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	msg := client.BuildPollCreation(question, options, selectableCount)
	return sendMessage(to, msg)
}

// VotePoll casts a vote on an existing poll.
// chatJID is the conversation JID, pollSenderJID is who created the poll, pollMessageID is the poll's message ID.
func VotePoll(chatJID, pollSenderJID types.JID, pollMessageID string, selectedOptions []string) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	pollInfo := &types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:   chatJID,
			Sender: pollSenderJID,
		},
		ID: pollMessageID,
	}
	msg, err := client.BuildPollVote(context.Background(), pollInfo, selectedOptions)
	if err != nil {
		return &waE2E.Message{}, &whatsmeow.SendResponse{}, fmt.Errorf("failed to build poll vote: %w", err)
	}
	return sendMessage(chatJID, msg)
}

// SendLiveLocation sends a live GPS location update.
// sequenceNumber should increment with each update. timeOffset is seconds since the initial live location message.
// messageID: when non-empty, reuses that message ID so WhatsApp treats the send as an update to the existing
// live-location share instead of creating a new message.
func SendLiveLocation(to types.JID, lat, lon float64, accuracyMeters uint32, speedMps float32, bearingDegrees uint32, caption string, sequenceNumber int64, timeOffset uint32, reply *ReplyInfo, messageID string) (*waE2E.Message, *whatsmeow.SendResponse, error) {
	msg := &waE2E.Message{
		LiveLocationMessage: &waE2E.LiveLocationMessage{
			DegreesLatitude:                   proto.Float64(lat),
			DegreesLongitude:                  proto.Float64(lon),
			AccuracyInMeters:                  proto.Uint32(accuracyMeters),
			SpeedInMps:                        proto.Float32(speedMps),
			DegreesClockwiseFromMagneticNorth: proto.Uint32(bearingDegrees),
			Caption:                           proto.String(caption),
			SequenceNumber:                    proto.Int64(sequenceNumber),
			TimeOffset:                        proto.Uint32(timeOffset),
			ContextInfo:                       buildContextInfo(reply),
		},
	}
	if messageID != "" {
		resp, err := client.SendMessage(context.Background(), to, msg, whatsmeow.SendRequestExtra{ID: types.MessageID(messageID)})
		if err != nil {
			logger.Errorf("Error sending live location update: %v", err)
			saveSentError("Error sending live location update", msg, &resp, err)
			return msg, &resp, err
		}
		logger.Infof("Live location updated server_timestamp=%v", resp.Timestamp)
		saveSentMessage(msg, &resp)
		return msg, &resp, nil
	}
	return sendMessage(to, msg)
}
