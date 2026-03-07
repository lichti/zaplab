package whatsapp

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// HandleCmd dispatches a command string to the appropriate handler.
func HandleCmd(cmd string, args []string, evt *events.Message) (output string) {
	output = "Command not found"
	switch cmd {
	case "set-default-webhook":
		output = cmdSetDefaultWebhook(args)
		logger.Infof("output: %s", output)
	case "set-error-webhook":
		output = cmdSetErrorWebhook(args)
		logger.Infof("output: %s", output)
	case "add-cmd-webhook":
		output = cmdAddCmdWebhook(args)
		logger.Infof("output: %s", output)
	case "rm-cmd-webhook":
		output = cmdRmCmdWebhook(args)
		logger.Infof("output: %s", output)
	case "print-cmd-webhooks-config":
		output = cmdPrintWebhookConfig()
		logger.Infof("output: %s", output)
	case "getgroup":
		output = cmdGetGroup(args)
		logger.Infof("output: %s", output)
	case "listgroups":
		output = cmdListGroups()
		logger.Infof("output: %s", output)
	case "send-spoofed-reply":
		output = cmdSendSpoofedReply(args)
		logger.Infof("output: %s", output)
	case "SendTimedMsg":
		output = cmdSendTimedMsg(args)
		logger.Infof("output: %s", output)
	case "removeOldMsg":
		output = cmdRemoveOldMsg(args)
		logger.Infof("output: %s", output)
	case "editOldMsg":
		output = cmdEditOldMsg(args)
		logger.Infof("output: %s", output)
	case "sendSpoofedReplyMessageInPrivate":
		output = cmdSendSpoofedReplyMessageInPrivate(args)
		logger.Infof("output: %s", output)
	case "send-spoofed-img-reply":
		output = cmdSendSpoofedImgReply(args)
		logger.Infof("output: %s", output)
	case "send-spoofed-demo":
		output = cmdSendSpoofedDemo(args)
		logger.Infof("output: %s", output)
	case "send-spoofed-demo-img":
		output = cmdSendSpoofedDemoImg(args)
		logger.Infof("output: %s", output)
	case "spoofed-reply-this":
		if evt != nil {
			if evt.Message.ExtendedTextMessage != nil {
				if evt.Message.ExtendedTextMessage.ContextInfo != nil {
					if evt.Message.ExtendedTextMessage.ContextInfo.QuotedMessage != nil {
						output = cmdSpoofedReplyThis(args, evt.Message)
						logger.Infof("output: %s", output)
					}
				}
			} else {
				output = "You need use this command replying a message"
				logger.Infof("output: %s", output)
			}
		} else {
			output = "You need to reply a message using your whatsapp client to use this command"
			logger.Infof("output: %s", output)
		}
	}
	return
}

// Webhook commands

func cmdSetDefaultWebhook(args []string) (output string) {
	if len(args) < 1 {
		output = "\n[set-default-webhook] Usage: set-default-webhook <url>"
		logger.Errorf("%s", output)
		return
	}
	url := args[0]
	err := wh.SetDefaultWebhook(url)
	if err != nil {
		output = fmt.Sprintf("\n[set-default-webhook] Failed to set default webhook: %v", err)
		logger.Errorf("%s", output)
		return
	}
	output = fmt.Sprintf("\n[set-default-webhook] Default Webhook set to %s", url)
	logger.Infof("%s", output)
	return
}

func cmdSetErrorWebhook(args []string) (output string) {
	if len(args) < 1 {
		output = "\n[set-error-webhook] Usage: set-error-webhook <url>"
		logger.Errorf("%s", output)
		return
	}
	url := args[0]
	err := wh.SetErrorWebhook(url)
	if err != nil {
		output = fmt.Sprintf("\n[set-error-webhook] Failed to set error webhook: %v", err)
		logger.Errorf("%s", output)
		return
	}
	output = fmt.Sprintf("\n[set-error-webhook] Error Webhook set to %s", url)
	logger.Infof("%s", output)
	return
}

func cmdAddCmdWebhook(args []string) (output string) {
	if len(args) < 1 {
		output = "\n[add-cmd-webhook] Usage: add-cmd-webhook <cmd>|<url>"
		logger.Errorf("%s", output)
		return
	}
	parameters := strings.SplitN(strings.Join(args[0:], " "), "|", 2)
	if len(parameters) < 2 {
		output = "\n[add-cmd-webhook] Usage: add-cmd-webhook <cmd>|<url>"
		logger.Errorf("%s", output)
		return
	}
	cmd := parameters[0]
	url := parameters[1]
	err := wh.AddCmdWebhook(cmd, url)
	if err != nil {
		output = fmt.Sprintf("\n[add-cmd-webhook] Failed to add cmd webhook: %v", err)
		logger.Errorf("%s", output)
		return
	}
	output = fmt.Sprintf("\n[add-cmd-webhook] Webhook added to %s -> %s", cmd, url)
	logger.Infof("%s", output)
	return
}

