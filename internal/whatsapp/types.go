package whatsapp

import (
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
)

type sentMessagePayload struct {
	Message  *waE2E.Message          `json:"message"`
	Response *whatsmeow.SendResponse `json:"response"`
}

type sentErrorPayload struct {
	Message  *waE2E.Message          `json:"message"`
	Response *whatsmeow.SendResponse `json:"response"`
	Error    error                   `json:"error"`
}
