package whatsapp

import (
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
