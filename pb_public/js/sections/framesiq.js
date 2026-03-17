// Frame Analyzers — IQ Node Analyzer and Binary Node Inspector.
function framesIQSection() {
  return {
    // ── state ──
    fiq: {
      tab:       'iq',   // 'iq' | 'binary'
      loading:   false,
      toast:     null,
      // IQ filter state
      iqLevel:   '',
      iqType:    '',
      iqFrames:  null,
      // Binary filter state
      binLevel:  '',
      binModule: '',
      binFrames: null,
      // Expanded frame
      expanded:  null,
    },

    // ── init ──
    initFramesIQ() {
      this.$watch('activeSection', val => {
        if (val === 'frames-iq' && !this.fiq.iqFrames) {
          this.loadIQFrames();
        }
      });
    },

    // ── IQ frames ──
    async loadIQFrames() {
      this.fiq.loading  = true;
      this.fiq.toast    = null;
      this.fiq.iqFrames = null;
      try {
        const params = new URLSearchParams();
        if (this.fiq.iqLevel) params.set('level', this.fiq.iqLevel);
        if (this.fiq.iqType)  params.set('iqtype', this.fiq.iqType);
        const res  = await this.zapFetch(`/zaplab/api/frames/iq?${params}`,
          { headers: { 'X-API-Token': this.apiToken } });
        const data = await res.json();
        if (res.ok) {
          this.fiq.iqFrames = data;
        } else {
          this.fiq.toast = { ok: false, message: data.error || 'Failed to load IQ frames' };
        }
      } catch (err) {
        this.fiq.toast = { ok: false, message: err.message };
      } finally {
        this.fiq.loading = false;
      }
    },

    // ── Binary frames ──
    async loadBinaryFrames() {
      this.fiq.loading   = true;
      this.fiq.toast     = null;
      this.fiq.binFrames = null;
      try {
        const params = new URLSearchParams();
        if (this.fiq.binLevel)  params.set('level', this.fiq.binLevel);
        if (this.fiq.binModule) params.set('module', this.fiq.binModule);
        const res  = await this.zapFetch(`/zaplab/api/frames/binary?${params}`,
          { headers: { 'X-API-Token': this.apiToken } });
        const data = await res.json();
        if (res.ok) {
          this.fiq.binFrames = data;
        } else {
          this.fiq.toast = { ok: false, message: data.error || 'Failed to load binary frames' };
        }
      } catch (err) {
        this.fiq.toast = { ok: false, message: err.message };
      } finally {
        this.fiq.loading = false;
      }
    },

    fiqLevelBadge(level) {
      const map = {
        debug: 'bg-gray-600 text-gray-200',
        info:  'bg-blue-700 text-blue-200',
        warn:  'bg-yellow-700 text-yellow-200',
        error: 'bg-red-700 text-red-200',
      };
      return map[(level||'').toLowerCase()] || 'bg-gray-700 text-gray-200';
    },

    fiqToggleExpand(id) {
      this.fiq.expanded = this.fiq.expanded === id ? null : id;
    },
  };
}
