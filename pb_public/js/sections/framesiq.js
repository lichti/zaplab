// Frame Analyzers — IQ Node Analyzer and Binary Node Inspector.
function framesIQSection() {
  return {
    // ── state ──
    fiq: {
      tab:       'iq',   // 'iq' | 'binary'
      loading:   false,
      toast:     null,
      // IQ filter state
      iqDirection: '',
      iqType:      '',
      iqFrames:    null,
      // Binary filter state
      binDirection: '',
      binModule:    '',
      binFrames:    null,
      // Expanded frame
      expanded: null,
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
      this.fiq.loading = true;
      this.fiq.toast   = null;
      this.fiq.iqFrames = null;
      try {
        const params = new URLSearchParams();
        if (this.fiq.iqDirection) params.set('direction', this.fiq.iqDirection);
        if (this.fiq.iqType)      params.set('iqtype', this.fiq.iqType);
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
      this.fiq.loading = true;
      this.fiq.toast   = null;
      this.fiq.binFrames = null;
      try {
        const params = new URLSearchParams();
        if (this.fiq.binDirection) params.set('direction', this.fiq.binDirection);
        if (this.fiq.binModule)    params.set('module', this.fiq.binModule);
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

    fiqDirectionBadge(dir) {
      return dir === 'in'
        ? 'bg-blue-700 text-blue-200'
        : 'bg-orange-700 text-orange-200';
    },

    fiqToggleExpand(id) {
      this.fiq.expanded = this.fiq.expanded === id ? null : id;
    },
  };
}
