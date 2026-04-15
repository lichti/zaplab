package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// CmdWebhookConfig maps a command name to its webhook URL.
type CmdWebhookConfig struct {
	Cmd     string  `json:"cmd"`
	Webhook url.URL `json:"webhook"`
}

// TextWebhook fires when the text content of an incoming Message matches a pattern.
//
//	MatchType: "prefix"   → message starts with Pattern
//	           "contains" → message contains Pattern anywhere
//	           "exact"    → message equals Pattern exactly
//	From:      "all"      → match regardless of sender
//	           "me"       → match only when IsFromMe == true
//	           "others"   → match only when IsFromMe == false
//	CaseSensitive: when false (default) comparison is done in lowercase.
type TextWebhook struct {
	ID            string  `json:"id"`
	MatchType     string  `json:"match_type"`
	Pattern       string  `json:"pattern"`
	From          string  `json:"from"`
	CaseSensitive bool    `json:"case_sensitive"`
	Webhook       url.URL `json:"webhook"`
}

// TextWebhookAPI is the API-friendly representation with URL as a plain string.
type TextWebhookAPI struct {
	ID            string `json:"id"`
	MatchType     string `json:"match_type"`
	Pattern       string `json:"pattern"`
	From          string `json:"from"`
	CaseSensitive bool   `json:"case_sensitive"`
	URL           string `json:"url"`
}

// EventTypeWebhook maps an event type pattern to a webhook URL.
// Pattern may be an exact type ("Message.ImageMessage") or a wildcard
// prefix ending in ".*" (e.g. "Message.*" matches all Message sub-types).
type EventTypeWebhook struct {
	EventType string  `json:"event_type"`
	Webhook   url.URL `json:"webhook"`
}

// EventTypeWebhookAPI is the API-friendly representation with URL as a plain string.
type EventTypeWebhookAPI struct {
	EventType string `json:"event_type"`
	URL       string `json:"url"`
}

// DeliveryRecord carries the result of a single delivery attempt for the caller to log.
type DeliveryRecord struct {
	WebhookURL  string
	EventType   string
	Status      string // "delivered" | "failed"
	Attempt     int
	HTTPStatus  int
	ErrorMsg    string
	DeliveredAt string
}

// Config holds the full webhook configuration.
type Config struct {
	CmdWebhooks    []CmdWebhookConfig `json:"webhook_config"`
	EventWebhooks  []EventTypeWebhook `json:"event_webhooks"`
	TextWebhooks   []TextWebhook      `json:"text_webhooks"`
	DefaultWebhook url.URL            `json:"default_webhook"`
	ErrorWebhook   url.URL            `json:"error_webhook"`
	ConfigFile     string             `json:"-"`
	// Secret is loaded from the WEBHOOK_SECRET env var at Load time.
	// When non-empty, every outgoing request includes an X-ZapLab-Signature header.
	Secret string `json:"-"`
	// OnDelivery is called after each delivery attempt (both success and failure).
	// Set by the caller to persist delivery records. May be nil.
	OnDelivery func(DeliveryRecord)
	log        waLog.Logger
}

