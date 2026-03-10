package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/lichti/zaplab/internal/simulation"
	"github.com/lichti/zaplab/internal/whatsapp"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	"rsc.io/qr"
)

const mediaBodyLimit = 50 * 1024 * 1024 // 50 MB

var pb *pocketbase.PocketBase

// Init injects the PocketBase instance needed for the logger.
func Init(pbApp *pocketbase.PocketBase) {
	pb = pbApp
}

// RegisterRoutes registers all HTTP API routes on the serve event router.
func RegisterRoutes(e *core.ServeEvent) error {
	// TODO: re-enable auth by adding .Bind(requireAPIToken()) back to each route
	e.Router.GET("/health", getHealth)
	e.Router.GET("/ping", getPing)
	e.Router.POST("/cmd", postSendCmd)
	e.Router.POST("/sendmessage", postSendMessage)
	e.Router.POST("/sendimage", postSendImage).Bind(apis.BodyLimit(mediaBodyLimit))
	e.Router.POST("/sendvideo", postSendVideo).Bind(apis.BodyLimit(mediaBodyLimit))
	e.Router.POST("/sendaudio", postSendAudio).Bind(apis.BodyLimit(mediaBodyLimit))
	e.Router.POST("/senddocument", postSendDocument).Bind(apis.BodyLimit(mediaBodyLimit))
	e.Router.POST("/sendraw", postSendRaw)
	e.Router.POST("/sendlocation", postSendLocation)
	e.Router.POST("/sendelivelocation", postSendLiveLocation)
	e.Router.POST("/setdisappearing", postSetDisappearing)
	e.Router.POST("/sendreaction", postSendReaction)
	e.Router.POST("/editmessage", postEditMessage)
	e.Router.POST("/revokemessage", postRevokeMessage)
	e.Router.POST("/settyping", postSetTyping)
	e.Router.POST("/sendcontact", postSendContact)
	e.Router.POST("/sendcontacts", postSendContacts)
	e.Router.POST("/createpoll", postCreatePoll)
	e.Router.POST("/votepoll", postVotePoll)
	e.Router.GET("/contacts", getContacts)
	e.Router.POST("/contacts/check", postContactsCheck)
	e.Router.GET("/contacts/{jid}", getContactInfo)
	e.Router.GET("/groups", getGroups)
	e.Router.GET("/groups/{jid}", getGroupInfo)
	e.Router.GET("/groups/{jid}/participants", getGroupParticipants)
	e.Router.POST("/groups", postCreateGroup)
	e.Router.POST("/groups/{jid}/participants", postGroupParticipants)
	e.Router.PATCH("/groups/{jid}", patchGroup)
	e.Router.POST("/groups/{jid}/leave", postLeaveGroup)
	e.Router.GET("/groups/{jid}/invitelink", getGroupInviteLink)
	e.Router.POST("/groups/join", postJoinGroup)
	e.Router.GET("/wa/status", getWAStatus)
	e.Router.GET("/wa/qrcode", getWAQRCode)
	e.Router.POST("/wa/logout", postWALogout)
	e.Router.GET("/wa/account", getWAAccount)
	e.Router.POST("/wa/qrtext", postQRText)
	e.Router.POST("/simulate/route", postSimulateRoute)
	e.Router.DELETE("/simulate/route/{id}", deleteSimulateRoute)
	e.Router.GET("/simulate/route", getSimulateRoutes)
	e.Router.GET("/tools/{path...}", apis.Static(os.DirFS("./pb_public"), false))

	return nil
}

// requireAPIToken validates the X-API-Token header against the API_TOKEN env var.
// If API_TOKEN is not set, all requests are rejected.
func requireAPIToken() *hook.Handler[*core.RequestEvent] {
	token := os.Getenv("API_TOKEN")
	if token == "" {
		pb.Logger().Warn("API_TOKEN env var not set — all API requests will be rejected")
	}
	return &hook.Handler[*core.RequestEvent]{
		Func: func(e *core.RequestEvent) error {
			if token == "" || e.Request.Header.Get("X-API-Token") != token {
				return apis.NewUnauthorizedError("Invalid or missing API token", nil)
			}
			return e.Next()
		},
	}
}

