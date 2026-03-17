package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/lichti/zaplab/internal/config"
	"github.com/lichti/zaplab/internal/simulation"
	"github.com/lichti/zaplab/internal/webhook"
	"github.com/lichti/zaplab/internal/whatsapp"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	"rsc.io/qr"
)

const mediaBodyLimit = 50 * 1024 * 1024 // 50 MB

var pb *pocketbase.PocketBase
var wh *webhook.Config
var cfg *config.Config
var staticFS fs.FS

// Init injects the PocketBase instance, webhook config, general config, and static file system.
func Init(pbApp *pocketbase.PocketBase, webhookCfg *webhook.Config, generalCfg *config.Config, staticFiles fs.FS) {
	pb = pbApp
	wh = webhookCfg
	cfg = generalCfg
	staticFS = staticFiles
	if err := initDBExplorer(whatsapp.GetDBAddress(), whatsapp.GetDBDialect()); err != nil {
		pb.Logger().Warn("DB Explorer init failed", "error", err)
	}
}

// RegisterRoutes registers all HTTP API routes on the serve event router.
func RegisterRoutes(e *core.ServeEvent) error {
	auth := requireAuth()

	// Redirects
	e.Router.GET("/", func(e *core.RequestEvent) error {
		return e.Redirect(http.StatusTemporaryRedirect, "/zaplab/tools/")
	})
	e.Router.GET("/zaplab", func(e *core.RequestEvent) error {
		return e.Redirect(http.StatusTemporaryRedirect, "/zaplab/tools/")
	})

	// Public routes
	e.Router.GET("/zaplab/api/health", getHealth)
	e.Router.GET("/zaplab/api/wa/status", getWAStatus)
	e.Router.GET("/zaplab/api/wa/qrcode", getWAQRCode)

	// Protected routes
	e.Router.GET("/zaplab/api/wa/account", getWAAccount).Bind(auth)
	e.Router.GET("/zaplab/api/ping", getPing).Bind(auth)
	e.Router.POST("/zaplab/api/cmd", postSendCmd).Bind(auth)
	e.Router.POST("/zaplab/api/sendmessage", postSendMessage).Bind(auth)
	e.Router.POST("/zaplab/api/sendimage", postSendImage).Bind(auth, apis.BodyLimit(mediaBodyLimit))
	e.Router.POST("/zaplab/api/sendvideo", postSendVideo).Bind(auth, apis.BodyLimit(mediaBodyLimit))
	e.Router.POST("/zaplab/api/sendaudio", postSendAudio).Bind(auth, apis.BodyLimit(mediaBodyLimit))
	e.Router.POST("/zaplab/api/senddocument", postSendDocument).Bind(auth, apis.BodyLimit(mediaBodyLimit))
	e.Router.POST("/zaplab/api/sendraw", postSendRaw).Bind(auth)
	e.Router.POST("/zaplab/api/sendlocation", postSendLocation).Bind(auth)
	e.Router.POST("/zaplab/api/sendelivelocation", postSendLiveLocation).Bind(auth)
	e.Router.POST("/zaplab/api/setdisappearing", postSetDisappearing).Bind(auth)
	e.Router.POST("/zaplab/api/sendreaction", postSendReaction).Bind(auth)
	e.Router.POST("/zaplab/api/editmessage", postEditMessage).Bind(auth)
	e.Router.POST("/zaplab/api/revokemessage", postRevokeMessage).Bind(auth)
	e.Router.POST("/zaplab/api/settyping", postSetTyping).Bind(auth)
	e.Router.POST("/zaplab/api/sendcontact", postSendContact).Bind(auth)
	e.Router.POST("/zaplab/api/sendcontacts", postSendContacts).Bind(auth)
	e.Router.POST("/zaplab/api/createpoll", postCreatePoll).Bind(auth)
	e.Router.POST("/zaplab/api/votepoll", postVotePoll).Bind(auth)
	e.Router.POST("/zaplab/api/media/download", postMediaDownload).Bind(auth, apis.BodyLimit(mediaBodyLimit))
	e.Router.POST("/zaplab/api/spoof/reply", postSpoofReply).Bind(auth)
	e.Router.POST("/zaplab/api/spoof/reply-private", postSpoofReplyPrivate).Bind(auth)
	e.Router.POST("/zaplab/api/spoof/reply-img", postSpoofReplyImg).Bind(auth, apis.BodyLimit(mediaBodyLimit))
	e.Router.POST("/zaplab/api/spoof/reply-location", postSpoofReplyLocation).Bind(auth)
	e.Router.POST("/zaplab/api/spoof/timed", postSpoofTimed).Bind(auth)
	e.Router.POST("/zaplab/api/spoof/demo", postSpoofDemo).Bind(auth, apis.BodyLimit(mediaBodyLimit))
	e.Router.GET("/zaplab/api/contacts", getContacts).Bind(auth)
	e.Router.POST("/zaplab/api/contacts/check", postContactsCheck).Bind(auth)
	e.Router.GET("/zaplab/api/contacts/{jid}", getContactInfo).Bind(auth)
	e.Router.GET("/zaplab/api/groups", getGroups).Bind(auth)
	e.Router.GET("/zaplab/api/groups/{jid}", getGroupInfo).Bind(auth)
	e.Router.GET("/zaplab/api/groups/{jid}/participants", getGroupParticipants).Bind(auth)
	e.Router.POST("/zaplab/api/groups", postCreateGroup).Bind(auth)
	e.Router.POST("/zaplab/api/groups/{jid}/participants", postGroupParticipants).Bind(auth)
	e.Router.PATCH("/zaplab/api/groups/{jid}", patchGroup).Bind(auth)
	e.Router.POST("/zaplab/api/groups/{jid}/photo", postGroupPhoto).Bind(auth, apis.BodyLimit(mediaBodyLimit))
	e.Router.POST("/zaplab/api/groups/{jid}/leave", postLeaveGroup).Bind(auth)
	e.Router.GET("/zaplab/api/groups/{jid}/invitelink", getGroupInviteLink).Bind(auth)
	e.Router.POST("/zaplab/api/groups/join", postJoinGroup).Bind(auth)
	e.Router.POST("/zaplab/api/wa/logout", postWALogout).Bind(auth)
	e.Router.POST("/zaplab/api/wa/qrtext", postQRText).Bind(auth)
	e.Router.POST("/zaplab/api/simulate/route", postSimulateRoute).Bind(auth)
	e.Router.DELETE("/zaplab/api/simulate/route/{id}", deleteSimulateRoute).Bind(auth)
	e.Router.GET("/zaplab/api/simulate/route", getSimulateRoutes).Bind(auth)
	e.Router.GET("/zaplab/api/webhook", getWebhookConfig).Bind(auth)
	e.Router.PUT("/zaplab/api/webhook/default", putWebhookDefault).Bind(auth)
	e.Router.DELETE("/zaplab/api/webhook/default", deleteWebhookDefault).Bind(auth)
	e.Router.PUT("/zaplab/api/webhook/error", putWebhookError).Bind(auth)
	e.Router.DELETE("/zaplab/api/webhook/error", deleteWebhookError).Bind(auth)
	e.Router.GET("/zaplab/api/webhook/events", getWebhookEvents).Bind(auth)
	e.Router.POST("/zaplab/api/webhook/events", postWebhookEvent).Bind(auth)
	e.Router.DELETE("/zaplab/api/webhook/events", deleteWebhookEvent).Bind(auth)
	e.Router.GET("/zaplab/api/webhook/text", getWebhookText).Bind(auth)
	e.Router.POST("/zaplab/api/webhook/text", postWebhookText).Bind(auth)
	e.Router.DELETE("/zaplab/api/webhook/text", deleteWebhookText).Bind(auth)
	e.Router.POST("/zaplab/api/webhook/test", postWebhookTest).Bind(auth)
	e.Router.GET("/zaplab/api/config", getConfig).Bind(auth)
	e.Router.PUT("/zaplab/api/config", putConfig).Bind(auth)
	e.Router.GET("/zaplab/api/db/tables", getDBTables).Bind(auth)
	e.Router.GET("/zaplab/api/db/tables/{table}", getDBTable).Bind(auth)
	e.Router.PATCH("/zaplab/api/db/tables/{table}/{rowid}", patchDBTableRow).Bind(auth)
	e.Router.DELETE("/zaplab/api/db/tables/{table}/{rowid}", deleteDBTableRow).Bind(auth)
	e.Router.POST("/zaplab/api/db/reconnect", postDBReconnect).Bind(auth)
	e.Router.POST("/zaplab/api/db/backup", postDBBackup).Bind(auth)
	e.Router.GET("/zaplab/api/db/backups", getDBBackups).Bind(auth)
	e.Router.POST("/zaplab/api/db/restore", postDBRestore).Bind(auth)
	e.Router.DELETE("/zaplab/api/db/backups/{name}", deleteDBBackup).Bind(auth)
	e.Router.GET("/zaplab/api/proto/schema", getProtoSchema).Bind(auth)
	e.Router.GET("/zaplab/api/proto/message", getProtoMessage).Bind(auth)
	e.Router.GET("/zaplab/api/frames", getFrames).Bind(auth)
	e.Router.GET("/zaplab/api/frames/ring", getFramesRing).Bind(auth)
	e.Router.GET("/zaplab/api/frames/modules", getFramesModules).Bind(auth)
	e.Router.GET("/zaplab/api/wa/keys", getDeviceKeys).Bind(auth)

	// Signal session visualizer
	e.Router.GET("/zaplab/api/signal/sessions", getSignalSessions).Bind(auth)
	e.Router.GET("/zaplab/api/signal/senderkeys", getSignalSenderKeys).Bind(auth)

	// Annotations
	e.Router.GET("/zaplab/api/annotations", getAnnotations).Bind(auth)
	e.Router.POST("/zaplab/api/annotations", postAnnotation).Bind(auth)
	e.Router.PATCH("/zaplab/api/annotations/{id}", patchAnnotation).Bind(auth)
	e.Router.DELETE("/zaplab/api/annotations/{id}", deleteAnnotation).Bind(auth)

	// Stats & heatmap
	e.Router.GET("/zaplab/api/stats/heatmap", getStatsHeatmap).Bind(auth)
	e.Router.GET("/zaplab/api/stats/daily", getStatsDaily).Bind(auth)
	e.Router.GET("/zaplab/api/stats/types", getStatsTypes).Bind(auth)
	e.Router.GET("/zaplab/api/stats/summary", getStatsSummary).Bind(auth)
	e.Router.GET("/zaplab/api/stats/editchain", getStatsEditChain).Bind(auth)

	// App State Inspector
	e.Router.GET("/zaplab/api/appstate/collections", getAppStateCollections).Bind(auth)
	e.Router.GET("/zaplab/api/appstate/synckeys", getAppStateSyncKeys).Bind(auth)
	e.Router.GET("/zaplab/api/appstate/mutations", getAppStateMutations).Bind(auth)

	// PCAP export
	e.Router.GET("/zaplab/api/frames/pcap", getFramesPCAP).Bind(auth)

	// Network graph
	e.Router.GET("/zaplab/api/network/graph", getNetworkGraph).Bind(auth)

	e.Router.GET("/zaplab/tools/{path...}", apis.Static(staticFS, false))

	return nil
}

