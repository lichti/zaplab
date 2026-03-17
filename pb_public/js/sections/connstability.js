// Connection Stability Dashboard — tracks connected/disconnected events over time.
function connStabilitySection() {
  return {
    // ── state ──
    conn: {
      loading: false,
      toast:   null,
      events:  null,
      stats:   null,
      days:    7,
    },
    connTab: 'timeline',

    // ── init ──
    initConnStability() {
      this.$watch('activeSection', val => {
        if (val === 'conn-stability' && !this.conn.events) {
          this.loadConnData();
        }
      });
    },

    // ── load ──
    async loadConnData() {
      this.conn.loading = true;
      this.conn.toast   = null;
      try {
        const [evtRes, statRes] = await Promise.all([
          this.zapFetch(`/zaplab/api/conn/events?days=${this.conn.days}&limit=500`,
            { headers: { 'X-API-Token': this.apiToken } }),
          this.zapFetch(`/zaplab/api/conn/stats?days=${this.conn.days}`,
            { headers: { 'X-API-Token': this.apiToken } }),
        ]);
        const evtData  = await evtRes.json();
        const statData = await statRes.json();
        if (evtRes.ok)  this.conn.events = evtData;
        if (statRes.ok) this.conn.stats  = statData;
        if (!evtRes.ok)  this.conn.toast = { ok: false, message: evtData.error || 'Failed to load events' };
      } catch (err) {
        this.conn.toast = { ok: false, message: err.message };
      } finally {
        this.conn.loading = false;
      }
    },

    connEventBadge(type) {
      const map = {
        connected:    'bg-green-700 text-green-200',
        disconnected: 'bg-red-700 text-red-200',
        reconnected:  'bg-yellow-700 text-yellow-200',
      };
      return map[type] || 'bg-gray-700 text-gray-200';
    },

    connStatCount(type) {
      if (!this.conn.stats) return 0;
      const entry = (this.conn.stats.by_type || []).find(e => e.event_type === type);
      return entry ? entry.count : 0;
    },

    connUptimePct() {
      if (!this.conn.stats) return '—';
      const connected    = this.connStatCount('connected');
      const disconnected = this.connStatCount('disconnected');
      const total = connected + disconnected;
      if (total === 0) return '100%';
      return ((connected / total) * 100).toFixed(1) + '%';
    },
  };
}