// replyToRequest is the optional reply_to block accepted by all send endpoints.
type replyToRequest struct {
	MessageID string `json:"message_id"`
	SenderJID string `json:"sender_jid"`
	Text      string `json:"quoted_text"`
}

// parseReplyTo converts a *replyToRequest into a *whatsapp.ReplyInfo.
// Returns nil when reply is nil or has no MessageID.
func parseReplyTo(r *replyToRequest) *whatsapp.ReplyInfo {
	if r == nil || r.MessageID == "" {
		return nil
	}
	senderJID, ok := whatsapp.ParseJID(r.SenderJID)
	if !ok {
		return nil
	}
	return &whatsapp.ReplyInfo{
		MessageID: r.MessageID,
		Sender:    senderJID,
		Text:      r.Text,
	}
}

func base64ToBytes(b64 string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("error decoding base64: %w", err)
	}
	return data, nil
}

func getWAStatus(e *core.RequestEvent) error {
	return e.JSON(http.StatusOK, map[string]any{
		"status": string(whatsapp.GetConnectionStatus()),
		"jid":    whatsapp.GetConnectedJID(),
	})
}

func getWAQRCode(e *core.RequestEvent) error {
	code := whatsapp.GetQRCode()
	if code == "" {
		return e.JSON(http.StatusNotFound, map[string]any{"message": "no QR code available"})
	}
	qrCode, err := qr.Encode(code, qr.L)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "failed to encode QR"})
	}
	b64 := base64.StdEncoding.EncodeToString(qrCode.PNG())
	return e.JSON(http.StatusOK, map[string]any{
		"status": string(whatsapp.GetConnectionStatus()),
		"image":  "data:image/png;base64," + b64,
	})
}

func getWAAccount(e *core.RequestEvent) error {
	info, err := whatsapp.GetAccountInfo()
	if err != nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]any{"message": err.Error()})
	}
	return e.JSON(http.StatusOK, info)
}

func postWALogout(e *core.RequestEvent) error {
	if err := whatsapp.Logout(); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "logged out"})
}

func getHealth(e *core.RequestEvent) error {
	cli := whatsapp.GetClient()
	connected := cli != nil && cli.IsConnected()
	status := map[string]any{
		"pocketbase": "ok",
		"whatsapp":   connected,
	}
	if !connected {
		return e.JSON(http.StatusServiceUnavailable, status)
	}
	return e.JSON(http.StatusOK, status)
}

func getPing(e *core.RequestEvent) error {
	return e.JSON(http.StatusOK, map[string]any{"message": "Pong!"})
}

func postSendCmd(e *core.RequestEvent) error {
	var req struct {
		Cmd  string `json:"cmd"`
		Args string `json:"args"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	args := []string{req.Args}
	output := whatsapp.HandleCmd(req.Cmd, args, nil)
	return e.JSON(http.StatusOK, map[string]any{"message": output})
}

func postSendMessage(e *core.RequestEvent) error {
	var req struct {
		To      string          `json:"to"`
		Message string          `json:"message"`
		ReplyTo *replyToRequest `json:"reply_to"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	toJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "To field is not a valid"})
	}
	msg, resp, err := whatsapp.SendConversationMessage(toJID, req.Message, parseReplyTo(req.ReplyTo))
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "Error to send message"})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":          "Message sent",
		"whatsapp_message": msg,
		"send_response":    resp,
	})
}