func cmdRmCmdWebhook(args []string) (output string) {
	if len(args) < 1 {
		output = "\n[rm-cmd-webhook] Usage: rm-cmd-webhook <cmd>"
		logger.Errorf("%s", output)
		return
	}
	parameters := strings.SplitN(strings.Join(args[0:], " "), "|", 1)
	cmd := parameters[0]
	err := wh.RemoveCmdWebhook(cmd)
	if err != nil {
		output = fmt.Sprintf("\n[rm-cmd-webhook] Failed to remove cmd webhook: %v", err)
		logger.Errorf("%s", output)
		return
	}
	output = fmt.Sprintf("\n[rm-cmd-webhook] Webhook removed: %s ", cmd)
	logger.Infof("%s", output)
	return
}

func cmdPrintWebhookConfig() (output string) {
	output = wh.PrintConfig()
	logger.Debugf("%s", output)
	return
}

// Group commands

func cmdGetGroup(args []string) (output string) {
	if len(args) < 1 {
		output = "\n[getgroup] Usage: getgroup <jid>"
		logger.Errorf("%s", output)
		return
	}
	group, ok := ParseJID(args[0])
	if !ok {
		output = "\n[getgroup] You need to specify a valid group JID"
		logger.Errorf("%s", output)
		return
	} else if group.Server != types.GroupServer {
		output = fmt.Sprintf("\n[getgroup] Input must be a group JID (@%s)", types.GroupServer)
		logger.Errorf("%s", output)
		return
	}
	resp, err := client.GetGroupInfo(context.Background(), group)
	if err != nil {
		output = fmt.Sprintf("\n[getgroup] Failed to get group info: %v", err)
		logger.Errorf("%s", output)
		return
	}
	output = fmt.Sprintf("\n[getgroup] Group info: %+v", resp)
	logger.Infof("%s", output)
	return
}

func cmdListGroups() (output string) {
	groups, err := client.GetJoinedGroups(context.Background())
	if err != nil {
		output = fmt.Sprintf("\n[listgroup] Failed to get group list: %v", err)
		logger.Errorf("%s", output)
		return
	}
	for _, group := range groups {
		output = fmt.Sprintf("%s \n[listgroup] %+v: %+v", output, group.GroupName.Name, group.JID)
		logger.Infof("%s", output)
	}
	return
}

// Spoofed commands

func cmdSendSpoofedReply(args []string) (output string) {
	if len(args) < 4 {
		output = "\n[send-spoofed-reply] Usage: send-spoofed-reply <chat_jid> <msgID:!|#ID> <spoofed_jid> <spoofed_text>|<text>"
		logger.Errorf("%s", output)
		return
	}
	chatJID, ok := ParseJID(args[0])
	if !ok {
		output = "\n[send-spoofed-reply] You need to specify a valid Chat ID (Group or User)"
		logger.Errorf("%s", output)
		return
	}
	msgID := args[1]
	if msgID[0] == '!' {
		msgID = client.GenerateMessageID()
	}
	spoofedJID, ok2 := ParseJID(args[2])
	if !ok2 {
		output = "\n[send-spoofed-reply] You need to specify a valid User ID to spoof"
		logger.Errorf("%s", output)
		return
	}
	parameters := strings.SplitN(strings.Join(args[3:], " "), "|", 2)
	spoofedText := parameters[0]
	text := parameters[1]
	_, resp, err := sendSpoofedReplyMessage(chatJID, spoofedJID, msgID, spoofedText, text)
	if err != nil {
		output = fmt.Sprintf("\n[send-spoofed-reply] Error on sending spoofed msg: %v", err)
		logger.Errorf("%s", output)
		return
	}
	output = fmt.Sprintf("\n[send-spoofed-reply] spoofed msg sended: %+v", resp)
	logger.Infof("%s", output)
	return
}

func cmdRemoveOldMsg(args []string) (output string) {
	if len(args) < 2 {
		output = "\n[cmdRemoveOldMsg] Usage: removeOldMsg <chat_jid> <msgID>"
		logger.Errorf("%s", output)
		return
	}
	chatJID, ok := ParseJID(args[0])
	if !ok {
		output = "\n[cmdRemoveOldMsg] You need to specify a valid Chat ID (Group or User)"
		logger.Errorf("%s", output)
		return
	}
	msgID := args[1]
	_, resp, err := removeOldMsg(chatJID, msgID)
	if err != nil {
		output = fmt.Sprintf("\n[cmdRemoveOldMsg] Error on sending spoofed msg: %v", err)
		logger.Errorf("%s", output)
		return
	}
	output = fmt.Sprintf("\n[cmdRemoveOldMsg] spoofed msg sended: %+v", resp)
	logger.Infof("%s", output)
	return
}

