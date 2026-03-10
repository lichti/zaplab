// Contacts Management section — list, check, and get info about contacts.
function contactsMgmtSection() {
  return {
    // ── state ──
    mgmt: {
      type:          'list',
      phonesToCheck: '',
      infoJid:       '',
      loading:       false,
      toast:         null,
      result:        null,
    },
    mgmtPreviewTab:    localStorage.getItem('zaplab-mgmt-preview-tab') || 'curl',
    mgmtPreviewCopied: false,

    // ── contact picker state ──
    mgmtStoreList:    [],
    mgmtStoreLoading: false,
    mgmtStoreFilter:  '',
    mgmtPickerOpen:   false,

    // ── section init ──
    initContactsMgmt() {
      this.$watch('mgmt.type', () => {
        this.mgmt.toast  = null;
        this.mgmt.result = null;
        if (this.mgmtPreviewTab === 'response' || this.mgmtPreviewTab === 'table') {
          this.mgmtPreviewTab = 'curl';
        }
        if (this.mgmt.type === 'list' && this.mgmtStoreList.length === 0 && !this.mgmtStoreLoading) {
          this.loadMgmtStore();
        }
      });
      this.$watch('mgmtPreviewTab', val => {
        localStorage.setItem('zaplab-mgmt-preview-tab', val);
      });
    },

    // ── helpers ──
    mgmtIsDisabled() {
      if (this.mgmt.loading) return true;
      if (this.mgmt.type === 'list')  return false;
      if (this.mgmt.type === 'check') return !this.mgmt.phonesToCheck.trim();
      if (this.mgmt.type === 'info')  return !this.mgmt.infoJid.trim();
      return false;
    },

    mgmtHasTable() {
      if (!this.mgmt.result) return false;
      return ['list', 'check'].includes(this.mgmt.type);
    },

    mgmtMethod() {
      return ['list', 'info'].includes(this.mgmt.type) ? 'GET' : 'POST';
    },

    mgmtEndpoint() {
      switch (this.mgmt.type) {
        case 'list':  return '/contacts';
        case 'check': return '/contacts/check';
        case 'info':  return `/contacts/${encodeURIComponent(this.mgmt.infoJid || '<jid>')}`;
        default:      return '';
      }
    },

    mgmtLabel() {
      return {
        list:  'List Contacts',
        check: 'Check Numbers',
        info:  'Get Info',
      }[this.mgmt.type] || 'Submit';
    },

    mgmtCurlPayload() {
      if (this.mgmt.type === 'check') {
        return { phones: this.mgmt.phonesToCheck.split('\n').map(p => p.trim()).filter(Boolean) };
      }
      return null;
    },

    mgmtCurlPreview() {
      const token   = this.apiToken || '<your-api-token>';
      const method  = this.mgmtMethod();
      const url     = `${window.location.origin}${this.mgmtEndpoint()}`;
      const payload = this.mgmtCurlPayload();
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

    mgmtResultPreview() {
      if (!this.mgmt.result) {
        return '<span style="color:var(--tw-prose-captions,#8b949e)">No response yet — submit an action first.</span>';
      }
      return this.highlight(this.mgmt.result);
    },

    mgmtPreviewContent() {
      if (this.mgmtPreviewTab === 'response') return this.mgmtResultPreview();
      if (this.mgmtPreviewTab === 'table')    return '';
      return this.highlightCurl(this.mgmtCurlPreview());
    },

    async copyMgmtPreview() {
      const text = this.mgmtPreviewTab === 'response'
        ? JSON.stringify(this.mgmt.result, null, 2)
        : this.mgmtCurlPreview();
      try {
        await navigator.clipboard.writeText(text);
        this.mgmtPreviewCopied = true;
        setTimeout(() => { this.mgmtPreviewCopied = false; }, 2000);
      } catch {}
    },

    // ── contact picker ──
    mgmtStoreFiltered() {
      const q = this.mgmtStoreFilter.toLowerCase();
      if (!q) return this.mgmtStoreList;
      return this.mgmtStoreList.filter(c =>
        (c.full_name     || '').toLowerCase().includes(q) ||
        (c.push_name     || '').toLowerCase().includes(q) ||
        (c.business_name || '').toLowerCase().includes(q) ||
        (c.phone         || '').includes(q) ||
        (c.jid           || '').toLowerCase().includes(q)
      );
    },

    async loadMgmtStore() {
      this.mgmtStoreLoading = true;
      try {
        const res  = await fetch('/contacts', { headers: { 'X-API-Token': this.apiToken } });
        const data = await res.json();
        if (res.ok) this.mgmtStoreList = data.contacts || [];
      } catch {}
      this.mgmtStoreLoading = false;
    },

    selectMgmtFromPicker(c) {
      this.mgmt.infoJid  = c.jid;
      this.mgmtPickerOpen = false;
    },

    mgmtDisplayName(c) {
      return c.full_name || c.push_name || c.business_name || c.phone || c.jid;
    },

    // ── CSV export ──
    exportMgmtCSV() {
      if (!this.mgmt.result || !this.mgmt.result.contacts) return;
      const rows = [['JID', 'Phone', 'Full Name', 'Push Name', 'Business Name']];
      for (const c of this.mgmt.result.contacts) {
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

    // ── quick action from list table ──
    mgmtQuickInfo(jid) {
      this.mgmt.type    = 'info';
      this.mgmt.infoJid = jid;
      this.$nextTick(() => this.submitMgmt());
    },

    // ── submit ──
    async submitMgmt() {
      this.mgmt.toast   = null;
      this.mgmt.loading = true;
      try {
        const method  = this.mgmtMethod();
        const payload = this.mgmtCurlPayload();
        const opts = {
          method,
          headers: { 'Content-Type': 'application/json', 'X-API-Token': this.apiToken },
        };
        if (payload !== null) opts.body = JSON.stringify(payload);
        const res  = await fetch(this.mgmtEndpoint(), opts);
        const data = await res.json();
        const successLabel = {
          list:  `${(data.contacts || []).length} contacts loaded`,
          check: `${(data.results  || []).length} numbers checked`,
          info:  data.jid ? `Info loaded: ${data.push_name || data.jid}` : 'Info loaded',
        }[this.mgmt.type];
        const toastMsg = data.message || (res.ok ? successLabel : null) || 'Done';
        this.mgmt.toast = { ok: res.ok, message: toastMsg };
        if (res.ok) {
          this.mgmt.result    = data;
          this.mgmtPreviewTab = this.mgmtHasTable() ? 'table' : 'response';
          if (this.mgmt.type === 'list') {
            this.mgmtStoreList = data.contacts || [];
          }
        }
      } catch (err) {
        this.mgmt.toast = { ok: false, message: err.message };
      } finally {
        this.mgmt.loading = false;
      }
    },
  };
}