func postSendImage(e *core.RequestEvent) error {
	var req struct {
		To      string          `json:"to"`
		Message string          `json:"message"`
		Image   string          `json:"image"`
		ReplyTo *replyToRequest `json:"reply_to"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	data, err := base64ToBytes(req.Image)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "Error to decode image"})
	}
	toJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "To field is not a valid"})
	}
	msg, resp, err := whatsapp.SendImage(toJID, data, req.Message, parseReplyTo(req.ReplyTo))
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "Error to send image message"})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":          "Image message sent",
		"whatsapp_message": msg,
		"send_response":    resp,
	})
}

func postSendVideo(e *core.RequestEvent) error {
	var req struct {
		To      string          `json:"to"`
		Message string          `json:"message"`
		Video   string          `json:"video"`
		ReplyTo *replyToRequest `json:"reply_to"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	data, err := base64ToBytes(req.Video)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "Error to decode video"})
	}
	toJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "To field is not a valid"})
	}
	msg, resp, err := whatsapp.SendVideo(toJID, data, req.Message, parseReplyTo(req.ReplyTo))
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "Error to send video message"})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":          "Video message sent",
		"whatsapp_message": msg,
		"send_response":    resp,
	})
}

func postSendAudio(e *core.RequestEvent) error {
	var req struct {
		To      string          `json:"to"`
		Audio   string          `json:"audio"`
		PTT     bool            `json:"ptt"`
		ReplyTo *replyToRequest `json:"reply_to"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	data, err := base64ToBytes(req.Audio)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "Error to decode audio"})
	}
	toJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "To field is not a valid"})
	}
	msg, resp, err := whatsapp.SendAudio(toJID, data, req.PTT, parseReplyTo(req.ReplyTo))
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "Error to send audio message"})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":          "Audio message sent",
		"whatsapp_message": msg,
		"send_response":    resp,
	})
}

func postSendDocument(e *core.RequestEvent) error {
	var req struct {
		To       string          `json:"to"`
		Message  string          `json:"message"`
		Document string          `json:"document"`
		ReplyTo  *replyToRequest `json:"reply_to"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	data, err := base64ToBytes(req.Document)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "Error to decode document"})
	}
	toJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "To field is not a valid"})
	}
	msg, resp, err := whatsapp.SendDocument(toJID, data, req.Message, parseReplyTo(req.ReplyTo))
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "Error to send document message"})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":          "Document message sent",
		"whatsapp_message": msg,
		"send_response":    resp,
	})
}

func postSendLocation(e *core.RequestEvent) error {
	var req struct {
		To      string          `json:"to"`
		Lat     float64         `json:"latitude"`
		Lon     float64         `json:"longitude"`
		Name    string          `json:"name"`
		Address string          `json:"address"`
		ReplyTo *replyToRequest `json:"reply_to"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	toJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "To field is not a valid"})
	}
	msg, resp, err := whatsapp.SendLocation(toJID, req.Lat, req.Lon, req.Name, req.Address, parseReplyTo(req.ReplyTo))
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "Error sending location"})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":          "Location sent",
		"whatsapp_message": msg,
		"send_response":    resp,
	})
}

func postSendLiveLocation(e *core.RequestEvent) error {
	var req struct {
		To             string          `json:"to"`
		Lat            float64         `json:"latitude"`
		Lon            float64         `json:"longitude"`
		AccuracyMeters uint32          `json:"accuracy_in_meters"`
		SpeedMps       float32         `json:"speed_in_mps"`
		BearingDegrees uint32          `json:"degrees_clockwise_from_magnetic_north"`
		Caption        string          `json:"caption"`
		SequenceNumber int64           `json:"sequence_number"`
		TimeOffset     uint32          `json:"time_offset"`
		ReplyTo        *replyToRequest `json:"reply_to"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	toJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "To field is not a valid"})
	}
	msg, resp, err := whatsapp.SendLiveLocation(toJID, req.Lat, req.Lon, req.AccuracyMeters, req.SpeedMps, req.BearingDegrees, req.Caption, req.SequenceNumber, req.TimeOffset, parseReplyTo(req.ReplyTo), "")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "Error sending live location"})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":          "Live location sent",
		"whatsapp_message": msg,
		"send_response":    resp,
	})
}