// Load reads (or creates if absent) the config file at filepath.
func Load(filepath string, logger waLog.Logger) (*Config, error) {
	cfg := &Config{ConfigFile: filepath, log: logger, Secret: os.Getenv("WEBHOOK_SECRET")}
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

// generateID returns a short random hex string used as TextWebhook ID.
func generateID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// matchesText reports whether text matches pattern according to matchType and caseSensitive.
func matchesText(pattern, matchType, text string, caseSensitive bool) bool {
	if !caseSensitive {
		pattern = strings.ToLower(pattern)
		text = strings.ToLower(text)
	}
	switch matchType {
	case "prefix":
		return strings.HasPrefix(text, pattern)
	case "exact":
		return text == pattern
	case "contains":
		return strings.Contains(text, pattern)
	}
	return false
}

// matchesFrom reports whether isFromMe satisfies the from filter.
func matchesFrom(from string, isFromMe bool) bool {
	switch from {
	case "me":
		return isFromMe
	case "others":
		return !isFromMe
	default: // "all"
		return true
	}
}

// AddTextWebhook registers a new text-pattern webhook.
// matchType: "prefix" | "contains" | "exact"
// from:      "all" | "me" | "others"
func (c *Config) AddTextWebhook(matchType, pattern, from string, caseSensitive bool, strURL string) error {
	if matchType != "prefix" && matchType != "contains" && matchType != "exact" {
		return fmt.Errorf("invalid match_type %q — use: prefix, contains, exact", matchType)
	}
	if from != "all" && from != "me" && from != "others" {
		return fmt.Errorf("invalid from %q — use: all, me, others", from)
	}
	if pattern == "" {
		return fmt.Errorf("pattern is required")
	}
	u, err := validateWebhookURL(strURL)
	if err != nil {
		c.log.Errorf("Invalid webhook URL %s: %v", strURL, err)
		return err
	}
	c.TextWebhooks = append(c.TextWebhooks, TextWebhook{
		ID:            generateID(),
		MatchType:     matchType,
		Pattern:       pattern,
		From:          from,
		CaseSensitive: caseSensitive,
		Webhook:       *u,
	})
	return c.writeConfig()
}

// RemoveTextWebhook removes the text webhook with the given id.
func (c *Config) RemoveTextWebhook(id string) error {
	for i, tw := range c.TextWebhooks {
		if tw.ID == id {
			c.TextWebhooks = append(c.TextWebhooks[:i], c.TextWebhooks[i+1:]...)
			return c.writeConfig()
		}
	}
	return fmt.Errorf("text webhook not found: %s", id)
}

// GetTextWebhooks returns a copy of the current text webhooks as API-friendly structs.
func (c *Config) GetTextWebhooks() []TextWebhookAPI {
	result := make([]TextWebhookAPI, len(c.TextWebhooks))
	for i, tw := range c.TextWebhooks {
		result[i] = TextWebhookAPI{
			ID:            tw.ID,
			MatchType:     tw.MatchType,
			Pattern:       tw.Pattern,
			From:          tw.From,
			CaseSensitive: tw.CaseSensitive,
			URL:           tw.Webhook.String(),
		}
	}
	return result
}

// SendToTextWebhooks fires all text webhooks whose pattern and from-filter match.
// text is the extracted message content; isFromMe indicates if the bot sent it.
func (c *Config) SendToTextWebhooks(text string, isFromMe bool, raw, extra interface{}) {
	if text == "" {
		return
	}
	for _, tw := range c.TextWebhooks {
		if matchesFrom(tw.From, isFromMe) && matchesText(tw.Pattern, tw.MatchType, text, tw.CaseSensitive) {
			u := tw.Webhook
			go func() {
				if err := c.send(&u, "Message.Text", raw, extra); err != nil {
					c.log.Warnf("Failed to send to text webhook id=%s url=%s error=%v",
						tw.ID, u.String(), err)
				}
			}()
		}
	}
}

// matchesEventType reports whether pattern matches evtType.
// "Message.*" matches "Message" and any "Message.X" subtype.
func matchesEventType(pattern, evtType string) bool {
	if pattern == evtType {
		return true
	}
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return evtType == prefix || strings.HasPrefix(evtType, prefix+".")
	}
	return false
}

// AddEventWebhook registers or replaces an event-type → webhook URL mapping.
func (c *Config) AddEventWebhook(eventType, strURL string) error {
	u, err := validateWebhookURL(strURL)
	if err != nil {
		c.log.Errorf("Invalid webhook URL %s: %v", strURL, err)
		return err
	}
	for i, v := range c.EventWebhooks {
		if v.EventType == eventType {
			c.EventWebhooks[i].Webhook = *u
			return c.writeConfig()
		}
	}
	c.EventWebhooks = append(c.EventWebhooks, EventTypeWebhook{EventType: eventType, Webhook: *u})
	return c.writeConfig()
}

// RemoveEventWebhook removes the event-type webhook for the given eventType.
func (c *Config) RemoveEventWebhook(eventType string) error {
	for i, v := range c.EventWebhooks {
		if v.EventType == eventType {
			c.EventWebhooks = append(c.EventWebhooks[:i], c.EventWebhooks[i+1:]...)
			return c.writeConfig()
		}
	}
	c.log.Errorf("Event webhook not found for event_type: %s", eventType)
	return fmt.Errorf("event webhook not found for event_type: %s", eventType)
}

// GetEventWebhooks returns a copy of the current event webhooks as API-friendly structs.
func (c *Config) GetEventWebhooks() []EventTypeWebhookAPI {
	result := make([]EventTypeWebhookAPI, len(c.EventWebhooks))
	for i, ew := range c.EventWebhooks {
		result[i] = EventTypeWebhookAPI{EventType: ew.EventType, URL: ew.Webhook.String()}
	}
	return result
}

