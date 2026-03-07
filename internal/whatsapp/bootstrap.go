package whatsapp

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mdp/qrterminal/v3"
	"github.com/pocketbase/pocketbase/core"
	"go.mau.fi/util/random"
	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// Bootstrap connects to WhatsApp. Call after Init.
func Bootstrap(e *core.BootstrapEvent) error {
	waBinary.IndentXML = true

	getIDSecret = strings.ToUpper(hex.EncodeToString(random.Bytes(8)))
	store.DeviceProps.Os = proto.String("WhatsApp Android")
	store.DeviceProps.Version.Primary = proto.Uint32(2)
	store.DeviceProps.Version.Secondary = proto.Uint32(24)
	store.DeviceProps.Version.Tertiary = proto.Uint32(23)
	store.DeviceProps.Version.Tertiary = proto.Uint32(78)
	store.DeviceProps.PlatformType = waCompanionReg.DeviceProps_ANDROID_PHONE.Enum()

	if *requestFullSync {
		store.DeviceProps.RequireFullSync = proto.Bool(true)
		store.DeviceProps.HistorySyncConfig = &waCompanionReg.DeviceProps_HistorySyncConfig{
			FullSyncDaysLimit:   proto.Uint32(3650),
			FullSyncSizeMbLimit: proto.Uint32(102400),
			StorageQuotaMb:      proto.Uint32(102400),
		}
	}

	dbLog := waLog.Stdout("Database", logLevel, true)
	storeContainer, err := sqlstore.New(context.Background(), *dbDialect, *dbAddress, dbLog)
	if err != nil {
		logger.Errorf("Failed to connect to database: %v", err)
		return fmt.Errorf("failed to connect to database: %v", err)
	}
	device, err := storeContainer.GetFirstDevice(context.Background())
	if err != nil {
		logger.Errorf("Failed to get device: %v", err)
		return fmt.Errorf("failed to get device: %v", err)
	}

	client = whatsmeow.NewClient(device, waLog.Stdout("Client", logLevel, true))
	var isWaitingForPair atomic.Bool
	client.PrePairCallback = func(jid types.JID, platform, businessName string) bool {
		isWaitingForPair.Store(true)
		defer isWaitingForPair.Store(false)
		logger.Infof("Pairing %s (platform: %q, business name: %q). Type r within 3 seconds to reject pair", jid, platform, businessName)
		select {
		case reject := <-pairRejectChan:
			if reject {
				logger.Infof("Rejecting pair")
				return false
			}
		case <-time.After(3 * time.Second):
		}
		logger.Infof("Accepting pair")
		return true
	}

	ch, err := client.GetQRChannel(context.Background())
	if err != nil {
		if !errors.Is(err, whatsmeow.ErrQRStoreContainsID) {
			logger.Errorf("Failed to get QR channel: %v", err)
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

	client.AddEventHandler(handler)
	setConnStatus(StatusConnecting)
	if err := client.Connect(); err != nil {
		logger.Errorf("Failed to connect: %v", err)
		return fmt.Errorf("failed to connect: %v", err)
	}

	return nil
}
