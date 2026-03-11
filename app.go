package main

import (
	"github.com/pocketbase/pocketbase"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// App holds shared application state for the main package.
type App struct {
	pb       *pocketbase.PocketBase
	log      waLog.Logger
	logLevel string

	// CLI flags (pointers bound by cobra/pflag)
	debugLogs         *bool
	dataDir           *string
	dbDialect         *string
	dbAddress         *string
	requestFullSync   *bool
	historyPath       *string
	webhookConfigFile *string
	generalConfigFile *string
	deviceSpoof       *string
}

var app *App
