// Message Control section — state, init, preview helpers, submit.
function ctrlSection() {
  return {
    // ── state ──
    ctrl: {
      type:      'react',
      to:        '',
      messageId: '',
      senderJid: '',
      emoji:     '❤️',
      newText:   '',
      state:     'composing',
      media:     'text',
      timer:     '86400',
      loading:   false,
      toast:     null,
      result:    null,
    },
    ctrlPreviewTab:    localStorage.getItem('zaplab-ctrl-preview-tab') || 'curl',
    ctrlPreviewCopied: false,

    // ── section init ──
    initCtrl() {
      this.$watch('ctrl.type', () => {
        this.ctrl.toast  = null;
        this.ctrl.result = null;
        if (this.ctrlPreviewTab === 'response') this.ctrlPreviewTab = 'curl';
      });
      this.$watch('ctrlPreviewTab', val => {
        localStorage.setItem('zaplab-ctrl-preview-tab', val);
      });
    },

    // ── methods ──
    ctrlEndpoint() {
      return { react: '/zaplab/api/sendreaction', edit: '/zaplab/api/editmessage', delete: '/zaplab/api/revokemessage', typing: '/zaplab/api/settyping', disappearing: '/zaplab/api/setdisappearing' }[this.ctrl.type] || '';
    },

    ctrlLabel() {
      return { react: 'Send Reaction', edit: 'Edit Message', delete: 'Delete Message', typing: 'Set Typing', disappearing: 'Set Timer' }[this.ctrl.type] || 'Send';
    },

    ctrlCurlPayload() {
      const to = this.ctrl.to || '<to>';
      switch (this.ctrl.type) {
        case 'react':
          return { to, message_id: this.ctrl.messageId || '<message_id>', sender_jid: this.ctrl.senderJid || '<sender_jid>', emoji: this.ctrl.emoji };
        case 'edit':
          return { to, message_id: this.ctrl.messageId || '<message_id>', new_text: this.ctrl.newText || '<new_text>' };
        case 'delete':
          return { to, message_id: this.ctrl.messageId || '<message_id>', sender_jid: this.ctrl.senderJid || '<sender_jid>' };
        case 'typing':
          return { to, state: this.ctrl.state, media: this.ctrl.media };
        case 'disappearing':
          return { to, timer: parseInt(this.ctrl.timer) };
        default:
          return {};
      }
    },

    ctrlCurlPreview() {
      const token   = this.apiToken || '<your-api-token>';
      const url     = `${window.location.origin}${this.ctrlEndpoint()}`;
      const payload = JSON.stringify(this.ctrlCurlPayload());
      return [
        `# Authentication enabled — X-API-Token or Dashboard Session required`,
        `curl -X POST \\`,
        `  ${url} \\`,
        `  -H "Content-Type: application/json" \\`,
        `  -H "X-API-Token: ${token}" \\`,
        `  -d '${payload}'`,
      ].join('\n');
    },

    ctrlResultPreview() {
      if (!this.ctrl.result) {
        return '<span style="color:var(--tw-prose-captions,#8b949e)">No response yet — submit an action first.</span>';
      }
      return this.highlight(this.ctrl.result);
    },

    ctrlPreviewContent() {
      if (this.ctrlPreviewTab === 'response') return this.ctrlResultPreview();
      return this.highlightCurl(this.ctrlCurlPreview());
    },

    async copyCtrlPreview() {
      const text = this.ctrlPreviewTab === 'response'
        ? JSON.stringify(this.ctrl.result, null, 2)
        : this.ctrlCurlPreview();
      try {
        await navigator.clipboard.writeText(text);
        this.ctrlPreviewCopied = true;
        setTimeout(() => { this.ctrlPreviewCopied = false; }, 2000);
      } catch {}
    },

    async submitCtrl() {
      this.ctrl.toast   = null;
      this.ctrl.loading = true;
      try {
        const res = await fetch(this.ctrlEndpoint(), {
          method:  'POST',
          headers: { 'Content-Type': 'application/json', 'X-API-Token': this.apiToken },
          body:    JSON.stringify(this.ctrlCurlPayload()),
        });
        const data = await res.json();
        this.ctrl.toast = { ok: res.ok, message: data.message || JSON.stringify(data) };
        if (res.ok) {
          this.ctrl.result   = data;
          this.ctrlPreviewTab = 'response';
        }
      } catch (err) {
        this.ctrl.toast = { ok: false, message: err.message };
      } finally {
        this.ctrl.loading = false;
      }
    },
  };
}