// requireAuth validates that the request is made by a logged-in PocketBase user
// OR contains a valid X-API-Token header.
func requireAuth() *hook.Handler[*core.RequestEvent] {
	apiToken := os.Getenv("API_TOKEN")
	if apiToken == "" {
		pb.Logger().Warn("API_TOKEN env var not set — X-API-Token auth will not be available")
	}
	return &hook.Handler[*core.RequestEvent]{
		Func: func(e *core.RequestEvent) error {
			// 1. Check for PocketBase auth session (JWT in Authorization header)
			if e.Auth != nil {
				return e.Next()
			}

			// 2. Check for X-API-Token header
			if apiToken != "" && e.Request.Header.Get("X-API-Token") == apiToken {
				return e.Next()
			}

			return apis.NewUnauthorizedError("Authentication required", nil)
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

// parseReplyToWithMentions creates a ReplyInfo from a reply_to block and a top-level mentions list.
// Returns nil only when both reply and mentions are absent.
func parseReplyToWithMentions(r *replyToRequest, mentions []string) *whatsapp.ReplyInfo {
	hasReply := r != nil && r.MessageID != ""
	hasMentions := len(mentions) > 0
	if !hasReply && !hasMentions {
		return nil
	}
	info := &whatsapp.ReplyInfo{MentionedJIDs: mentions}
	if hasReply {
		senderJID, _ := whatsapp.ParseJID(r.SenderJID)
		info.MessageID = r.MessageID
		info.Sender = senderJID
		info.Text = r.Text
	}
	return info
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
		To       string          `json:"to"`
		Message  string          `json:"message"`
		ReplyTo  *replyToRequest `json:"reply_to"`
		Mentions []string        `json:"mentions"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	toJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "To field is not a valid"})
	}
	msg, resp, err := whatsapp.SendConversationMessage(toJID, req.Message, parseReplyToWithMentions(req.ReplyTo, req.Mentions))
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
		To       string          `json:"to"`
		Message  string          `json:"message"`
		Image    string          `json:"image"`
		ReplyTo  *replyToRequest `json:"reply_to"`
		Mentions []string        `json:"mentions"`
		ViewOnce bool            `json:"view_once"`
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
	msg, resp, err := whatsapp.SendImage(toJID, data, req.Message, parseReplyToWithMentions(req.ReplyTo, req.Mentions), req.ViewOnce)
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
		To       string          `json:"to"`
		Message  string          `json:"message"`
		Video    string          `json:"video"`
		ReplyTo  *replyToRequest `json:"reply_to"`
		Mentions []string        `json:"mentions"`
		ViewOnce bool            `json:"view_once"`
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
	msg, resp, err := whatsapp.SendVideo(toJID, data, req.Message, parseReplyToWithMentions(req.ReplyTo, req.Mentions), req.ViewOnce)
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

func postGroupPhoto(e *core.RequestEvent) error {
	jidStr := e.Request.PathValue("jid")
	groupJID, ok := whatsapp.ParseJID(jidStr)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "Invalid group JID"})
	}
	var req struct {
		Image string `json:"image"` // base64 JPEG or PNG
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.Image == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "image is required (base64 JPEG or PNG)"})
	}
	data, err := base64ToBytes(req.Image)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "Error decoding image"})
	}
	pictureID, err := whatsapp.SetGroupPhoto(groupJID, data)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": fmt.Sprintf("Error setting group photo: %v", err)})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "Group photo updated", "picture_id": pictureID})
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

// ── Media download & decrypt ──────────────────────────────────────────────────

func postMediaDownload(e *core.RequestEvent) error {
	var req struct {
		URL       string `json:"url"`
		MediaKey  string `json:"media_key"`
		MediaType string `json:"media_type"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.URL == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "url is required"})
	}
	if req.MediaKey == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "media_key is required"})
	}
	if req.MediaType == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "media_type is required (image, video, audio, document, sticker)"})
	}
	result, err := whatsapp.DownloadAndDecryptMedia(e.Request.Context(), req.URL, req.MediaKey, req.MediaType)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": err.Error()})
	}
	disposition := fmt.Sprintf(`attachment; filename="media%s"`, result.Ext)
	e.Response.Header().Set("Content-Disposition", disposition)
	return e.Stream(http.StatusOK, result.MimeType, bytes.NewReader(result.Data))
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

