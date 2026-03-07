package whatsapp

import (
	"context"
	"errors"
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// AccountInfo holds the connected account details returned by GetAccountInfo.
type AccountInfo struct {
	JID          string `json:"jid"`
	Phone        string `json:"phone"`
	PushName     string `json:"push_name"`
	BusinessName string `json:"business_name"`
	Platform     string `json:"platform"`
	Status       string `json:"status"`
	AvatarURL    string `json:"avatar_url"`
}

// GetAccountInfo collects account details from the local device store and the
// WhatsApp servers (profile picture + about text). It returns an error when the
// client is not connected.
func GetAccountInfo() (*AccountInfo, error) {
	if client == nil || client.Store == nil || client.Store.ID == nil {
		return nil, fmt.Errorf("not connected")
	}

	jid := *client.Store.ID
	info := &AccountInfo{
		JID:          jid.String(),
		Phone:        jid.User,
		PushName:     client.Store.PushName,
		BusinessName: client.Store.BusinessName,
		Platform:     client.Store.Platform,
	}

	// About/status text
	userInfoMap, err := client.GetUserInfo(context.Background(), []types.JID{jid})
	if err == nil {
		if ui, ok := userInfoMap[jid]; ok {
			info.Status = ui.Status
		}
	} else {
		logger.Debugf("GetAccountInfo: GetUserInfo failed: %v", err)
	}

	// Profile picture URL
	pic, err := client.GetProfilePictureInfo(context.Background(), jid, &whatsmeow.GetProfilePictureParams{Preview: false})
	if err == nil && pic != nil {
		info.AvatarURL = pic.URL
	} else if err != nil &&
		!errors.Is(err, whatsmeow.ErrProfilePictureNotSet) &&
		!errors.Is(err, whatsmeow.ErrProfilePictureUnauthorized) {
		logger.Debugf("GetAccountInfo: GetProfilePictureInfo failed: %v", err)
	}

	return info, nil
}
