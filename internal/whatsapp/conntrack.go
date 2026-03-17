package whatsapp

import (
	"github.com/pocketbase/pocketbase/core"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// recordConnEvent persists a connection lifecycle event to the conn_events collection.
func recordConnEvent(eventType, reason string) {
	col, err := pb.FindCollectionByNameOrId("conn_events")
	if err != nil {
		return
	}
	jid := ""
	if client != nil && client.Store != nil && client.Store.ID != nil {
		jid = client.Store.ID.String()
	}
	record := core.NewRecord(col)
	record.Set("event_type", eventType)
	record.Set("reason", reason)
	record.Set("jid", jid)
	_ = pb.Save(record)
}

// recordGroupMembership persists participant changes from a GroupInfo event.
func recordGroupMembership(evt *events.GroupInfo) {
	col, err := pb.FindCollectionByNameOrId("group_membership")
	if err != nil {
		return
	}

	groupJID := evt.JID.String()
	actorJID := ""
	if evt.Sender != nil {
		actorJID = evt.Sender.String()
	}

	type memberAction struct {
		jids   []types.JID
		action string
	}

	changes := []memberAction{
		{jids: evt.Join, action: "join"},
		{jids: evt.Leave, action: "leave"},
		{jids: evt.Promote, action: "promote"},
		{jids: evt.Demote, action: "demote"},
	}

	name := ""
	if evt.Name != nil {
		name = evt.Name.Name
	}

	for _, change := range changes {
		for _, memberJID := range change.jids {
			record := core.NewRecord(col)
			record.Set("group_jid", groupJID)
			record.Set("group_name", name)
			record.Set("member_jid", memberJID.String())
			record.Set("action", change.action)
			record.Set("actor_jid", actorJID)
			_ = pb.Save(record)
		}
	}
}