// ── Spoof endpoints ───────────────────────────────────────────────────────────

func parseSpoofBase(e *core.RequestEvent, to, fromJID *string) (whatsapp.JID, whatsapp.JID, bool, error) {
	chatJID, ok := whatsapp.ParseJID(*to)
	if !ok {
		return whatsapp.JID{}, whatsapp.JID{}, false, e.JSON(http.StatusBadRequest, map[string]any{"message": "to is not a valid JID"})
	}
	spoofJID, ok := whatsapp.ParseJID(*fromJID)
	if !ok {
		return whatsapp.JID{}, whatsapp.JID{}, false, e.JSON(http.StatusBadRequest, map[string]any{"message": "from_jid is not a valid JID"})
	}
	return chatJID, spoofJID, true, nil
}

func resolveMsgID(msgID string) string {
	if msgID == "" {
		return whatsapp.GetClient().GenerateMessageID()
	}
	return msgID
}

func postSpoofReply(e *core.RequestEvent) error {
	var req struct {
		To         string `json:"to"`
		FromJID    string `json:"from_jid"`
		MsgID      string `json:"msg_id"`
		QuotedText string `json:"quoted_text"`
		Text       string `json:"text"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	chatJID, spoofJID, ok, jsonErr := parseSpoofBase(e, &req.To, &req.FromJID)
	if !ok {
		return jsonErr
	}
	msg, resp, err := whatsapp.SpoofReply(chatJID, spoofJID, resolveMsgID(req.MsgID), req.QuotedText, req.Text)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "Spoofed reply sent", "whatsapp_message": msg, "send_response": resp})
}

func postSpoofReplyPrivate(e *core.RequestEvent) error {
	var req struct {
		To         string `json:"to"`
		FromJID    string `json:"from_jid"`
		MsgID      string `json:"msg_id"`
		QuotedText string `json:"quoted_text"`
		Text       string `json:"text"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	chatJID, spoofJID, ok, jsonErr := parseSpoofBase(e, &req.To, &req.FromJID)
	if !ok {
		return jsonErr
	}
	msg, resp, err := whatsapp.SpoofReplyPrivate(chatJID, spoofJID, resolveMsgID(req.MsgID), req.QuotedText, req.Text)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "Spoofed private reply sent", "whatsapp_message": msg, "send_response": resp})
}