func postSetDisappearing(e *core.RequestEvent) error {
	var req struct {
		To    string `json:"to"`
		Timer uint32 `json:"timer"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	chatJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "to field is not a valid JID"})
	}
	// WhatsApp only accepts: 0 (off), 86400 (24h), 604800 (7d), 7776000 (90d)
	allowed := map[uint32]bool{0: true, 86400: true, 604800: true, 7776000: true}
	if !allowed[req.Timer] {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "timer must be 0, 86400, 604800, or 7776000"})
	}
	if err := whatsapp.SetDisappearing(chatJID, req.Timer); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error setting disappearing timer: %v", err)})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "Disappearing timer updated"})
}

func postSendReaction(e *core.RequestEvent) error {
	var req struct {
		To        string `json:"to"`
		MessageID string `json:"message_id"`
		SenderJID string `json:"sender_jid"`
		Emoji     string `json:"emoji"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.MessageID == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "message_id is required"})
	}
	chatJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "to field is not a valid JID"})
	}
	senderJID, ok := whatsapp.ParseJID(req.SenderJID)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "sender_jid field is not a valid JID"})
	}
	msg, resp, err := whatsapp.SendReaction(chatJID, senderJID, req.MessageID, req.Emoji)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "Error sending reaction"})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":          "Reaction sent",
		"whatsapp_message": msg,
		"send_response":    resp,
	})
}

func postEditMessage(e *core.RequestEvent) error {
	var req struct {
		To        string `json:"to"`
		MessageID string `json:"message_id"`
		NewText   string `json:"new_text"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.MessageID == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "message_id is required"})
	}
	if req.NewText == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "new_text is required"})
	}
	chatJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "to field is not a valid JID"})
	}
	msg, resp, err := whatsapp.EditMessage(chatJID, req.MessageID, req.NewText)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "Error editing message"})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":          "Message edited",
		"whatsapp_message": msg,
		"send_response":    resp,
	})
}

func postRevokeMessage(e *core.RequestEvent) error {
	var req struct {
		To        string `json:"to"`
		MessageID string `json:"message_id"`
		SenderJID string `json:"sender_jid"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.MessageID == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "message_id is required"})
	}
	chatJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "to field is not a valid JID"})
	}
	senderJID, ok := whatsapp.ParseJID(req.SenderJID)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "sender_jid field is not a valid JID"})
	}
	msg, resp, err := whatsapp.RevokeMessage(chatJID, senderJID, req.MessageID)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "Error revoking message"})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":          "Message revoked",
		"whatsapp_message": msg,
		"send_response":    resp,
	})
}

func postSetTyping(e *core.RequestEvent) error {
	var req struct {
		To    string `json:"to"`
		State string `json:"state"`
		Media string `json:"media"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.State != "composing" && req.State != "paused" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "state must be 'composing' or 'paused'"})
	}
	chatJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "to field is not a valid JID"})
	}
	if err := whatsapp.SetTyping(chatJID, req.State, req.Media); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "Error setting typing state"})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "Typing state updated"})
}

func postSendContact(e *core.RequestEvent) error {
	var req struct {
		To          string          `json:"to"`
		DisplayName string          `json:"display_name"`
		Vcard       string          `json:"vcard"`
		ReplyTo     *replyToRequest `json:"reply_to"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.Vcard == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "vcard is required"})
	}
	toJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "to field is not a valid JID"})
	}
	msg, resp, err := whatsapp.SendContact(toJID, req.DisplayName, req.Vcard, parseReplyTo(req.ReplyTo))
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "Error sending contact"})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":          "Contact sent",
		"whatsapp_message": msg,
		"send_response":    resp,
	})
}

