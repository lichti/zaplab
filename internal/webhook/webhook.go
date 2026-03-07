package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// CmdWebhookConfig maps a command name to its webhook URL.
type CmdWebhookConfig struct {
	Cmd     string  `json:"cmd"`
	Webhook url.URL `json:"webhook"`
}

// Config holds the full webhook configuration.
type Config struct {
	CmdWebhooks    []CmdWebhookConfig `json:"webhook_config"`
	DefaultWebhook url.URL            `json:"default_webhook"`
	ErrorWebhook   url.URL            `json:"error_webhook"`
	ConfigFile     string             `json:"-"`
	log            waLog.Logger
}

// Load reads (or creates if absent) the config file at filepath.
func Load(filepath string, logger waLog.Logger) (*Config, error) {
	cfg := &Config{ConfigFile: filepath, log: logger}
	if _, err := os.Stat(cfg.ConfigFile); os.IsNotExist(err) {
		if err := cfg.writeConfig(); err != nil {
			logger.Errorf("Error to create webhook config file (%s): %+v", cfg.ConfigFile, err)
			cfg.ConfigFile = ""
			return nil, err
		}
	}
	if err := cfg.readConfig(); err != nil {
		logger.Errorf("Error to read webhook config file: %+v", err)
		return nil, err
	}
	cfg.log = logger // restore after unmarshal (unexported field is not touched by json)
	return cfg, nil
}

func (c *Config) readConfig() error {
	data, err := os.ReadFile(c.ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			c.CmdWebhooks = []CmdWebhookConfig{}
			c.DefaultWebhook = url.URL{}
			c.ErrorWebhook = url.URL{}
			return nil
		}
		c.log.Errorf("Error to read webhook config file: %+v", err)
		return err
	}
	if err := json.Unmarshal(data, c); err != nil {
		c.log.Errorf("Error to unmarshal webhook config: %+v", err)
		return err
	}
	return nil
}

func (c *Config) writeConfig() error {
	data, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		c.log.Errorf("Error to marshal webhook config: %+v", err)
		return err
	}
	if err := os.WriteFile(c.ConfigFile, data, 0644); err != nil {
		c.log.Errorf("Error to write webhook config file(%s): %+v", c.ConfigFile, err)
		return err
	}
	return nil
}

// CheckCmdExist returns true if cmd is already registered.
func (c *Config) CheckCmdExist(cmd string) bool {
	for _, v := range c.CmdWebhooks {
		if v.Cmd == cmd {
			return true
		}
	}
	return false
}

// AddCmdWebhook registers a new command → webhook URL mapping.
func (c *Config) AddCmdWebhook(cmd, strURL string) error {
	u, err := validateWebhookURL(strURL)
	if err != nil {
		c.log.Errorf("Invalid webhook URL %s: %v", strURL, err)
		return err
	}
	if c.CheckCmdExist(cmd) {
		c.log.Errorf("Webhook already exists for cmd: %s", cmd)
		return fmt.Errorf("webhook already exists for cmd: %s", cmd)
	}
	c.CmdWebhooks = append(c.CmdWebhooks, CmdWebhookConfig{Cmd: cmd, Webhook: *u})
	return c.writeConfig()
}

// RemoveCmdWebhook removes a command → webhook URL mapping.
func (c *Config) RemoveCmdWebhook(cmd string) error {
	for i, v := range c.CmdWebhooks {
		if v.Cmd == cmd {
			c.CmdWebhooks = append(c.CmdWebhooks[:i], c.CmdWebhooks[i+1:]...)
			return c.writeConfig()
		}
	}
	c.log.Errorf("Webhook not found for cmd: %s", cmd)
	return fmt.Errorf("webhook not found for cmd: %s", cmd)
}

func (c *Config) getCmdWebhook(cmd string) (*url.URL, error) {
	for _, v := range c.CmdWebhooks {
		if v.Cmd == cmd {
			return &v.Webhook, nil
		}
	}
	c.log.Errorf("Webhook not found for cmd: %s", cmd)
	return nil, fmt.Errorf("webhook not found for cmd: %s", cmd)
}