func postSpoofReplyImg(e *core.RequestEvent) error {
	var req struct {
		To         string `json:"to"`
		FromJID    string `json:"from_jid"`
		MsgID      string `json:"msg_id"`
		Image      string `json:"image"`
		QuotedText string `json:"quoted_text"`
		Text       string `json:"text"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	chatJID, spoofJID, ok, jsonErr := parseSpoofBase(e, &req.To, &req.FromJID)
	if !ok {
		return jsonErr
	}
	if req.Image == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "image (base64) is required"})
	}
	imgData, err := base64ToBytes(req.Image)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "Error decoding image"})
	}
	msg, resp, err := whatsapp.SpoofReplyImg(chatJID, spoofJID, resolveMsgID(req.MsgID), imgData, req.QuotedText, req.Text)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "Spoofed image reply sent", "whatsapp_message": msg, "send_response": resp})
}

func postSpoofReplyLocation(e *core.RequestEvent) error {
	var req struct {
		To      string `json:"to"`
		FromJID string `json:"from_jid"`
		MsgID   string `json:"msg_id"`
		Text    string `json:"text"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	chatJID, spoofJID, ok, jsonErr := parseSpoofBase(e, &req.To, &req.FromJID)
	if !ok {
		return jsonErr
	}
	msg, resp, err := whatsapp.SpoofReplyLocation(chatJID, spoofJID, resolveMsgID(req.MsgID), req.Text)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "Spoofed location reply sent", "whatsapp_message": msg, "send_response": resp})
}