func postSendContacts(e *core.RequestEvent) error {
	var req struct {
		To          string `json:"to"`
		DisplayName string `json:"display_name"`
		Contacts    []struct {
			Name  string `json:"name"`
			Vcard string `json:"vcard"`
		} `json:"contacts"`
		ReplyTo *replyToRequest `json:"reply_to"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if len(req.Contacts) == 0 {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "contacts array is required and must not be empty"})
	}
	toJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "to field is not a valid JID"})
	}
	contacts := make([]struct{ Name, Vcard string }, len(req.Contacts))
	for i, c := range req.Contacts {
		contacts[i] = struct{ Name, Vcard string }{Name: c.Name, Vcard: c.Vcard}
	}
	msg, resp, err := whatsapp.SendContacts(toJID, req.DisplayName, contacts, parseReplyTo(req.ReplyTo))
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "Error sending contacts"})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":          "Contacts sent",
		"whatsapp_message": msg,
		"send_response":    resp,
	})
}

func postCreatePoll(e *core.RequestEvent) error {
	var req struct {
		To              string   `json:"to"`
		Question        string   `json:"question"`
		Options         []string `json:"options"`
		SelectableCount int      `json:"selectable_count"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.Question == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "question is required"})
	}
	if len(req.Options) < 2 {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "at least 2 options are required"})
	}
	toJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "to field is not a valid JID"})
	}
	msg, resp, err := whatsapp.CreatePoll(toJID, req.Question, req.Options, req.SelectableCount)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "Error creating poll"})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":          "Poll created",
		"whatsapp_message": msg,
		"send_response":    resp,
	})
}

func postVotePoll(e *core.RequestEvent) error {
	var req struct {
		To              string   `json:"to"`
		PollMessageID   string   `json:"poll_message_id"`
		PollSenderJID   string   `json:"poll_sender_jid"`
		SelectedOptions []string `json:"selected_options"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.PollMessageID == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "poll_message_id is required"})
	}
	if len(req.SelectedOptions) == 0 {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "selected_options must not be empty"})
	}
	chatJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "to field is not a valid JID"})
	}
	pollSenderJID, ok := whatsapp.ParseJID(req.PollSenderJID)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "poll_sender_jid is not a valid JID"})
	}
	msg, resp, err := whatsapp.VotePoll(chatJID, pollSenderJID, req.PollMessageID, req.SelectedOptions)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error voting on poll: %v", err)})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":          "Vote cast",
		"whatsapp_message": msg,
		"send_response":    resp,
	})
}

func getGroups(e *core.RequestEvent) error {
	groups, err := whatsapp.GetJoinedGroups()
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error fetching groups: %v", err)})
	}
	return e.JSON(http.StatusOK, map[string]any{"groups": groups})
}

func getGroupInfo(e *core.RequestEvent) error {
	jidStr := e.Request.PathValue("jid")
	groupJID, ok := whatsapp.ParseJID(jidStr)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "Invalid group JID"})
	}
	info, err := whatsapp.GetGroupInfo(groupJID)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error fetching group info: %v", err)})
	}
	return e.JSON(http.StatusOK, info)
}

func postCreateGroup(e *core.RequestEvent) error {
	var req struct {
		Name         string   `json:"name"`
		Participants []string `json:"participants"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.Name == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "name is required"})
	}
	if len(req.Participants) == 0 {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "participants must not be empty"})
	}
	participantJIDs := make([]whatsapp.JID, 0, len(req.Participants))
	for _, p := range req.Participants {
		jid, ok := whatsapp.ParseJID(p)
		if !ok {
			return e.JSON(http.StatusBadRequest, map[string]any{"message": fmt.Sprintf("Invalid participant JID: %s", p)})
		}
		participantJIDs = append(participantJIDs, jid)
	}
	info, err := whatsapp.CreateGroup(req.Name, participantJIDs)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error creating group: %v", err)})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "Group created", "group": info})
}

