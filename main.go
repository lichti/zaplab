package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"crypto/rand"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/spf13/cobra"
	waLog "go.mau.fi/whatsmeow/util/log"
	"math/big"
	_ "modernc.org/sqlite" // register "sqlite" driver for whatsmeow sqlstore

	"github.com/lichti/zaplab/internal/api"
	"github.com/lichti/zaplab/internal/config"
	"github.com/lichti/zaplab/internal/webhook"
	"github.com/lichti/zaplab/internal/whatsapp"
	_ "github.com/lichti/zaplab/migrations"
)

// Version is set at build time via -ldflags "-X main.Version=x.y.z".
var Version = "dev"

// defaultDataDir returns the base data directory from the environment variable
// ZAPLAB_DATA_DIR, falling back to $HOME/.zaplab.
func defaultDataDir() string {
	if v := os.Getenv("ZAPLAB_DATA_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".zaplab"
	}
	return filepath.Join(home, ".zaplab")
}

// preScanDataDir reads --data-dir from os.Args before cobra parses flags.
// This is needed so that pocketbase.NewWithConfig receives the correct
// DefaultDataDir (for its own --dir flag) before flag parsing begins.
func preScanDataDir() string {
	base := defaultDataDir()
	args := os.Args[1:]
	for i, arg := range args {
		if arg == "--data-dir" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, "--data-dir=") {
			return strings.TrimPrefix(arg, "--data-dir=")
		}
	}
	return base
}

// setupLogFile tees os.Stdout to a log file at <baseDir>/logs/app.log.
// Must be called before any logger is created so that all output is captured.
func setupLogFile(baseDir string) {
	logsDir := filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return
	}
	logFile, err := os.OpenFile(filepath.Join(logsDir, "app.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	r, w, err := os.Pipe()
	if err != nil {
		logFile.Close()
		return
	}
	origStdout := os.Stdout
	os.Stdout = w
	go func() {
		io.Copy(io.MultiWriter(origStdout, logFile), r)
		logFile.Close()
	}()
}

// generateRandomPassword creates a secure random string for initial login.
func generateRandomPassword(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	res := make([]byte, length)
	for i := range res {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "zaplab-p@ssw0rd!" // fallback
		}
		res[i] = chars[num.Int64()]
	}
	return string(res)
}

