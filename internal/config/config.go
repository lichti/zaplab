package config

import (
	"encoding/json"
	"os"
	"sync"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// Config holds general application settings.
type Config struct {
	RecoverEdits             bool   `json:"recover_edits"`
	RecoverDeletes           bool   `json:"recover_deletes"`
	ActivityTrackerEnabled   bool   `json:"activity_tracker_enabled"`
	SuppressDeliveryReceipts bool   `json:"suppress_delivery_receipts"`
	AppearOffline            bool   `json:"appear_offline"`
	configFile               string `json:"-"`
	log                      waLog.Logger
	mu                       sync.RWMutex
}

// Load reads (or creates if absent) the config file at filepath.
func Load(filepath string, logger waLog.Logger) (*Config, error) {
	cfg := &Config{configFile: filepath, log: logger}
	if _, err := os.Stat(cfg.configFile); os.IsNotExist(err) {
		if err := cfg.save(); err != nil {
			logger.Errorf("Error creating config file (%s): %v", cfg.configFile, err)
			return nil, err
		}
	}
	if err := cfg.load(); err != nil {
		logger.Errorf("Error reading config file: %v", err)
		return nil, err
	}
	cfg.log = logger
	return cfg, nil
}

func (c *Config) load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.configFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, c)
}

func (c *Config) save() error {
	data, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.configFile, data, 0644)
}

// SetRecoverEdits updates the edit recovery setting and persists it.
func (c *Config) SetRecoverEdits(enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.RecoverEdits = enabled
	return c.save()
}

// SetRecoverDeletes updates the delete recovery setting and persists it.
func (c *Config) SetRecoverDeletes(enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.RecoverDeletes = enabled
	return c.save()
}

// IsRecoverEditsEnabled returns the current state of edit recovery.
func (c *Config) IsRecoverEditsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.RecoverEdits
}

// IsRecoverDeletesEnabled returns the current state of delete recovery.
func (c *Config) IsRecoverDeletesEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.RecoverDeletes
}

// SetActivityTrackerEnabled updates the activity tracker feature flag and persists it.
func (c *Config) SetActivityTrackerEnabled(enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ActivityTrackerEnabled = enabled
	return c.save()
}

// IsActivityTrackerEnabled returns whether the device activity tracker is enabled.
func (c *Config) IsActivityTrackerEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ActivityTrackerEnabled
}

// SetSuppressDeliveryReceipts controls whether delivery (grey-tick) receipts are sent.
func (c *Config) SetSuppressDeliveryReceipts(enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SuppressDeliveryReceipts = enabled
	return c.save()
}

// IsSuppressDeliveryReceipts returns whether delivery receipt suppression is enabled.
func (c *Config) IsSuppressDeliveryReceipts() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.SuppressDeliveryReceipts
}

// SetAppearOffline controls whether the client advertises itself as offline to contacts.
func (c *Config) SetAppearOffline(offline bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.AppearOffline = offline
	return c.save()
}

// IsAppearOffline returns whether the client is configured to appear offline.
func (c *Config) IsAppearOffline() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.AppearOffline
}
