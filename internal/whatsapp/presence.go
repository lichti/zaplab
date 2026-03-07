package whatsapp

import (
	"context"
	"time"

	"go.mau.fi/whatsmeow/types"
)

// SetTyping sends a typing or recording presence indicator to a chat.
// state: "composing" (show indicator) or "paused" (stop indicator).
// media: "text" (typing indicator) or "audio" (voice recording indicator).
// media is only relevant when state is "composing".
func SetTyping(chat types.JID, state, media string) error {
	presenceState := types.ChatPresencePaused
	if state == "composing" {
		presenceState = types.ChatPresenceComposing
	}
	presenceMedia := types.ChatPresenceMediaText
	if media == "audio" {
		presenceMedia = types.ChatPresenceMediaAudio
	}
	return client.SendChatPresence(context.Background(), chat, presenceState, presenceMedia)
}

// SetDisappearing sets the auto-delete (disappearing messages) timer for a chat.
// timerSeconds: 0 (off), 86400 (24h), 604800 (7d), 7776000 (90d).
// WhatsApp only accepts the four official values — non-standard durations may be rejected.
func SetDisappearing(chat types.JID, timerSeconds uint32) error {
	return client.SetDisappearingTimer(context.Background(), chat, time.Duration(timerSeconds)*time.Second, time.Time{})
}
