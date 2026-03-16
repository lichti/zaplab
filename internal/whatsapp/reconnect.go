package whatsapp

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// ForceReconnect disconnects the active WhatsApp client and immediately reconnects.
// Useful after editing individual DB rows to observe protocol behaviour changes.
func ForceReconnect() error {
	if client == nil {
		return fmt.Errorf("no active client")
	}
	client.Disconnect()
	return client.Connect()
}

// Reinitialize fully tears down the current whatsmeow client and store, then
// rebuilds from the configured DSN. Use after replacing the database file
// (e.g. restoring a backup) so that all in-memory state is discarded.
func Reinitialize() error {
	if client != nil {
		client.Disconnect()
		client = nil
	}
	if storeContainer != nil {
		_ = storeContainer.Close()
		storeContainer = nil
	}

	dbLog := waLog.Stdout("Database", logLevel, true)
	var err error
	storeContainer, err = sqlstore.New(context.Background(), *dbDialect, *dbAddress, dbLog)
	if err != nil {
		return fmt.Errorf("reinitialize: create store: %w", err)
	}
	device, err := storeContainer.GetFirstDevice(context.Background())
	if err != nil {
		return fmt.Errorf("reinitialize: get device: %w", err)
	}

	client = whatsmeow.NewClient(device, waLog.Stdout("Client", logLevel, true))
	client.AddEventHandler(handler)
	setConnStatus(StatusConnecting)
	if err := client.Connect(); err != nil {
		return fmt.Errorf("reinitialize: connect: %w", err)
	}
	return nil
}
