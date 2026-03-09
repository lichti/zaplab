package whatsapp

import (
	"github.com/lichti/zaplab/internal/webhook"
	"github.com/pocketbase/pocketbase"
	"go.mau.fi/whatsmeow"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var (
	client          *whatsmeow.Client
	pb              *pocketbase.PocketBase
	wh              *webhook.Config
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
func Init(pbApp *pocketbase.PocketBase, webhookCfg *webhook.Config, log waLog.Logger,
	hp *string, dialect *string, addr *string, fullSync *bool, level string, spoof *string) {
	pb = pbApp
	wh = webhookCfg
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