func postSpoofTimed(e *core.RequestEvent) error {
	var req struct {
		To   string `json:"to"`
		Text string `json:"text"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	if req.Text == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "text is required"})
	}
	chatJID, ok := whatsapp.ParseJID(req.To)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "to is not a valid JID"})
	}
	msg, resp, err := whatsapp.SendTimedMessage(chatJID, req.Text)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"message": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "Timed message sent", "whatsapp_message": msg, "send_response": resp})
}

func postSpoofDemo(e *core.RequestEvent) error {
	var req struct {
		To       string `json:"to"`
		FromJID  string `json:"from_jid"`
		Gender   string `json:"gender"`
		Language string `json:"language"`
		Image    string `json:"image"`
	}
	if err := e.BindBody(&req); err != nil {
		return apis.NewBadRequestError("Failed to read request data", err)
	}
	chatJID, spoofJID, ok, jsonErr := parseSpoofBase(e, &req.To, &req.FromJID)
	if !ok {
		return jsonErr
	}
	if req.Gender != "boy" && req.Gender != "girl" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "gender must be 'boy' or 'girl'"})
	}
	if req.Language != "br" && req.Language != "en" {
		return e.JSON(http.StatusBadRequest, map[string]any{"message": "language must be 'br' or 'en'"})
	}
	var imgData []byte
	if req.Image != "" {
		var err error
		imgData, err = base64ToBytes(req.Image)
		if err != nil {
			return e.JSON(http.StatusBadRequest, map[string]any{"message": "Error decoding image"})
		}
	}
	go whatsapp.SpoofDemo(chatJID, spoofJID, req.Gender, req.Language, imgData)
	return e.JSON(http.StatusOK, map[string]any{"message": fmt.Sprintf("Demo started (%s/%s)", req.Gender, req.Language)})
}

// ─── Webhook management ────────────────────────────────────────────────────────

type webhookConfigSummary struct {
	DefaultURL    string                        `json:"default_url"`
	ErrorURL      string                        `json:"error_url"`
	EventWebhooks []webhook.EventTypeWebhookAPI `json:"event_webhooks"`
	TextWebhooks  []webhook.TextWebhookAPI      `json:"text_webhooks"`
}

func getWebhookConfig(e *core.RequestEvent) error {
	return e.JSON(http.StatusOK, webhookConfigSummary{
		DefaultURL:    wh.GetDefaultWebhook().String(),
		ErrorURL:      wh.GetErrorWebhook().String(),
		EventWebhooks: wh.GetEventWebhooks(),
		TextWebhooks:  wh.GetTextWebhooks(),
	})
}

func putWebhookDefault(e *core.RequestEvent) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := e.BindBody(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "invalid body"})
	}
	if req.URL == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "url is required"})
	}
	if err := wh.SetDefaultWebhook(req.URL); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "default webhook updated", "url": req.URL})
}

func deleteWebhookDefault(e *core.RequestEvent) error {
	if err := wh.ClearDefaultWebhook(); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "default webhook cleared"})
}

func putWebhookError(e *core.RequestEvent) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := e.BindBody(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "invalid body"})
	}
	if req.URL == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "url is required"})
	}
	if err := wh.SetErrorWebhook(req.URL); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "error webhook updated", "url": req.URL})
}

func deleteWebhookError(e *core.RequestEvent) error {
	if err := wh.ClearErrorWebhook(); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "error webhook cleared"})
}

func getWebhookEvents(e *core.RequestEvent) error {
	return e.JSON(http.StatusOK, map[string]any{"event_webhooks": wh.GetEventWebhooks()})
}

func postWebhookEvent(e *core.RequestEvent) error {
	var req struct {
		EventType string `json:"event_type"`
		URL       string `json:"url"`
	}
	if err := e.BindBody(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "invalid body"})
	}
	if req.EventType == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "event_type is required"})
	}
	if req.URL == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "url is required"})
	}
	if err := wh.AddEventWebhook(req.EventType, req.URL); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "event webhook saved", "event_type": req.EventType})
}

func deleteWebhookEvent(e *core.RequestEvent) error {
	var req struct {
		EventType string `json:"event_type"`
	}
	if err := e.BindBody(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "invalid body"})
	}
	if req.EventType == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "event_type is required"})
	}
	if err := wh.RemoveEventWebhook(req.EventType); err != nil {
		return e.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "event webhook removed", "event_type": req.EventType})
}

func getWebhookText(e *core.RequestEvent) error {
	return e.JSON(http.StatusOK, map[string]any{"text_webhooks": wh.GetTextWebhooks()})
}

func postWebhookText(e *core.RequestEvent) error {
	var req struct {
		MatchType     string `json:"match_type"`
		Pattern       string `json:"pattern"`
		From          string `json:"from"`
		CaseSensitive bool   `json:"case_sensitive"`
		URL           string `json:"url"`
	}
	if err := e.BindBody(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "invalid body"})
	}
	if err := wh.AddTextWebhook(req.MatchType, req.Pattern, req.From, req.CaseSensitive, req.URL); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "text webhook added"})
}

func deleteWebhookText(e *core.RequestEvent) error {
	var req struct {
		ID string `json:"id"`
	}
	if err := e.BindBody(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "invalid body"})
	}
	if req.ID == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "id is required"})
	}
	if err := wh.RemoveTextWebhook(req.ID); err != nil {
		return e.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "text webhook removed"})
}

func postWebhookTest(e *core.RequestEvent) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := e.BindBody(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "invalid body"})
	}
	if req.URL == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "url is required"})
	}
	testPayload := map[string]any{"test": true, "source": "zaplab", "message": "webhook test payload"}
	if err := wh.SendTo(req.URL, "Test", testPayload, nil); err != nil {
		return e.JSON(http.StatusBadGateway, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"message": "test payload delivered", "url": req.URL})
}

func getConfig(e *core.RequestEvent) error {
	return e.JSON(http.StatusOK, cfg)
}

func putConfig(e *core.RequestEvent) error {
	var req struct {
		RecoverEdits   *bool `json:"recover_edits"`
		RecoverDeletes *bool `json:"recover_deletes"`
	}
	if err := e.BindBody(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "invalid body"})
	}
	if req.RecoverEdits != nil {
		if err := cfg.SetRecoverEdits(*req.RecoverEdits); err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
	}
	if req.RecoverDeletes != nil {
		if err := cfg.SetRecoverDeletes(*req.RecoverDeletes); err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
	}
	return e.JSON(http.StatusOK, cfg)
}
