// Contacts & Polls section — send contacts/vCards/polls + contact management.
function contactsSection() {
  return {
    // ── state ──
    contacts: {
      type:            'list',
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
      phonesToCheck:   '',
      infoJid:         '',
      loading:         false,
      toast:           null,
      result:          null,
    },
    contactsPreviewTab:    localStorage.getItem('zaplab-contacts-preview-tab') || 'curl',
    contactsPreviewCopied: false,

    // ── Contact list picker state ──
    contactsStoreList:      [],
    contactsStoreLoading:   false,
    contactsStoreFilter:    '',
    contactsPickerOpen:     false,

    // ── section init ──
    initContacts() {
      this.$watch('contacts.type', () => {
        this.contacts.toast  = null;
        this.contacts.result = null;
        if (this.contactsPreviewTab === 'response' || this.contactsPreviewTab === 'table') {
          this.contactsPreviewTab = 'curl';
        }
        if (this.contacts.type === 'list' && this.contactsStoreList.length === 0 && !this.contactsStoreLoading) {
          this.loadContactsStore();
        }
      });
      this.$watch('contactsPreviewTab', val => {
        localStorage.setItem('zaplab-contacts-preview-tab', val);
      });
    },

    // ── helpers ──
    contactsIsMgmt() {
      return ['list', 'check', 'info'].includes(this.contacts.type);
    },

    contactsHasTable() {
      if (!this.contacts.result) return false;
      return ['list', 'check'].includes(this.contacts.type);
    },

    contactsIsDisabled() {
      if (this.contacts.loading) return true;
      if (this.contacts.type === 'list')  return false;
      if (this.contacts.type === 'check') return !this.contacts.phonesToCheck.trim();
      if (this.contacts.type === 'info')  return !this.contacts.infoJid.trim();
      return !this.contacts.to;
    },

    contactsMethod() {
      return ['list', 'info'].includes(this.contacts.type) ? 'GET' : 'POST';
    },

    contactsEndpoint() {
      switch (this.contacts.type) {
        case 'list':     return '/contacts';
        case 'check':    return '/contacts/check';
        case 'info':     return `/contacts/${encodeURIComponent(this.contacts.infoJid || '<jid>')}`;
        case 'contact':  return '/sendcontact';
        case 'contacts': return '/sendcontacts';
        case 'poll':     return '/createpoll';
        case 'votepoll': return '/votepoll';
        default:         return '';
      }
    },

    contactsLabel() {
      return {
        list:     'List Contacts',
        check:    'Check Numbers',
        info:     'Get Info',
        contact:  'Send Contact',
        contacts: 'Send Contacts',
        poll:     'Create Poll',
        votepoll: 'Vote on Poll',
      }[this.contacts.type] || 'Submit';
    },

    contactsCurlPayload() {
      const to = this.contacts.to || '<to>';
      switch (this.contacts.type) {
        case 'check':
          return { phones: this.contacts.phonesToCheck.split('\n').map(p => p.trim()).filter(Boolean) };
        case 'contact':
          return { to, display_name: this.contacts.displayName || '<name>', vcard: this.contacts.vcard || '<vcard>' };
        case 'contacts':
          return { to, display_name: this.contacts.displayName || '<name>', contacts: this.contacts.contactsList.map(c => ({ name: c.name || '<name>', vcard: c.vcard || '<vcard>' })) };
        case 'poll':
          return { to, question: this.contacts.question || '<question>', options: this.contacts.options.filter(o => o.trim()), selectable_count: parseInt(this.contacts.selectableCount) };
        case 'votepoll':
          return { to, poll_message_id: this.contacts.pollMessageId || '<poll_message_id>', poll_sender_jid: this.contacts.pollSenderJid || '<poll_sender_jid>', selected_options: this.contacts.selectedOptions.split('\n').map(s => s.trim()).filter(Boolean) };
        default:
          return null;
      }
    },

    contactsCurlPreview() {
      const token   = this.apiToken || '<your-api-token>';
      const method  = this.contactsMethod();
      const url     = `${window.location.origin}${this.contactsEndpoint()}`;
      const payload = this.contactsCurlPayload();
      const lines = [
        `# auth disabled — X-API-Token not required`,
        `curl -X ${method} \\`,
        `  ${url} \\`,
        `  -H "Content-Type: application/json" \\`,
        `  -H "X-API-Token: ${token}"`,
      ];
      if (payload !== null) {
        lines[lines.length - 1] += ' \\';
        lines.push(`  -d '${JSON.stringify(payload)}'`);
      }
      return lines.join('\n');
    },

    contactsResultPreview() {
      if (!this.contacts.result) {
        return '<span style="color:var(--tw-prose-captions,#8b949e)">No response yet — submit an action first.</span>';
      }
      return this.highlight(this.contacts.result);
    },

    contactsPreviewContent() {
      if (this.contactsPreviewTab === 'response') return this.contactsResultPreview();
      if (this.contactsPreviewTab === 'table')    return '';
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

    // ── Contact store picker ──
    contactsStoreFiltered() {
      const q = this.contactsStoreFilter.toLowerCase();
      if (!q) return this.contactsStoreList;
      return this.contactsStoreList.filter(c =>
        (c.full_name     || '').toLowerCase().includes(q) ||
        (c.push_name     || '').toLowerCase().includes(q) ||
        (c.business_name || '').toLowerCase().includes(q) ||
        (c.phone         || '').includes(q) ||
        (c.jid           || '').toLowerCase().includes(q)
      );
    },

    async loadContactsStore() {
      this.contactsStoreLoading = true;
      try {
        const res  = await fetch('/contacts', { headers: { 'X-API-Token': this.apiToken } });
        const data = await res.json();
        if (res.ok) this.contactsStoreList = data.contacts || [];
      } catch {}
      this.contactsStoreLoading = false;
    },

    selectContactFromPicker(c) {
      this.contacts.infoJid  = c.jid;
      this.contactsPickerOpen = false;
    },

    contactDisplayName(c) {
      return c.full_name || c.push_name || c.business_name || c.phone || c.jid;
    },

    // ── Polls helpers ──
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

    // ── CSV export for list ──
    exportContactsCSV() {
      if (!this.contacts.result || !this.contacts.result.contacts) return;
      const rows = [['JID', 'Phone', 'Full Name', 'Push Name', 'Business Name']];
      for (const c of this.contacts.result.contacts) {
        rows.push([
          c.jid || '',
          c.phone || '',
          `"${(c.full_name     || '').replace(/"/g, '""')}"`,
          `"${(c.push_name     || '').replace(/"/g, '""')}"`,
          `"${(c.business_name || '').replace(/"/g, '""')}"`,
        ]);
      }
      const csv  = rows.map(r => r.join(',')).join('\n');
      const blob = new Blob([csv], { type: 'text/csv' });
      const a    = document.createElement('a');
      a.href     = URL.createObjectURL(blob);
      a.download = `contacts-${Date.now()}.csv`;
      a.click();
    },

    // ── Quick action from table ──
    contactsQuickInfo(jid) {
      this.contacts.type    = 'info';
      this.contacts.infoJid = jid;
      this.$nextTick(() => this.submitContacts());
    },

    // ── Submit ──
    async submitContacts() {
      this.contacts.toast   = null;
      this.contacts.loading = true;
      try {
        const method  = this.contactsMethod();
        const payload = this.contactsCurlPayload();
        const opts = {
          method,
          headers: { 'Content-Type': 'application/json', 'X-API-Token': this.apiToken },
        };
        if (payload !== null) opts.body = JSON.stringify(payload);
        const res  = await fetch(this.contactsEndpoint(), opts);
        const data = await res.json();
        const successLabel = {
          list:  `${(data.contacts || []).length} contacts loaded`,
          check: `${(data.results  || []).length} numbers checked`,
          info:  data.jid ? `Info loaded: ${data.push_name || data.jid}` : 'Info loaded',
        }[this.contacts.type];
        const toastMsg = data.message || (res.ok ? successLabel : null) || 'Done';
        this.contacts.toast = { ok: res.ok, message: toastMsg };
        if (res.ok) {
          this.contacts.result    = data;
          this.contactsPreviewTab = this.contactsHasTable() ? 'table' : 'response';
          // refresh picker cache on list
          if (this.contacts.type === 'list') {
            this.contactsStoreList = data.contacts || [];
          }
        }
      } catch (err) {
        this.contacts.toast = { ok: false, message: err.message };
      } finally {
        this.contacts.loading = false;
      }
    },
  };
}