func cmdEditOldMsg(args []string) (output string) {
	if len(args) < 3 {
		output = "\n[cmdRemoveOldMsg] Usage: editOldMsg <chat_jid> <msgID> <newMSG>"
		logger.Errorf("%s", output)
		return
	}
	chatJID, ok := ParseJID(args[0])
	if !ok {
		output = "\n[cmdRemoveOldMsg] You need to specify a valid Chat ID (Group or User)"
		logger.Errorf("%s", output)
		return
	}
	msgID := args[1]
	newMsg := strings.Join(args[2:], " ")
	_, resp, err := editOldMsg(chatJID, msgID, newMsg)
	if err != nil {
		output = fmt.Sprintf("\n[cmdRemoveOldMsg] Error on sending spoofed msg: %v", err)
		logger.Errorf("%s", output)
		return
	}
	output = fmt.Sprintf("\n[cmdRemoveOldMsg] spoofed msg sended: %+v", resp)
	logger.Infof("%s", output)
	return
}

func cmdSendTimedMsg(args []string) (output string) {
	if len(args) < 2 {
		output = "\n[sendTimedMsg] Usage: sendTimedMsg <chat_jid> <text>"
		logger.Errorf("%s", output)
		return
	}
	chatJID, ok := ParseJID(args[0])
	if !ok {
		output = "\n[sendTimedMsg] You need to specify a valid Chat ID (Group or User)"
		logger.Errorf("%s", output)
		return
	}
	text := strings.Join(args[1:], " ")
	_, resp, err := sendTimedMsg(chatJID, text)
	if err != nil {
		output = fmt.Sprintf("\n[sendTimedMsg] Error on sending spoofed msg: %v", err)
		logger.Errorf("%s", output)
		return
	}
	output = fmt.Sprintf("\n[sendTimedMsg] spoofed msg sended: %+v", resp)
	logger.Infof("%s", output)
	return
}

func cmdSendSpoofedReplyMessageInPrivate(args []string) (output string) {
	if len(args) < 4 {
		output = "\n[cmdSendSpoofedReplyMessageInPrivate] Usage: cmdSendSpoofedReplyMessageInPrivate <chat_jid> <msgID:!|#ID> <spoofed_jid> <spoofed_text>|<text>"
		logger.Errorf("%s", output)
		return
	}
	chatJID, ok := ParseJID(args[0])
	if !ok {
		output = "\n[cmdSendSpoofedReplyMessageInPrivate] You need to specify a valid Chat ID (Group or User)"
		logger.Errorf("%s", output)
		return
	}
	msgID := args[1]
	if msgID[0] == '!' {
		msgID = client.GenerateMessageID()
	}
	spoofedJID, ok2 := ParseJID(args[2])
	if !ok2 {
		output = "\n[cmdSendSpoofedReplyMessageInPrivate] You need to specify a valid User ID to spoof"
		logger.Errorf("%s", output)
		return
	}
	parameters := strings.SplitN(strings.Join(args[3:], " "), "|", 2)
	spoofedText := parameters[0]
	text := parameters[1]
	_, resp, err := sendSpoofedReplyMessageInPrivate(chatJID, spoofedJID, msgID, spoofedText, text)
	if err != nil {
		output = fmt.Sprintf("\n[cmdSendSpoofedReplyMessageInPrivate] Error on sending spoofed msg: %v", err)
		logger.Errorf("%s", output)
		return
	}
	output = fmt.Sprintf("\n[cmdSendSpoofedReplyMessageInPrivate] spoofed msg sended: %+v", resp)
	logger.Infof("%s", output)
	return
}

func cmdSendSpoofedImgReply(args []string) (output string) {
	if len(args) < 5 {
		output = "\n[send-spoofed-img-reply] Usage: send-spoofed-img-reply <chat_jid> <msgID:!|#ID> <spoofed_jid> <spoofed_file> <spoofed_text>|<text>"
		logger.Errorf("%s", output)
		return
	}
	chatJID, ok := ParseJID(args[0])
	if !ok {
		output = "\n[send-spoofed-img-reply] You need to specify a valid Chat ID (Group or User)"
		logger.Errorf("%s", output)
		return
	}
	msgID := args[1]
	if msgID[0] == '!' {
		msgID = client.GenerateMessageID()
	}
	spoofedJID, ok2 := ParseJID(args[2])
	if !ok2 {
		output = "\n[send-spoofed-img-reply] You need to specify a valid User ID to spoof"
		logger.Errorf("%s", output)
		return
	}
	spoofedFile := args[3]
	parameters := strings.SplitN(strings.Join(args[4:], " "), "|", 2)
	spoofedText := parameters[0]
	text := parameters[1]
	_, resp, err := sendSpoofedReplyImg(chatJID, spoofedJID, msgID, spoofedFile, spoofedText, text)
	if err != nil {
		output = fmt.Sprintf("\n[send-spoofed-img-reply] Error on sending spoofed msg: %v", err)
		logger.Errorf("%s", output)
		return
	}
	output = fmt.Sprintf("\n[send-spoofed-img-reply] spoofed msg sended: %+v", resp)
	logger.Infof("%s", output)
	return
}