func main() {
	baseDir := preScanDataDir()
	setupLogFile(baseDir)

	app = &App{
		logLevel:          "INFO",
		debugLogs:         new(bool),
		dataDir:           new(string),
		dbDialect:         new(string),
		dbAddress:         new(string),
		requestFullSync:   new(bool),
		historyPath:       new(string),
		webhookConfigFile: new(string),
		generalConfigFile: new(string),
		deviceSpoof:       new(string),
	}

	// Use pre-scanned base dir as the PocketBase default data dir.
	// The user can still override with --dir (PocketBase's own flag).
	app.pb = pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir: filepath.Join(baseDir, "pb_data"),
	})

	// --data-dir sets the base for all derived paths.
	// Default is resolved from ZAPLAB_DATA_DIR env var or $HOME/.zaplab.
	app.pb.RootCmd.PersistentFlags().StringVar(app.dataDir, "data-dir", baseDir,
		"Base directory for all runtime data (env: ZAPLAB_DATA_DIR)")

	app.pb.RootCmd.PersistentFlags().BoolVar(app.debugLogs, "debug", false,
		"Enable debug logs")
	app.pb.RootCmd.PersistentFlags().StringVar(app.dbDialect, "whatsapp-db-dialect", "sqlite",
		"WhatsApp database dialect (sqlite or postgres)")

	// Sub-paths default to empty string; resolved from --data-dir in the bootstrap hook.
	app.pb.RootCmd.PersistentFlags().StringVar(app.dbAddress, "whatsapp-db-address", "",
		"WhatsApp database DSN (default: file:<data-dir>/db/whatsapp.db?_pragma=foreign_keys(1))")
	app.pb.RootCmd.PersistentFlags().BoolVar(app.requestFullSync, "whatsapp-request-full-sync", false,
		"Request full history sync (10 years) on first login")
	app.pb.RootCmd.PersistentFlags().StringVar(app.historyPath, "whatsapp-history-path", "",
		"Directory for HistorySync JSON dumps (default: <data-dir>/history)")
	app.pb.RootCmd.PersistentFlags().StringVar(app.webhookConfigFile, "webhook-config-file", "",
		"Webhook configuration file path (default: <data-dir>/webhook.json)")
	app.pb.RootCmd.PersistentFlags().StringVar(app.generalConfigFile, "config-file", "",
		"General configuration file path (default: <data-dir>/config.json)")
	app.pb.RootCmd.PersistentFlags().StringVar(app.deviceSpoof, "device-spoof", "companion",
		"Device identity presented to WhatsApp servers: companion (default), android, ios. "+
			"Experimental — re-pair after changing. See docs for details.")

	// app.log starts at INFO; upgraded to DEBUG inside the bootstrap hook once flags are parsed.
	app.log = waLog.Stdout("Main", app.logLevel, true)
	app.log.Infof("zaplab %s", Version)

	// OnBootstrap wraps the core bootstrap: resolve paths, init packages, then connect WhatsApp.
	app.pb.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if *app.debugLogs {
			app.logLevel = "DEBUG"
			app.log = waLog.Stdout("Main", app.logLevel, true)
		}

		// Resolve derived paths from --data-dir when not explicitly set.
		base := *app.dataDir
		if *app.dbAddress == "" {
			*app.dbAddress = "file:" + filepath.Join(base, "db", "whatsapp.db") + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
		}
		if *app.historyPath == "" {
			*app.historyPath = filepath.Join(base, "history")
		}
		if *app.webhookConfigFile == "" {
			*app.webhookConfigFile = filepath.Join(base, "webhook.json")
		}
		if *app.generalConfigFile == "" {
			*app.generalConfigFile = filepath.Join(base, "config.json")
		}

		// Ensure directories exist before use.
		for _, dir := range []string{
			filepath.Join(base, "db"),
			*app.historyPath,
		} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("failed to create data dir %s: %w", dir, err)
			}
		}

		wh, err := webhook.Load(*app.webhookConfigFile, app.log)
		if err != nil {
			return fmt.Errorf("error loading webhook config: %w", err)
		}
		fmt.Print(wh.PrintConfig())

		cfg, err := config.Load(*app.generalConfigFile, app.log)
		if err != nil {
			return fmt.Errorf("error loading general config: %w", err)
		}

		whatsapp.Init(app.pb, wh, cfg, app.log, app.historyPath, app.dbDialect, app.dbAddress, app.requestFullSync, app.logLevel, app.deviceSpoof)
		api.Init(app.pb, wh, cfg, getStaticFS())

		// Let core bootstrap run (DB init, migrations, cache reload, etc.).
		if err := e.Next(); err != nil {
			return err
		}

		// Auto-create the first user if the collection is empty.
		users, err := app.pb.FindCollectionByNameOrId("users")
		if err == nil {
			records, err := app.pb.FindRecordsByFilter("users", "id != ''", "", 1, 0)
			if err == nil && len(records) == 0 {
				pass := generateRandomPassword(16)
				rec := core.NewRecord(users)
				rec.Set("email", "zaplab@zaplab.local")
				rec.Set("password", pass)
				rec.Set("verified", true)
				rec.Set("force_password_change", true)
				if err := app.pb.SaveNoValidate(rec); err == nil {
					fmt.Printf("\n==================================================\n")
					fmt.Printf("FIRST RUN: Created default user zaplab@zaplab.local\n")
					fmt.Printf("Initial Password: %s\n", pass)
					fmt.Printf("Please change it after logging in.\n")
					fmt.Printf("==================================================\n\n")
				}
			}
		}

		// Connect to WhatsApp after bootstrap completes.
		return whatsapp.Bootstrap(e)
	})

	// Signal the whatsapp package to stop persisting events before the DB closes.
	app.pb.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		whatsapp.SetShuttingDown()
		return e.Next()
	})

	// Register HTTP API routes on serve.
	app.pb.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := api.RegisterRoutes(e); err != nil {
			return err
		}
		return e.Next()
	})

	app.pb.RootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the zaplab version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(Version)
		},
	})

	userCmd := &cobra.Command{
		Use:   "user",
		Short: "Manage dashboard users",
	}

	createCmd := &cobra.Command{
		Use:   "create <email> <password>",
		Short: "Create a new dashboard user",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			email := args[0]
			password := args[1]

			if err := app.pb.Bootstrap(); err != nil {
				app.log.Errorf("Failed to bootstrap app: %v", err)
				os.Exit(1)
			}

			collection, err := app.pb.FindCollectionByNameOrId("users")
			if err != nil {
				app.log.Errorf("Failed to find users collection: %v", err)
				os.Exit(1)
			}

			record := core.NewRecord(collection)
			record.Set("email", email)
			record.Set("password", password)
			record.Set("verified", true)
			record.Set("force_password_change", true)

			if err := app.pb.SaveNoValidate(record); err != nil {
				app.log.Errorf("Failed to save user: %v", err)
				os.Exit(1)
			}

			fmt.Printf("User %s created successfully.\n", email)
		},
	}

	resetPwdCmd := &cobra.Command{
		Use:   "reset-password <email> <new-password>",
		Short: "Reset a user's password",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			email := args[0]
			newPassword := args[1]

			if err := app.pb.Bootstrap(); err != nil {
				app.log.Errorf("Failed to bootstrap app: %v", err)
				os.Exit(1)
			}

			user, err := app.pb.FindAuthRecordByEmail("users", email)
			if err != nil {
				app.log.Errorf("User not found: %v", err)
				os.Exit(1)
			}

			user.Set("password", newPassword)
			user.Set("force_password_change", true)
			if err := app.pb.SaveNoValidate(user); err != nil {
				app.log.Errorf("Failed to update password: %v", err)
				os.Exit(1)
			}

			fmt.Printf("Password for %s reset successfully.\n", email)
		},
	}

	userCmd.AddCommand(createCmd)
	userCmd.AddCommand(resetPwdCmd)
	app.pb.RootCmd.AddCommand(userCmd)

	migratecmd.MustRegister(app.pb, app.pb.RootCmd, migratecmd.Config{
		Automigrate: true,
	})

	if err := app.pb.Start(); err != nil {
		app.log.Errorf("unexpected status code from pocketbase: %+v", err)
		os.Exit(100)
	}
}
