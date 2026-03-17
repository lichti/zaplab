// Receipt Latency Tracker — shows delivery/read receipt times and latency stats.
function receiptsSection() {
  return {
    // ── state ──
    rlLoading: false,
    rlError:   '',
    rlRows:    [],
    rlStats:   null,
    rlDays:    30,
    rlJid:     '',
    rlType:    '',

    // ── init ──
    initReceipts() {},

    // ── data ──
    async rlLoad() {
      if (this.rlLoading) return;
      this.rlLoading = true;
      this.rlError   = '';
      try {
        const p = new URLSearchParams({ days: this.rlDays, limit: 500 });
        if (this.rlJid.trim())  p.set('jid',  this.rlJid.trim());
        if (this.rlType.trim()) p.set('type', this.rlType.trim());
        const res = await fetch(`/zaplab/api/stats/receipt-latency?${p}`, { headers: this.apiHeaders() });
        if (!res.ok) { const t = await res.text(); throw new Error(t); }
        const data   = await res.json();
        this.rlRows  = data.rows  || [];
        this.rlStats = data.stats || null;
      } catch (err) {
        this.rlError = err.message || 'Load failed';
      } finally {
        this.rlLoading = false;
      }
    },

    // ── helpers ──
    rlFmt(ms) {
      if (!ms || ms <= 0) return '—';
      if (ms < 1000) return ms + 'ms';
      if (ms < 60000) return (ms / 1000).toFixed(1) + 's';
      return (ms / 60000).toFixed(1) + 'min';
    },

    rlLatClass(ms) {
      if (!ms || ms <= 0) return 'text-gray-400 dark:text-[#484f58]';
      if (ms < 2000)      return 'text-green-600 dark:text-green-400';
      if (ms < 10000)     return 'text-yellow-600 dark:text-yellow-400';
      return 'text-red-600 dark:text-red-400';
    },

    rlBar(ms, maxMs) {
      if (!ms || ms <= 0 || !maxMs) return 0;
      return Math.min(100, Math.round((ms / maxMs) * 100));
    },

    rlMaxLatency() {
      if (!this.rlRows.length) return 1;
      return Math.max(...this.rlRows.map(r => r.latency_ms || 0)) || 1;
    },
  };
}