func postGroupParticipants(e *core.RequestEvent) error {
	jidStr := e.Request.PathValue("jid")
	groupJID, ok := whatsapp.ParseJID(jidStr)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "Invalid group JID"})
	}
	var req struct {
		Action       string   `json:"action"`
		Participants []string `json:"participants"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	allowed := map[string]bool{"add": true, "remove": true, "promote": true, "demote": true}
	if !allowed[req.Action] {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "action must be add, remove, promote, or demote"})
	}
	if len(req.Participants) == 0 {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "participants must not be empty"})
	}
	participantJIDs := make([]whatsapp.JID, 0, len(req.Participants))
	for _, p := range req.Participants {
		jid, ok := whatsapp.ParseJID(p)
		if !ok {
			return e.JSON(http.StatusBadRequest, map[string]any{"message": fmt.Sprintf("Invalid participant JID: %s", p)})
		}
		participantJIDs = append(participantJIDs, jid)
	}
	results, err := whatsapp.UpdateGroupParticipants(groupJID, participantJIDs, req.Action)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error updating participants: %v", err)})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "Participants updated", "results": results})
}

func patchGroup(e *core.RequestEvent) error {
	jidStr := e.Request.PathValue("jid")
	groupJID, ok := whatsapp.ParseJID(jidStr)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "Invalid group JID"})
	}
	var req struct {
		Name     *string `json:"name"`
		Topic    *string `json:"topic"`
		Announce *bool   `json:"announce"`
		Locked   *bool   `json:"locked"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.Name == nil && req.Topic == nil && req.Announce == nil && req.Locked == nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "at least one field (name, topic, announce, locked) is required"})
	}
	if req.Name != nil {
		if err := whatsapp.SetGroupName(groupJID, *req.Name); err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error setting group name: %v", err)})
		}
	}
	if req.Topic != nil {
		if err := whatsapp.SetGroupTopic(groupJID, *req.Topic); err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error setting group topic: %v", err)})
		}
	}
	if req.Announce != nil {
		if err := whatsapp.SetGroupAnnounce(groupJID, *req.Announce); err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error setting announce mode: %v", err)})
		}
	}
	if req.Locked != nil {
		if err := whatsapp.SetGroupLocked(groupJID, *req.Locked); err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error setting locked mode: %v", err)})
		}
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "Group updated"})
}

func postLeaveGroup(e *core.RequestEvent) error {
	jidStr := e.Request.PathValue("jid")
	groupJID, ok := whatsapp.ParseJID(jidStr)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "Invalid group JID"})
	}
	if err := whatsapp.LeaveGroup(groupJID); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error leaving group: %v", err)})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "Left group"})
}

func getGroupInviteLink(e *core.RequestEvent) error {
	jidStr := e.Request.PathValue("jid")
	groupJID, ok := whatsapp.ParseJID(jidStr)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "Invalid group JID"})
	}
	reset := e.Request.URL.Query().Get("reset") == "true"
	link, err := whatsapp.GetGroupInviteLink(groupJID, reset)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error getting invite link: %v", err)})
	}
	return e.JSON(http.StatusOK, map[string]any{"link": link})
}

