package whatsapp

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// JID is an alias for types.JID, exported so callers don't need to import go.mau.fi/whatsmeow/types directly.
type JID = types.JID

// GetJoinedGroups returns all groups the bot is a member of.
func GetJoinedGroups() ([]*types.GroupInfo, error) {
	return client.GetJoinedGroups(context.Background())
}

// GetGroupInfo returns detailed info about a single group.
func GetGroupInfo(jid types.JID) (*types.GroupInfo, error) {
	return client.GetGroupInfo(context.Background(), jid)
}

// CreateGroup creates a new WhatsApp group with the given name and participants.
// Group names are limited to 25 characters by WhatsApp.
func CreateGroup(name string, participants []types.JID) (*types.GroupInfo, error) {
	req := whatsmeow.ReqCreateGroup{
		Name:         name,
		Participants: participants,
	}
	return client.CreateGroup(context.Background(), req)
}

// UpdateGroupParticipants adds, removes, promotes, or demotes participants.
// action must be one of: "add", "remove", "promote", "demote".
func UpdateGroupParticipants(jid types.JID, participants []types.JID, action string) ([]types.GroupParticipant, error) {
	var change whatsmeow.ParticipantChange
	switch action {
	case "add":
		change = whatsmeow.ParticipantChangeAdd
	case "remove":
		change = whatsmeow.ParticipantChangeRemove
	case "promote":
		change = whatsmeow.ParticipantChangePromote
	case "demote":
		change = whatsmeow.ParticipantChangeDemote
	default:
		return nil, fmt.Errorf("invalid action %q: must be add, remove, promote, or demote", action)
	}
	return client.UpdateGroupParticipants(context.Background(), jid, participants, change)
}

// SetGroupName changes the group's name/subject.
func SetGroupName(jid types.JID, name string) error {
	return client.SetGroupName(context.Background(), jid, name)
}

// SetGroupTopic changes the group's description/topic.
func SetGroupTopic(jid types.JID, topic string) error {
	// previousID and newID are optional; passing empty strings lets the server handle them.
	return client.SetGroupTopic(context.Background(), jid, "", "", topic)
}

// SetGroupAnnounce toggles announce-only mode (only admins can send).
func SetGroupAnnounce(jid types.JID, announce bool) error {
	return client.SetGroupAnnounce(context.Background(), jid, announce)
}

// SetGroupLocked toggles locked mode (only admins can edit group info).
func SetGroupLocked(jid types.JID, locked bool) error {
	return client.SetGroupLocked(context.Background(), jid, locked)
}

// LeaveGroup makes the bot leave the specified group.
func LeaveGroup(jid types.JID) error {
	return client.LeaveGroup(context.Background(), jid)
}

// GetGroupInviteLink returns the invite link for a group.
// Pass reset=true to revoke the current link and generate a new one.
func GetGroupInviteLink(jid types.JID, reset bool) (string, error) {
	return client.GetGroupInviteLink(context.Background(), jid, reset)
}

// JoinGroupWithLink joins a group using an invite link code.
// code can be the full chat.whatsapp.com URL or just the code portion.
func JoinGroupWithLink(code string) (types.JID, error) {
	return client.JoinGroupWithLink(context.Background(), code)
}