// SendToEventWebhooks fires all event webhooks whose pattern matches evtType.
// Each dispatch runs in its own goroutine; errors are logged but not returned.
func (c *Config) SendToEventWebhooks(evtType string, raw, extra interface{}) {
	for _, ew := range c.EventWebhooks {
		if matchesEventType(ew.EventType, evtType) {
			u := ew.Webhook
			go func() {
				if err := c.send(&u, evtType, raw, extra); err != nil {
					c.log.Warnf("Failed to send event to event webhook event_type=%s url=%s error=%v",
						evtType, u.String(), err)
				}
			}()
		}
	}
}

// SendTo sends a test event payload to an arbitrary URL.
func (c *Config) SendTo(rawURL, evtType string, raw, extra interface{}) error {
	u, err := validateWebhookURL(rawURL)
	if err != nil {
		return err
	}
	return c.send(u, evtType, raw, extra)
}

// ClearDefaultWebhook removes the default webhook URL.
func (c *Config) ClearDefaultWebhook() error {
	c.DefaultWebhook = url.URL{}
	return c.writeConfig()
}

// ClearErrorWebhook removes the error webhook URL.
func (c *Config) ClearErrorWebhook() error {
	c.ErrorWebhook = url.URL{}
	return c.writeConfig()
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

// SendToDefault sends an event payload to the default webhook asynchronously with retry.
func (c *Config) SendToDefault(evtType string, raw, extra interface{}) {
	if !c.HasDefaultWebhook() {
		return
	}
	c.SendAsync(c.GetDefaultWebhook(), evtType, raw, extra)
}

// SendToError sends an event payload to the error webhook asynchronously with retry.
func (c *Config) SendToError(evtType string, raw, extra interface{}) {
	if !c.HasErrorWebhook() {
		return
	}
	c.SendAsync(c.GetErrorWebhook(), evtType, raw, extra)
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

// sendOnce makes a single HTTP POST attempt. Returns (httpStatus, error).
// When Config.Secret is set, adds an X-ZapLab-Signature: sha256=<hex> header.
func (c *Config) sendOnce(webhookURL *url.URL, evtType string, raw, extra interface{}) (int, error) {
	if webhookURL.String() == "" {
		return 0, fmt.Errorf("webhookURL is empty")
	}
	body, err := json.Marshal([]eventPayload{{Type: evtType, Raw: raw, Extra: extra}})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest("POST", webhookURL.String(), bytes.NewBuffer(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Secret != "" {
		mac := hmac.New(sha256.New, []byte(c.Secret))
		mac.Write(body)
		req.Header.Set("X-ZapLab-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

// send is the internal single-attempt sender kept for backward-compat (SendTo, test endpoint).
func (c *Config) send(webhookURL *url.URL, evtType string, raw, extra interface{}) error {
	_, err := c.sendOnce(webhookURL, evtType, raw, extra)
	return err
}

// SendAsync dispatches the payload in a goroutine with up to 3 attempts (0 s, 5 s, 25 s backoff).
// After each attempt, OnDelivery is called if set.
func (c *Config) SendAsync(webhookURL *url.URL, evtType string, raw, extra interface{}) {
	go func() {
		delays := []time.Duration{0, 5 * time.Second, 25 * time.Second}
		for i, delay := range delays {
			if delay > 0 {
				time.Sleep(delay)
			}
			httpStatus, err := c.sendOnce(webhookURL, evtType, raw, extra)
			rec := DeliveryRecord{
				WebhookURL: webhookURL.String(),
				EventType:  evtType,
				Attempt:    i + 1,
				HTTPStatus: httpStatus,
			}
			if err != nil {
				rec.Status = "failed"
				rec.ErrorMsg = err.Error()
				c.log.Warnf("Webhook delivery attempt=%d url=%s err=%v", i+1, webhookURL.String(), err)
			} else {
				rec.Status = "delivered"
				rec.DeliveredAt = time.Now().UTC().Format(time.RFC3339)
			}
			if c.OnDelivery != nil {
				c.OnDelivery(rec)
			}
			if rec.Status == "delivered" {
				return
			}
		}
	}()
}

func validateURL(raw string) (*url.URL, error) {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %s err=%+v", raw, err)
	}
	return u, nil
}
