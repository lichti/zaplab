package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/mdp/qrterminal/v3"
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

// ReinitializeForPairing tears down the current client, rebuilds it from the
// configured DSN (which will have no stored device after a logout), opens a
// fresh QR channel, and connects — allowing the user to scan a new QR code
// without restarting the server.
func ReinitializeForPairing() error {
	if client != nil {
		client.Disconnect()
		client = nil
	}
	if storeContainer != nil {
		_ = storeContainer.Close()
		storeContainer = nil
	}

	dbLog := NewCapturingLogger(waLog.Stdout("Database", logLevel, true), "Database")
	var err error
	storeContainer, err = sqlstore.New(context.Background(), *dbDialect, *dbAddress, dbLog)
	if err != nil {
		return fmt.Errorf("repaire: create store: %w", err)
	}
	device, err := storeContainer.GetFirstDevice(context.Background())
	if err != nil {
		return fmt.Errorf("repaire: get device: %w", err)
	}

	clientLog := NewCapturingLogger(waLog.Stdout("Client", logLevel, true), "Client")
	client = whatsmeow.NewClient(device, clientLog)
	client.AddEventHandler(handler)

	ch, err := client.GetQRChannel(context.Background())
	if err != nil {
		if !errors.Is(err, whatsmeow.ErrQRStoreContainsID) {
			return fmt.Errorf("repaire: get QR channel: %w", err)
		}
	} else {
		go func() {
			for evt := range ch {
				if evt.Event == "code" {
					logger.Infof("QRCode: %s", evt.Code)
					qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
					setConnStatus(StatusQR)
					setLastQR(evt.Code)
				} else {
					logger.Infof("QR channel result: %s", evt.Event)
					if evt.Event == "success" {
						setConnStatus(StatusConnected)
						setLastQR("")
					} else if evt.Event == "timeout" {
						setConnStatus(StatusTimeout)
						setLastQR("")
					}
				}
			}
		}()
	}

	setConnStatus(StatusConnecting)
	if err := client.Connect(); err != nil {
		return fmt.Errorf("repaire: connect: %w", err)
	}
	return nil
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
