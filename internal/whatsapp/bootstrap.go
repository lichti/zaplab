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
	"go.mau.fi/whatsmeow/proto/waWa6"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// applyDeviceSpoof configures store.DeviceProps and store.BaseClientPayload to present the
// client as the specified device type to the WhatsApp server.
//
// WARNING: Work in Progress — not fully functional. These changes affect the identity payload
// sent during the WebSocket handshake but have not been confirmed to unlock restricted
// features (e.g. live location). Connection failures, session termination, or account bans
// are possible. For full effect, re-pair the device after changing this setting.
// Already-paired companion devices retain their companion Device ID (>0) in the stored JID
// regardless of these fields.
//
// Modes:
//   - "companion" (default) — standard whatsmeow companion/linked device (ANDROID_PHONE type, WEB platform)
//   - "android"             — impersonate a native Android phone (ANDROID platform, no WebInfo)
//   - "ios"                 — impersonate a native iPhone (IOS platform, no WebInfo)
func applyDeviceSpoof(mode string) {
	switch mode {
	case "android":
		logger.Infof("Device spoof: android — presenting as native Android phone")

		// Companion registration props: declare as Android phone
		store.DeviceProps.Os = proto.String("Android")
		store.DeviceProps.PlatformType = waCompanionReg.DeviceProps_ANDROID_PHONE.Enum()
		store.DeviceProps.Version.Primary = proto.Uint32(2)
		store.DeviceProps.Version.Secondary = proto.Uint32(24)
		store.DeviceProps.Version.Tertiary = proto.Uint32(78)

		// UserAgent: present as native Android app, not a browser
		store.BaseClientPayload.UserAgent.Platform = waWa6.ClientPayload_UserAgent_ANDROID.Enum()
		store.BaseClientPayload.UserAgent.DeviceType = waWa6.ClientPayload_UserAgent_PHONE.Enum()
		store.BaseClientPayload.UserAgent.Manufacturer = proto.String("Samsung")
		store.BaseClientPayload.UserAgent.Device = proto.String("Galaxy S23")
		store.BaseClientPayload.UserAgent.OsVersion = proto.String("14.0.0")
		store.BaseClientPayload.UserAgent.OsBuildNumber = proto.String("UP1A.231005.007")
		store.BaseClientPayload.UserAgent.LocaleLanguageIso6391 = proto.String("en")
		store.BaseClientPayload.UserAgent.LocaleCountryIso31661Alpha2 = proto.String("US")

		// Native apps do not send WebInfo — remove it
		store.BaseClientPayload.WebInfo = nil

	case "ios":
		logger.Infof("Device spoof: ios — presenting as native iPhone")

		// Companion registration props: declare as iOS phone
		store.DeviceProps.Os = proto.String("iOS")
		store.DeviceProps.PlatformType = waCompanionReg.DeviceProps_IOS_PHONE.Enum()
		store.DeviceProps.Version.Primary = proto.Uint32(24)
		store.DeviceProps.Version.Secondary = proto.Uint32(3)
		store.DeviceProps.Version.Tertiary = proto.Uint32(81)

		// UserAgent: present as native iOS app
		store.BaseClientPayload.UserAgent.Platform = waWa6.ClientPayload_UserAgent_IOS.Enum()
		store.BaseClientPayload.UserAgent.DeviceType = waWa6.ClientPayload_UserAgent_PHONE.Enum()
		store.BaseClientPayload.UserAgent.Manufacturer = proto.String("Apple")
		store.BaseClientPayload.UserAgent.Device = proto.String("iPhone15,3")
		store.BaseClientPayload.UserAgent.OsVersion = proto.String("17.4.1")
		store.BaseClientPayload.UserAgent.OsBuildNumber = proto.String("21E236")
		store.BaseClientPayload.UserAgent.LocaleLanguageIso6391 = proto.String("en")
		store.BaseClientPayload.UserAgent.LocaleCountryIso31661Alpha2 = proto.String("US")

		// Native apps do not send WebInfo — remove it
		store.BaseClientPayload.WebInfo = nil

	default: // "companion"
		logger.Infof("Device spoof: companion (default) — standard linked/companion device")

		store.DeviceProps.Os = proto.String("WhatsApp Android")
		store.DeviceProps.PlatformType = waCompanionReg.DeviceProps_ANDROID_PHONE.Enum()
		store.DeviceProps.Version.Primary = proto.Uint32(2)
		store.DeviceProps.Version.Secondary = proto.Uint32(24)
		store.DeviceProps.Version.Tertiary = proto.Uint32(78)
		// Keep default WEB platform and WebInfo — standard companion behavior
	}
}

// Bootstrap connects to WhatsApp. Call after Init.
func Bootstrap(e *core.BootstrapEvent) error {
	waBinary.IndentXML = true

	getIDSecret = strings.ToUpper(hex.EncodeToString(random.Bytes(8)))

	spoof := "companion"
	if deviceSpoof != nil && *deviceSpoof != "" {
		spoof = *deviceSpoof
	}
	applyDeviceSpoof(spoof)

	if *requestFullSync {
		store.DeviceProps.RequireFullSync = proto.Bool(true)
		store.DeviceProps.HistorySyncConfig = &waCompanionReg.DeviceProps_HistorySyncConfig{
			FullSyncDaysLimit:   proto.Uint32(3650),
			FullSyncSizeMbLimit: proto.Uint32(102400),
			StorageQuotaMb:      proto.Uint32(102400),
		}
	}

	dbLog := NewCapturingLogger(waLog.Stdout("Database", logLevel, true), "Database")
	var err error
	storeContainer, err = sqlstore.New(context.Background(), *dbDialect, *dbAddress, dbLog)
	if err != nil {
		logger.Errorf("Failed to connect to database: %v", err)
		return fmt.Errorf("failed to connect to database: %v", err)
	}
	device, err := storeContainer.GetFirstDevice(context.Background())
	if err != nil {
		logger.Errorf("Failed to get device: %v", err)
		return fmt.Errorf("failed to get device: %v", err)
	}

	clientLog := NewCapturingLogger(waLog.Stdout("Client", logLevel, true), "Client")
	client = whatsmeow.NewClient(device, clientLog)

	// Apply persisted delivery receipt suppression setting.
	if cfg != nil && cfg.IsSuppressDeliveryReceipts() {
		client.SetSuppressDeliveryReceipts(true)
		logger.Infof("Bootstrap: delivery receipt suppression enabled (from config)")
	}

	// Start log consumer after pb is available (set via Init before Bootstrap).
	StartLogConsumer()

	// Force Device=0 in the login payload to impersonate a primary device.
	// The GetClientPayload hook runs at every handshake and overrides the device
	// number that would normally come from the stored JID (Device > 0 for companions).
	//
	// WARNING: The stored JID still has Device > 0 from the original pairing.
	// The server may accept the session, reject it, or downgrade it silently.
	// This is experimental — connection failures or session termination are possible.
	if spoof == "android" || spoof == "ios" {
		logger.Warnf("Device spoof %q: overriding ClientPayload.Device=0 to impersonate primary device — experimental", spoof)
		client.GetClientPayload = func() *waWa6.ClientPayload {
			payload := client.Store.GetClientPayload()
			payload.Device = proto.Uint32(0)
			return payload
		}
	}

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

	startEventWorker()
	startReceiptLatencyWorker()
	client.AddEventHandler(handler)
	setConnStatus(StatusConnecting)
	if err := client.Connect(); err != nil {
		logger.Errorf("Failed to connect: %v", err)
		return fmt.Errorf("failed to connect: %v", err)
	}

	return nil
}
