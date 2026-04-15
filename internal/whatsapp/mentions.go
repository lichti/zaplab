package whatsapp

import (
	"github.com/pocketbase/pocketbase/core"
	"go.mau.fi/whatsmeow/types/events"
)

// DetectAndRecordMentions inspects an incoming message for @mentions and persists
// each mention to the mentions collection. Marks is_bot=true when the mentioned
// JID matches the connected device's own JID.
func DetectAndRecordMentions(evt *events.Message) {
	if shuttingDown.Load() || pb == nil || pb.DB() == nil || client == nil {
		return
	}

	// Mentions live in ContextInfo, which can come from several message types.
	var mentionedJIDs []string
	if ci := evt.Message.GetExtendedTextMessage().GetContextInfo(); ci != nil {
		mentionedJIDs = append(mentionedJIDs, ci.GetMentionedJID()...)
	}
	if ci := evt.Message.GetImageMessage().GetContextInfo(); ci != nil {
		mentionedJIDs = append(mentionedJIDs, ci.GetMentionedJID()...)
	}
	if ci := evt.Message.GetVideoMessage().GetContextInfo(); ci != nil {
		mentionedJIDs = append(mentionedJIDs, ci.GetMentionedJID()...)
	}
	if len(mentionedJIDs) == 0 {
		return
	}

	col, err := pb.FindCollectionByNameOrId("mentions")
	if err != nil {
		return
	}

	botJID := ""
	if client.Store.ID != nil {
		botJID = client.Store.ID.User + "@s.whatsapp.net"
	}

	// Extract a short context snippet (first 200 chars of message text).
	text := getMsg(evt)
	if len(text) > 200 {
		text = text[:200]
	}

	senderJID := evt.Info.Sender.String()
	chatJID := evt.Info.Chat.String()
	msgID := evt.Info.ID

	for _, mentioned := range mentionedJIDs {
		isBot := mentioned == botJID
		r := core.NewRecord(col)
		r.Set("message_id", msgID)
		r.Set("chat_jid", chatJID)
		r.Set("sender_jid", senderJID)
		r.Set("mentioned_jid", mentioned)
		r.Set("is_bot", isBot)
		r.Set("context_text", text)
		if err := pb.Save(r); err != nil && !shuttingDown.Load() {
			logger.Warnf("DetectAndRecordMentions: save failed: %v", err)
			continue
		}
		if isBot {
			go CreateNotification(
				"mention",
				"You were mentioned",
				senderJID+" in "+chatJID+": "+text,
				r.Id,
				senderJID,
				map[string]any{
					"message_id": msgID,
					"chat_jid":   chatJID,
					"sender_jid": senderJID,
					"context":    text,
				},
			)
		}
	}
}
