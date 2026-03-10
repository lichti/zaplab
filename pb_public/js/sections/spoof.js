// Spoof Messages section — spoofed replies, timed messages, and demo sequences.
function spoofSection() {
  return {
    // ── state ──
    spoof: {
      type:        'reply',
      to:          '',
      fromJid:     '',
      msgId:       '',
      quotedText:  '',
      text:        '',
      image:       '',    // base64
      imageName:   '',
      gender:      'boy',
      language:    'br',
      loading:     false,
      toast:       null,
      result:      null,
    },
    spoofPreviewTab:    localStorage.getItem('zaplab-spoof-preview-tab') || 'curl',
    spoofPreviewCopied: false,

    // ── section init ──
    initSpoof() {
      this.$watch('spoof.type', () => {
        this.spoof.toast  = null;
        this.spoof.result = null;
        if (this.spoofPreviewTab === 'response') this.spoofPreviewTab = 'curl';
      });
      this.$watch('spoofPreviewTab', val => {
        localStorage.setItem('zaplab-spoof-preview-tab', val);
      });
    },

    // ── helpers ──
    spoofNeedsFrom() {
      return ['reply', 'reply-private', 'reply-img', 'reply-location', 'demo'].includes(this.spoof.type);
    },

    spoofNeedsText() {
      return ['reply', 'reply-private', 'reply-img', 'reply-location', 'timed'].includes(this.spoof.type);
    },

    spoofNeedsQuoted() {
      return ['reply', 'reply-private', 'reply-img'].includes(this.spoof.type);
    },

    spoofNeedsImage() {
      return this.spoof.type === 'reply-img';
    },

    spoofIsDisabled() {
      if (this.spoof.loading) return true;
      if (!this.spoof.to.trim()) return true;
      if (this.spoofNeedsFrom() && !this.spoof.fromJid.trim()) return true;
      if (this.spoof.type === 'reply-img' && !this.spoof.image) return true;
      return false;
    },

    spoofEndpoint() {
      return {
        'reply':          '/zaplab/api/spoof/reply',
        'reply-private':  '/zaplab/api/spoof/reply-private',
        'reply-img':      '/zaplab/api/spoof/reply-img',
        'reply-location': '/zaplab/api/spoof/reply-location',
        'timed':          '/zaplab/api/spoof/timed',
        'demo':           '/zaplab/api/spoof/demo',
      }[this.spoof.type] || '';
    },

    spoofLabel() {
      return {
        'reply':          'Send Spoofed Reply',
        'reply-private':  'Send Spoofed Reply (Private)',
        'reply-img':      'Send Spoofed Image Reply',
        'reply-location': 'Send Spoofed Location Reply',
        'timed':          'Send Timed Message',
        'demo':           'Run Demo Sequence',
      }[this.spoof.type] || 'Send';
    },

    spoofCurlPayload() {
      const to      = this.spoof.to      || '<to>';
      const fromJid = this.spoof.fromJid || '<from_jid>';
      const msgId   = this.spoof.msgId   || '<msg_id or leave empty to auto-generate>';
      switch (this.spoof.type) {
        case 'reply':
          return { to, from_jid: fromJid, msg_id: msgId, quoted_text: this.spoof.quotedText || '<quoted_text>', text: this.spoof.text || '<text>' };
        case 'reply-private':
          return { to, from_jid: fromJid, msg_id: msgId, quoted_text: this.spoof.quotedText || '<quoted_text>', text: this.spoof.text || '<text>' };
        case 'reply-img':
          return { to, from_jid: fromJid, msg_id: msgId, image: this.spoof.image ? '<base64_image>' : '<base64_image>', quoted_text: this.spoof.quotedText || '<quoted_text>', text: this.spoof.text || '<text>' };
        case 'reply-location':
          return { to, from_jid: fromJid, msg_id: msgId, text: this.spoof.text || '<text>' };
        case 'timed':
          return { to, text: this.spoof.text || '<text>' };
        case 'demo':
          return { to, from_jid: fromJid, gender: this.spoof.gender, language: this.spoof.language, image: '<base64_image (optional)>' };
        default:
          return {};
      }
    },

    spoofCurlPreview() {
      const token   = this.apiToken || '<your-api-token>';
      const url     = `${window.location.origin}${this.spoofEndpoint()}`;
      const payload = JSON.stringify(this.spoofCurlPayload());
      return [
        `# auth disabled — X-API-Token not required`,
        `curl -X POST \\`,
        `  ${url} \\`,
        `  -H "Content-Type: application/json" \\`,
        `  -H "X-API-Token: ${token}" \\`,
        `  -d '${payload}'`,
      ].join('\n');
    },

    spoofResultPreview() {
      if (!this.spoof.result) {
        return '<span style="color:var(--tw-prose-captions,#8b949e)">No response yet — submit an action first.</span>';
      }
      return this.highlight(this.spoof.result);
    },

    spoofPreviewContent() {
      if (this.spoofPreviewTab === 'response') return this.spoofResultPreview();
      return this.highlightCurl(this.spoofCurlPreview());
    },

    async copySpoofPreview() {
      const text = this.spoofPreviewTab === 'response'
        ? JSON.stringify(this.spoof.result, null, 2)
        : this.spoofCurlPreview();
      try {
        await navigator.clipboard.writeText(text);
        this.spoofPreviewCopied = true;
        setTimeout(() => { this.spoofPreviewCopied = false; }, 2000);
      } catch {}
    },

    // ── image picker ──
    spoofLoadImage(event) {
      const file = event.target.files[0];
      if (!file) return;
      this.spoof.imageName = file.name;
      const reader = new FileReader();
      reader.onload = (e) => {
        // strip data URL prefix to get raw base64
        this.spoof.image = e.target.result.split(',')[1];
      };
      reader.readAsDataURL(file);
    },

    // ── submit ──
    async submitSpoof() {
      this.spoof.toast   = null;
      this.spoof.loading = true;
      try {
        const payload = { ...this.spoofCurlPayload() };
        // replace placeholder with real base64 for image types
        if (this.spoof.type === 'reply-img' && this.spoof.image) {
          payload.image = this.spoof.image;
        }
        if (this.spoof.type === 'demo' && this.spoof.image) {
          payload.image = this.spoof.image;
        } else if (this.spoof.type === 'demo') {
          delete payload.image;
        }
        // use actual msg_id (empty = server generates)
        if ('msg_id' in payload) payload.msg_id = this.spoof.msgId || '';

        const res  = await fetch(this.spoofEndpoint(), {
          method:  'POST',
          headers: { 'Content-Type': 'application/json', 'X-API-Token': this.apiToken },
          body:    JSON.stringify(payload),
        });
        const data = await res.json();
        this.spoof.toast = { ok: res.ok, message: data.message || JSON.stringify(data) };
        if (res.ok) {
          this.spoof.result    = data;
          this.spoofPreviewTab = 'response';
        }
      } catch (err) {
        this.spoof.toast = { ok: false, message: err.message };
      } finally {
        this.spoof.loading = false;
      }
    },
  };
}
