package whatsapp

import (
	"fmt"

	"github.com/lichti/zaplab/internal/config"
	"github.com/lichti/zaplab/internal/webhook"
	"github.com/pocketbase/pocketbase"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var (
	client          *whatsmeow.Client
	storeContainer  *sqlstore.Container
	pb              *pocketbase.PocketBase
	wh              *webhook.Config
	cfg             *config.Config
	logger          waLog.Logger
	getIDSecret     string
	pairRejectChan  = make(chan bool, 1)
	historyPath     *string
	dbDialect       *string
	dbAddress       *string
	requestFullSync *bool
	logLevel        string
	deviceSpoof     *string
)

// Init injects all dependencies into the whatsapp package before Bootstrap is called.
func Init(pbApp *pocketbase.PocketBase, webhookCfg *webhook.Config, generalCfg *config.Config, log waLog.Logger,
	hp *string, dialect *string, addr *string, fullSync *bool, level string, spoof *string) {
	pb = pbApp
	wh = webhookCfg
	cfg = generalCfg
	logger = log
	historyPath = hp
	dbDialect = dialect
	dbAddress = addr
	requestFullSync = fullSync
	logLevel = level
	deviceSpoof = spoof
}

// TriggerDispatchFunc is set by package api during Init to dispatch script triggers
// without creating an import cycle (api imports whatsapp, not vice-versa).
var TriggerDispatchFunc func(evtType string, rawJSON []byte)

// GetClient returns the active whatsmeow client (nil if not yet bootstrapped).
func GetClient() *whatsmeow.Client { return client }

// GetDBAddress returns the whatsapp SQLite DSN (empty string if not yet set).
func GetDBAddress() string {
	if dbAddress == nil {
		return ""
	}
	return *dbAddress
}

// GetDBDialect returns the configured DB dialect ("sqlite" or "postgres").
func GetDBDialect() string {
	if dbDialect == nil {
		return ""
	}
	return *dbDialect
}

// DeviceKeyInfo holds the public key material of the registered device.
// Private keys are never exposed.
type DeviceKeyInfo struct {
	JID            string `json:"jid"`
	NoisePublicKey string `json:"noise_pub"`    // hex-encoded 32-byte Curve25519 key
	IdentityPubKey string `json:"identity_pub"` // hex-encoded 32-byte Ed25519 key
	RegistrationID uint32 `json:"registration_id"`
	AdvSecretKey   string `json:"adv_secret_key,omitempty"` // hex-encoded if present
	Platform       string `json:"platform"`
	BusinessName   string `json:"business_name"`
	PushName       string `json:"push_name"`
}

// GetDeviceKeys returns the public key info for the current device.
// Returns nil if the client is not yet bootstrapped.
func GetDeviceKeys() *DeviceKeyInfo {
	if client == nil || client.Store == nil {
		return nil
	}
	s := client.Store
	info := &DeviceKeyInfo{
		RegistrationID: s.RegistrationID,
		Platform:       s.Platform,
		BusinessName:   s.BusinessName,
		PushName:       s.PushName,
	}
	if s.ID != nil {
		info.JID = s.ID.String()
	}
	if s.NoiseKey != nil {
		info.NoisePublicKey = fmt.Sprintf("%x", s.NoiseKey.Pub)
	}
	if s.IdentityKey != nil {
		info.IdentityPubKey = fmt.Sprintf("%x", s.IdentityKey.Pub)
	}
	return info
}