// PrintConfig returns a formatted summary of the current configuration.
func (c *Config) PrintConfig() string {
	text := "\n\n ================= Webhook config ================= \n"
	text = fmt.Sprintf("%s - Default webhook:\t%s\n", text, c.GetDefaultWebhook().String())
	text = fmt.Sprintf("%s - Error webhook:\t%s\n", text, c.GetErrorWebhook().String())
	for _, v := range c.CmdWebhooks {
		text = fmt.Sprintf("%s - cmd: %s\t\t%s\n", text, v.Cmd, v.Webhook.String())
	}
	text = fmt.Sprintf("%s ================================================== \n\n", text)
	return text
}

// validateWebhookURL parses and validates that a URL is a reachable http/https endpoint.
func validateWebhookURL(strURL string) (*url.URL, error) {
	u, err := validateURL(strURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid URL scheme: %s", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid URL: missing host")
	}
	return u, nil
}

// SetDefaultWebhook validates and persists a new default webhook URL.
func (c *Config) SetDefaultWebhook(strURL string) error {
	u, err := validateWebhookURL(strURL)
	if err != nil {
		c.log.Errorf("Invalid webhook URL %s: %v", strURL, err)
		return err
	}
	c.DefaultWebhook = *u
	return c.writeConfig()
}

// GetDefaultWebhook returns the default webhook URL (never nil).
func (c *Config) GetDefaultWebhook() *url.URL {
	if c.DefaultWebhook.String() == "" {
		return &url.URL{}
	}
	return &c.DefaultWebhook
}

// HasDefaultWebhook reports whether a default webhook is configured.
func (c *Config) HasDefaultWebhook() bool {
	return c.DefaultWebhook.String() != ""
}

// SetErrorWebhook validates and persists a new error webhook URL.
func (c *Config) SetErrorWebhook(strURL string) error {
	u, err := validateWebhookURL(strURL)
	if err != nil {
		c.log.Errorf("Invalid webhook URL %s: %v", strURL, err)
		return err
	}
	c.ErrorWebhook = *u
	return c.writeConfig()
}

// GetErrorWebhook returns the error webhook URL (never nil).
func (c *Config) GetErrorWebhook() *url.URL {
	if c.ErrorWebhook.String() == "" {
		return &url.URL{}
	}
	return &c.ErrorWebhook
}

// HasErrorWebhook reports whether an error webhook is configured.
func (c *Config) HasErrorWebhook() bool {
	return c.ErrorWebhook.String() != ""
}

// SendToDefault sends an event payload to the default webhook.
func (c *Config) SendToDefault(evtType string, raw, extra interface{}) error {
	if !c.HasDefaultWebhook() {
		return fmt.Errorf("default webhook not configured")
	}
	return c.send(c.GetDefaultWebhook(), evtType, raw, extra)
}

// SendToError sends an event payload to the error webhook.
func (c *Config) SendToError(evtType string, raw, extra interface{}) error {
	if !c.HasErrorWebhook() {
		return fmt.Errorf("error webhook not configured")
	}
	return c.send(c.GetErrorWebhook(), evtType, raw, extra)
}

// SendToCmd sends an event payload to the webhook registered for cmd.
func (c *Config) SendToCmd(cmd, evtType string, raw, extra interface{}) error {
	u, err := c.getCmdWebhook(cmd)
	if err != nil {
		return err
	}
	return c.send(u, evtType, raw, extra)
}

type eventPayload struct {
	Type  string      `json:"type"`
	Raw   interface{} `json:"raw"`
	Extra interface{} `json:"extra,omitempty"`
}

func (c *Config) send(webhookURL *url.URL, evtType string, raw, extra interface{}) error {
	if webhookURL.String() == "" {
		return fmt.Errorf("webhookURL is empty")
	}
	body, err := json.Marshal([]eventPayload{{Type: evtType, Raw: raw, Extra: extra}})
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", webhookURL.String(), bytes.NewBuffer(body))
	if err != nil {
		c.log.Debugf("Error creating HTTP request: %+v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		c.log.Debugf("Error sending HTTP request: %+v", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code from webhook: %d", resp.StatusCode)
	}
	return nil
}

func validateURL(raw string) (*url.URL, error) {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %s err=%+v", raw, err)
	}
	return u, nil
}
