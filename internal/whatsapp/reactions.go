package whatsapp

import (
	"time"

	"github.com/pocketbase/pocketbase/core"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

// RecordReaction persists a decrypted reaction to the message_reactions collection.
// rm is the result of client.DecryptReaction; orig is the raw EncReactionMessage event.
func RecordReaction(rm *waE2E.ReactionMessage, orig *events.Message) {
	if rm == nil || shuttingDown.Load() || pb == nil || pb.DB() == nil {
		return
	}

	targetMsgID := rm.GetKey().GetID()
	emoji := rm.GetText()
	removed := emoji == ""
	senderJID := orig.Info.Sender.String()
	chatJID := orig.Info.Chat.String()
	reactAt := orig.Info.Timestamp.UTC().Format(time.RFC3339)

	col, err := pb.FindCollectionByNameOrId("message_reactions")
	if err != nil {
		logger.Warnf("RecordReaction: collection not found: %v", err)
		return
	}
	r := core.NewRecord(col)
	r.Set("message_id", targetMsgID)
	r.Set("chat_jid", chatJID)
	r.Set("sender_jid", senderJID)
	r.Set("emoji", emoji)
	r.Set("removed", removed)
	r.Set("react_at", reactAt)
	if err := pb.Save(r); err != nil && !shuttingDown.Load() {
		logger.Warnf("RecordReaction: save failed: %v", err)
	}
}