func cmdSendSpoofedDemo(args []string) (output string) {
	if len(args) < 4 {
		output = "\n[send-spoofed-demo] Usage: send-spoofed-demo <toGender:boy|girl> <language:br|en> <chat_jid> <spoofed_jid>"
		logger.Errorf("%s", output)
		return
	}
	if args[0] != "boy" && args[0] != "girl" {
		output = "\n[send-spoofed-demo] Error: <boy|girl>"
		logger.Errorf("%s", output)
		return
	}
	toGender := args[0]
	if args[1] != "br" && args[1] != "en" {
		output = "\n[send-spoofed-demo] Error: <br|en>"
		logger.Errorf("%s", output)
		return
	}
	language := args[1]
	chatJID, ok := ParseJID(args[2])
	if !ok {
		output = "\n[send-spoofed-demo] You need to specify a valid Chat ID (Group or User)"
		logger.Errorf("%s", output)
		return
	}
	spoofedJID, ok2 := ParseJID(args[3])
	if !ok2 {
		output = "\n[send-spoofed-demo] You need to specify a valid User ID to spoof"
		logger.Errorf("%s", output)
		return
	}
	sendSpoofedTalkDemo(chatJID, spoofedJID, toGender, language, "")
	output = fmt.Sprintf("\n[send-spoofed-demo] spoofed msg sended to %s as %s", chatJID, spoofedJID)
	return
}

func cmdSendSpoofedDemoImg(args []string) (output string) {
	if len(args) < 5 {
		logger.Errorf("\n[send-spoofed-demo-img] Usage: send-spoofed-demo-img <toGender:boy|girl> <language:br|en> <chat_jid> <spoofed_jid> <spoofed_img>")
		return
	}
	if args[0] != "boy" && args[0] != "girl" {
		output = "\n[send-spoofed-demo-img] Error: <boy|girl>"
		logger.Errorf("%s", output)
		return
	}
	toGender := args[0]
	if args[1] != "br" && args[1] != "en" {
		output = "\n[send-spoofed-demo-img] Error: <br|en>"
		logger.Errorf("%s", output)
		return
	}
	language := args[1]
	chatJID, ok := ParseJID(args[2])
	if !ok {
		output = "\n[send-spoofed-demo-img] You need to specify a valid Chat ID (Group or User)"
		logger.Errorf("%s", output)
		return
	}
	spoofedJID, ok2 := ParseJID(args[3])
	if !ok2 {
		output = "\n[send-spoofed-demo-img] You need to specify a valid User ID to spoof"
		logger.Errorf("%s", output)
		return
	}
	spoofedImg := args[4]
	sendSpoofedTalkDemo(chatJID, spoofedJID, toGender, language, spoofedImg)
	output = fmt.Sprintf("\n[send-spoofed-demo-img] send-spoofed-demo-img: spoofed msg sended to %s as %s", chatJID, spoofedJID)
	return
}

func cmdSpoofedReplyThis(args []string, msg *waE2E.Message) (output string) {
	if len(args) < 4 {
		output = "\n[spoofed-reply-this] Usage: spoofed-reply-this <chat_jid> <msgID:!|#ID> <spoofed_jid> <text>"
		logger.Errorf("%s", output)
		return
	}
	chatJID, ok := ParseJID(args[0])
	if !ok {
		output = "\n[send-spoofed-reply] You need to specify a valid Chat ID (Group or User)"
		logger.Errorf("%s", output)
		return
	}
	msgID := args[1]
	if msgID[0] == '!' {
		msgID = client.GenerateMessageID()
	}
	spoofedJID, ok2 := ParseJID(args[2])
	if !ok2 {
		output = "\n[send-spoofed-reply] You need to specify a valid User ID to spoof"
		logger.Errorf("%s", output)
		return
	}
	text := strings.Join(args[3:], " ")
	_, resp, err := sendSpoofedReplyThis(chatJID, spoofedJID, msgID, text, msg)
	if err != nil {
		output = fmt.Sprintf("\n[reply-spoofed-this] Error on sending spoofed msg: %v", err)
		logger.Errorf("%s", output)
		return
	}
	output = fmt.Sprintf("\n[reply-spoofed-this] spoofed msg sended: %+v", resp)
	logger.Infof("%s", output)
	return
}
