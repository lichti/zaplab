// Export — download events, conversations, and frame captures as CSV / JSON / HAR.
function exportSection() {
  return {
    // ── state ──
    expTab:      'events',   // 'events' | 'conversation' | 'frames'
    expFormat:   'csv',      // 'csv' | 'json' | 'har'
    expLimit:    1000,
    expType:     '',
    expFrom:     '',
    expTo:       '',
    expJid:      '',
    expHarLimit: 500,
    expLoading:  false,
    expError:    '',
    expSuccess:  '',

    // ── init ──
    initExport() {},

    // ── export ──
    async expDownload() {
      this.expLoading = true;
      this.expError   = '';
      this.expSuccess = '';
      try {
        let url = '', filename = '';

        if (this.expTab === 'events') {
          const p = new URLSearchParams({ format: this.expFormat, limit: this.expLimit });
          if (this.expType.trim()) p.set('type', this.expType.trim());
          if (this.expFrom)        p.set('from', this.expFrom);
          if (this.expTo)          p.set('to',   this.expTo);
          url      = `/zaplab/api/export/events?${p}`;
          filename = `zaplab-events-${this._ts()}.${this.expFormat}`;

        } else if (this.expTab === 'conversation') {
          if (!this.expJid.trim()) {
            this.expError = 'JID is required for conversation export';
            return;
          }
          const p = new URLSearchParams({ format: this.expFormat, limit: this.expLimit, jid: this.expJid.trim() });
          url      = `/zaplab/api/export/conversation?${p}`;
          filename = `zaplab-conv-${this.expJid.trim()}-${this._ts()}.${this.expFormat}`;

        } else { // frames HAR
          const p = new URLSearchParams({ limit: this.expHarLimit });
          if (this.expFrom) p.set('from', this.expFrom);
          if (this.expTo)   p.set('to',   this.expTo);
          url      = `/zaplab/api/export/frames/har?${p}`;
          filename = `zaplab-frames-${this._ts()}.har`;
        }

        const res = await fetch(url, { headers: apiHeaders() });
        if (!res.ok) { const t = await res.text(); throw new Error(t); }
        const blob = await res.blob();
        const a    = document.createElement('a');
        a.href     = URL.createObjectURL(blob);
        a.download = filename;
        a.click();
        URL.revokeObjectURL(a.href);
        this.expSuccess = `Downloaded ${filename}`;
      } catch (err) {
        this.expError = err.message || 'Export failed';
      } finally {
        this.expLoading = false;
      }
    },

    _ts() { return new Date().toISOString().slice(0, 16).replace('T', '-').replace(':', ''); },

    expTabSelect(tab) {
      this.expTab    = tab;
      this.expFormat = tab === 'frames' ? 'har' : 'csv';
      this.expError  = '';
      this.expSuccess = '';
    },
  };
}