func postJoinGroup(e *core.RequestEvent) error {
	var req struct {
		Link string `json:"link"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.Link == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "link is required"})
	}
	jid, err := whatsapp.JoinGroupWithLink(req.Link)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error joining group: %v", err)})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "Joined group", "jid": jid.String()})
}

func getGroupParticipants(e *core.RequestEvent) error {
	jidStr := e.Request.PathValue("jid")
	groupJID, ok := whatsapp.ParseJID(jidStr)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "Invalid group JID"})
	}
	info, err := whatsapp.GetGroupInfo(groupJID)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error fetching group info: %v", err)})
	}
	type participant struct {
		JID          string `json:"jid"`
		Phone        string `json:"phone"`
		IsAdmin      bool   `json:"is_admin"`
		IsSuperAdmin bool   `json:"is_super_admin"`
	}
	out := make([]participant, len(info.Participants))
	for i, p := range info.Participants {
		out[i] = participant{
			JID:          p.JID.String(),
			Phone:        p.JID.User,
			IsAdmin:      p.IsAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
		}
	}
	return e.JSON(http.StatusOK, map[string]any{
		"jid":          info.JID.String(),
		"participants": out,
	})
}

func postQRText(e *core.RequestEvent) error {
	var req struct {
		Text string `json:"text"`
	}
	if err := e.BindBody(&req); err != nil || req.Text == "" {
		return apis.NewBadRequestError("text is required", nil)
	}
	qrCode, err := qr.Encode(req.Text, qr.L)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": "failed to encode QR"})
	}
	b64 := base64.StdEncoding.EncodeToString(qrCode.PNG())
	return e.JSON(http.StatusOK, map[string]any{"image": "data:image/png;base64," + b64})
}

func postSendRaw(e *core.RequestEvent) error {
	var req struct {
		To      string          `json:"to"`
		Message json.RawMessage `json:"message"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if len(req.Message) == 0 {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "message field is required"})
	}
	toJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "To field is not a valid"})
	}
	msg, resp, err := whatsapp.SendRaw(toJID, []byte(req.Message))
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":          "Raw message sent",
		"whatsapp_message": msg,
		"send_response":    resp,
	})
}

// ── Contact management ────────────────────────────────────────────────────────

func getContacts(e *core.RequestEvent) error {
	contacts, err := whatsapp.GetAllContacts()
	if err != nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]any{"message": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"contacts": contacts, "count": len(contacts)})
}

func postContactsCheck(e *core.RequestEvent) error {
	var req struct {
		Phones []string `json:"phones"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if len(req.Phones) == 0 {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "phones array is required"})
	}
	results, err := whatsapp.CheckOnWhatsApp(req.Phones)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error checking phones: %v", err)})
	}
	return e.JSON(http.StatusOK, map[string]any{"results": results, "count": len(results)})
}

func getContactInfo(e *core.RequestEvent) error {
	jidStr := e.Request.PathValue("jid")
	jid, ok := whatsapp.ParseJID(jidStr)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "Invalid JID"})
	}
	info, err := whatsapp.GetContactInfo(jid)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error fetching contact info: %v", err)})
	}
	return e.JSON(http.StatusOK, info)
}

// ── Route simulation ──────────────────────────────────────────────────────────

func postSimulateRoute(e *core.RequestEvent) error {
	var req struct {
		To              string  `json:"to"`
		GPXBase64       string  `json:"gpx_base64"`
		SpeedKmh        float64 `json:"speed_kmh"`
		IntervalSeconds float64 `json:"interval_seconds"`
		Caption         string  `json:"caption"`
		MessageID       string  `json:"message_id"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.To == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "to is required"})
	}
	if req.GPXBase64 == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "gpx_base64 is required"})
	}
	if req.MessageID == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "message_id is required — start with POST /sendelivelocation and use the returned send_response.ID"})
	}
	toJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "to field is not a valid JID"})
	}
	sim, err := simulation.Start(toJID, simulation.SimRequest{
		To:              req.To,
		GPXBase64:       req.GPXBase64,
		SpeedKmh:        req.SpeedKmh,
		IntervalSeconds: req.IntervalSeconds,
		Caption:         req.Caption,
		MessageID:       req.MessageID,
	})
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"message":    "Simulation started",
		"simulation": sim,
	})
}

func deleteSimulateRoute(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if !simulation.Stop(id) {
		return e.JSON(http.StatusNotFound, map[string]any{"message": "simulation not found"})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "Simulation stopped"})
}

func getSimulateRoutes(e *core.RequestEvent) error {
	return e.JSON(http.StatusOK, map[string]any{"simulations": simulation.List()})
}
