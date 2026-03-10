// Webhook configuration section.
function webhookSection() {
  const KNOWN_EVENT_TYPES = [
    'Message',
    'Message.*',
    'Message.ImageMessage',
    'Message.AudioMessage',
    'Message.VideoMessage',
    'Message.DocumentMessage',
    'Message.StickerMessage',
    'Message.ContactMessage',
    'Message.LocationMessage',
    'Message.LiveLocationMessage',
    'Message.PollUpdateMessage',
    'Message.EncReactionMessage',
    'ReceiptRead',
    'ReceiptReadSelf',
    'ReceiptDelivered',
    'ReceiptPlayed',
    'ReceiptSender',
    'ReceiptRetry',
    'Presence.Online',
    'Presence.Offline',
    'Presence.OfflineLastSeen',
    'HistorySync',
    'AppStateSyncComplete',
    'Connected',
    'PushNameSetting',
    'SentMessage',
  ];

  return {
    wh: {
      defaultUrl:    '',
      errorUrl:      '',
      eventWebhooks: [],
      newEventType:  '',
      newEventUrl:   '',
      textWebhooks:      [],
      newTextMatchType:  'prefix',
      newTextPattern:    '',
      newTextFrom:       'all',
      newTextCaseSensitive: false,
      newTextUrl:        '',
      testUrl:       '',
      testResult:    null,
      saving:        false,
      testing:       false,
      toast:         null,
      activeTab:     'event',  // 'event' | 'text'
    },
    whKnownEventTypes: KNOWN_EVENT_TYPES,

    initWebhook() {
      this.$watch('activeSection', val => {
        if (val === 'webhook') this.loadWebhookConfig();
      });
    },

    async loadWebhookConfig() {
      try {
        const res  = await fetch('/zaplab/api/webhook');
        const data = await res.json();
        if (!res.ok) { this.wh.toast = { ok: false, message: data.error || 'Failed to load config' }; return; }
        this.wh.defaultUrl    = data.default_url    || '';
        this.wh.errorUrl      = data.error_url      || '';
        this.wh.eventWebhooks = data.event_webhooks || [];
        this.wh.textWebhooks  = data.text_webhooks  || [];
      } catch (err) {
        this.wh.toast = { ok: false, message: err.message };
      }
    },

    async whSaveDefault() {
      this.wh.saving = true; this.wh.toast = null;
      try {
        const res  = await fetch('/zaplab/api/webhook/default', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ url: this.wh.defaultUrl }),
        });
        const data = await res.json();
        this.wh.toast = { ok: res.ok, message: data.message || data.error || '' };
      } catch (err) {
        this.wh.toast = { ok: false, message: err.message };
      } finally {
        this.wh.saving = false;
      }
    },

    async whClearDefault() {
      this.wh.toast = null;
      try {
        const res  = await fetch('/zaplab/api/webhook/default', { method: 'DELETE' });
        const data = await res.json();
        if (res.ok) this.wh.defaultUrl = '';
        this.wh.toast = { ok: res.ok, message: data.message || data.error || '' };
      } catch (err) {
        this.wh.toast = { ok: false, message: err.message };
      }
    },

    async whSaveError() {
      this.wh.saving = true; this.wh.toast = null;
      try {
        const res  = await fetch('/zaplab/api/webhook/error', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ url: this.wh.errorUrl }),
        });
        const data = await res.json();
        this.wh.toast = { ok: res.ok, message: data.message || data.error || '' };
      } catch (err) {
        this.wh.toast = { ok: false, message: err.message };
      } finally {
        this.wh.saving = false;
      }
    },

    async whClearError() {
      this.wh.toast = null;
      try {
        const res  = await fetch('/zaplab/api/webhook/error', { method: 'DELETE' });
        const data = await res.json();
        if (res.ok) this.wh.errorUrl = '';
        this.wh.toast = { ok: res.ok, message: data.message || data.error || '' };
      } catch (err) {
        this.wh.toast = { ok: false, message: err.message };
      }
    },

    async whAddEventWebhook() {
      if (!this.wh.newEventType || !this.wh.newEventUrl) {
        this.wh.toast = { ok: false, message: 'Event type and URL are required' };
        return;
      }
      this.wh.toast = null;
      try {
        const res  = await fetch('/zaplab/api/webhook/events', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ event_type: this.wh.newEventType, url: this.wh.newEventUrl }),
        });
        const data = await res.json();
        this.wh.toast = { ok: res.ok, message: data.message || data.error || '' };
        if (res.ok) {
          this.wh.newEventType = '';
          this.wh.newEventUrl  = '';
          await this.loadWebhookConfig();
        }
      } catch (err) {
        this.wh.toast = { ok: false, message: err.message };
      }
    },

    async whRemoveEventWebhook(eventType) {
      this.wh.toast = null;
      try {
        const res  = await fetch('/zaplab/api/webhook/events', {
          method: 'DELETE',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ event_type: eventType }),
        });
        const data = await res.json();
        this.wh.toast = { ok: res.ok, message: data.message || data.error || '' };
        if (res.ok) await this.loadWebhookConfig();
      } catch (err) {
        this.wh.toast = { ok: false, message: err.message };
      }
    },

    async whAddTextWebhook() {
      if (!this.wh.newTextPattern || !this.wh.newTextUrl) {
        this.wh.toast = { ok: false, message: 'Pattern and URL are required' };
        return;
      }
      this.wh.toast = null;
      try {
        const res  = await fetch('/zaplab/api/webhook/text', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            match_type:     this.wh.newTextMatchType,
            pattern:        this.wh.newTextPattern,
            from:           this.wh.newTextFrom,
            case_sensitive: this.wh.newTextCaseSensitive,
            url:            this.wh.newTextUrl,
          }),
        });
        const data = await res.json();
        this.wh.toast = { ok: res.ok, message: data.message || data.error || '' };
        if (res.ok) {
          this.wh.newTextPattern = '';
          this.wh.newTextUrl     = '';
          await this.loadWebhookConfig();
        }
      } catch (err) {
        this.wh.toast = { ok: false, message: err.message };
      }
    },

    async whRemoveTextWebhook(id) {
      this.wh.toast = null;
      try {
        const res  = await fetch('/zaplab/api/webhook/text', {
          method: 'DELETE',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ id }),
        });
        const data = await res.json();
        this.wh.toast = { ok: res.ok, message: data.message || data.error || '' };
        if (res.ok) await this.loadWebhookConfig();
      } catch (err) {
        this.wh.toast = { ok: false, message: err.message };
      }
    },

    async whTest() {
      if (!this.wh.testUrl) { this.wh.toast = { ok: false, message: 'Test URL is required' }; return; }
      this.wh.testing = true; this.wh.testResult = null; this.wh.toast = null;
      try {
        const res  = await fetch('/zaplab/api/webhook/test', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ url: this.wh.testUrl }),
        });
        const data = await res.json();
        this.wh.testResult = data;
        this.wh.toast = { ok: res.ok, message: data.message || data.error || '' };
      } catch (err) {
        this.wh.toast = { ok: false, message: err.message };
      } finally {
        this.wh.testing = false;
      }
    },
  };
}
