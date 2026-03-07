package whatsapp

import (
	"context"
	"sync"
)

// ConnectionStatus represents the WhatsApp connection state exposed to the API.
type ConnectionStatus string

const (
	StatusConnecting   ConnectionStatus = "connecting"
	StatusConnected    ConnectionStatus = "connected"
	StatusQR           ConnectionStatus = "qr"
	StatusTimeout      ConnectionStatus = "timeout"
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusLoggedOut    ConnectionStatus = "loggedout"
)

var (
	connMu     sync.RWMutex
	connStatus = StatusDisconnected
	lastQR     string
)

func setConnStatus(s ConnectionStatus) {
	connMu.Lock()
	connStatus = s
	connMu.Unlock()
}

func setLastQR(code string) {
	connMu.Lock()
	lastQR = code
	connMu.Unlock()
}

// GetConnectionStatus returns the current WhatsApp connection status.
func GetConnectionStatus() ConnectionStatus {
	connMu.RLock()
	defer connMu.RUnlock()
	return connStatus
}

// GetQRCode returns the latest QR code string (empty when none is available).
func GetQRCode() string {
	connMu.RLock()
	defer connMu.RUnlock()
	return lastQR
}

// GetConnectedJID returns the JID string of the paired account, or empty string if not connected.
func GetConnectedJID() string {
	if client == nil || client.Store == nil || client.Store.ID == nil {
		return ""
	}
	return client.Store.ID.String()
}

// Logout logs out and clears the WhatsApp session.
func Logout() error {
	if client == nil {
		return nil
	}
	return client.Logout(context.Background())
}
