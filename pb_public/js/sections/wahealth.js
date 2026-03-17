// WA Health — pre-key health monitor and message secret inspector.
function waHealthSection() {
  return {
    // ── state ──
    wah: {
      tab:     'prekeys',  // 'prekeys' | 'secrets'
      loading: false,
      toast:   null,
      prekeys: null,
      secrets: null,
      secretsFilter: '',
      secretsOffset: 0,
      secretsLimit:  100,
    },

    // ── init ──
    initWAHealth() {
      this.$watch('activeSection', val => {
        if (val === 'wa-health' && !this.wah.prekeys) {
          this.loadWAPrekeys();
        }
      });
      this.$watch('wah.tab', val => {
        if (val === 'prekeys' && !this.wah.prekeys) this.loadWAPrekeys();
        if (val === 'secrets' && !this.wah.secrets)  this.loadWASecrets();
      });
    },

    // ── prekeys ──
    async loadWAPrekeys() {
      this.wah.loading = true;
      this.wah.toast   = null;
      try {
        const res  = await this.zapFetch('/zaplab/api/wa/prekeys',
          { headers: { 'X-API-Token': this.apiToken } });
        const data = await res.json();
        if (res.ok) {
          this.wah.prekeys = data;
        } else {
          this.wah.toast = { ok: false, message: data.error || 'Failed to load pre-keys' };
        }
      } catch (err) {
        this.wah.toast = { ok: false, message: err.message };
      } finally {
        this.wah.loading = false;
      }
    },

    // ── secrets ──
    async loadWASecrets() {
      this.wah.loading = true;
      this.wah.toast   = null;
      try {
        const jid = this.wah.secretsFilter ? `&jid=${encodeURIComponent(this.wah.secretsFilter)}` : '';
        const url = `/zaplab/api/wa/secrets?limit=${this.wah.secretsLimit}&offset=${this.wah.secretsOffset}${jid}`;
        const res  = await this.zapFetch(url,
          { headers: { 'X-API-Token': this.apiToken } });
        const data = await res.json();
        if (res.ok) {
          this.wah.secrets = data;
        } else {
          this.wah.toast = { ok: false, message: data.error || 'Failed to load secrets' };
        }
      } catch (err) {
        this.wah.toast = { ok: false, message: err.message };
      } finally {
        this.wah.loading = false;
      }
    },

    wahPrekeyBar(uploaded, total) {
      if (!total) return 0;
      return Math.round((uploaded / total) * 100);
    },
  };
}
