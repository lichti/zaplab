// Contacts & Polls section — state, init, preview helpers, submit.
function contactsSection() {
  return {
    // ── state ──
    contacts: {
      type:            'contact',
      to:              '',
      displayName:     '',
      vcard:           '',
      contactsList:    [{ name: '', vcard: '' }],
      question:        '',
      options:         ['', ''],
      selectableCount: '1',
      pollMessageId:   '',
      pollSenderJid:   '',
      selectedOptions: '',
      loading:         false,
      toast:           null,
      result:          null,
    },
    contactsPreviewTab:    localStorage.getItem('zaplab-contacts-preview-tab') || 'curl',
    contactsPreviewCopied: false,

    // ── section init ──
    initContacts() {
      this.$watch('contacts.type', () => {
        this.contacts.toast  = null;
        this.contacts.result = null;
        if (this.contactsPreviewTab === 'response') this.contactsPreviewTab = 'curl';
      });
      this.$watch('contactsPreviewTab', val => {
        localStorage.setItem('zaplab-contacts-preview-tab', val);
      });
    },

    // ── methods ──
    addContact() {
      this.contacts.contactsList.push({ name: '', vcard: '' });
    },
    removeContact(i) {
      if (this.contacts.contactsList.length > 1) this.contacts.contactsList.splice(i, 1);
    },
    addPollOption() {
      if (this.contacts.options.length < 12) this.contacts.options.push('');
    },
    removePollOption(i) {
      if (this.contacts.options.length > 2) this.contacts.options.splice(i, 1);
    },

    contactsEndpoint() {
      return { contact: '/sendcontact', contacts: '/sendcontacts', poll: '/createpoll', votepoll: '/votepoll' }[this.contacts.type] || '';
    },

    contactsLabel() {
      return { contact: 'Send Contact', contacts: 'Send Contacts', poll: 'Create Poll', votepoll: 'Vote on Poll' }[this.contacts.type] || 'Send';
    },

    contactsCurlPayload() {
      const to = this.contacts.to || '<to>';
      switch (this.contacts.type) {
        case 'contact':
          return { to, display_name: this.contacts.displayName || '<name>', vcard: this.contacts.vcard || '<vcard>' };
        case 'contacts':
          return { to, display_name: this.contacts.displayName || '<name>', contacts: this.contacts.contactsList.map(c => ({ name: c.name || '<name>', vcard: c.vcard || '<vcard>' })) };
        case 'poll':
          return { to, question: this.contacts.question || '<question>', options: this.contacts.options.filter(o => o.trim()), selectable_count: parseInt(this.contacts.selectableCount) };
        case 'votepoll':
          return { to, poll_message_id: this.contacts.pollMessageId || '<poll_message_id>', poll_sender_jid: this.contacts.pollSenderJid || '<poll_sender_jid>', selected_options: this.contacts.selectedOptions.split('\n').map(s => s.trim()).filter(Boolean) };
        default:
          return {};
      }
    },

    contactsCurlPreview() {
      const token   = this.apiToken || '<your-api-token>';
      const url     = `${window.location.origin}${this.contactsEndpoint()}`;
      const payload = JSON.stringify(this.contactsCurlPayload());
      return [
        `# auth disabled — X-API-Token not required`,
        `curl -X POST \\`,
        `  ${url} \\`,
        `  -H "Content-Type: application/json" \\`,
        `  -H "X-API-Token: ${token}" \\`,
        `  -d '${payload}'`,
      ].join('\n');
    },

    contactsResultPreview() {
      if (!this.contacts.result) {
        return '<span style="color:var(--tw-prose-captions,#8b949e)">No response yet — submit an action first.</span>';
      }
      return this.highlight(this.contacts.result);
    },

    contactsPreviewContent() {
      if (this.contactsPreviewTab === 'response') return this.contactsResultPreview();
      return this.highlightCurl(this.contactsCurlPreview());
    },

    async copyContactsPreview() {
      const text = this.contactsPreviewTab === 'response'
        ? JSON.stringify(this.contacts.result, null, 2)
        : this.contactsCurlPreview();
      try {
        await navigator.clipboard.writeText(text);
        this.contactsPreviewCopied = true;
        setTimeout(() => { this.contactsPreviewCopied = false; }, 2000);
      } catch {}
    },

    async submitContacts() {
      this.contacts.toast   = null;
      this.contacts.loading = true;
      try {
        const res = await fetch(this.contactsEndpoint(), {
          method:  'POST',
          headers: { 'Content-Type': 'application/json', 'X-API-Token': this.apiToken },
          body:    JSON.stringify(this.contactsCurlPayload()),
        });
        const data = await res.json();
        this.contacts.toast = { ok: res.ok, message: data.message || JSON.stringify(data) };
        if (res.ok) {
          this.contacts.result    = data;
          this.contactsPreviewTab = 'response';
        }
      } catch (err) {
        this.contacts.toast = { ok: false, message: err.message };
      } finally {
        this.contacts.loading = false;
      }
    },
  };
}
