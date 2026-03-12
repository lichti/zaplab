// Pairing section — WhatsApp connection status, QR code display, logout.
function pairingSection() {
  return {
    // ── state ──
    wa: {
      status:  'unknown',   // connecting | connected | qr | timeout | disconnected | loggedout | unknown
      jid:     '',
      qrImage: '',
      loading: false,
      toast:   null,
    },
    _waPollInterval: null,

    // ── section init ──
    initPairing() {
      this.fetchWAStatus();
      this._waPollInterval = setInterval(() => this.fetchWAStatus(), 3000);
    },

    // ── methods ──
    async fetchWAStatus() {
      try {
        const res  = await this.zapFetch('/zaplab/api/wa/status');
        const data = await res.json();
        const prev = this.wa.status;
        this.wa.status = data.status || 'unknown';
        this.wa.jid    = data.jid    || '';

        if (this.wa.status === 'qr') {
          await this.fetchWAQR();
        } else {
          this.wa.qrImage = '';
        }

        // Refresh account info once after connecting
        if (prev !== 'connected' && this.wa.status === 'connected') {
          this.account.jid = '';  // reset so fetchAccount re-fetches
          this.fetchAccount();
        }

        // Auto-navigate to pairing section on first detection of a problem
        if (['qr', 'timeout', 'disconnected'].includes(this.wa.status) && this.activeSection !== 'pairing') {
          if (prev === 'unknown' || prev === 'connected') {
            this.setSection('pairing');
          }
        }
      } catch { /* ignore transient network errors */ }
    },

    async fetchWAQR() {
      try {
        const res = await this.zapFetch(`/zaplab/api/wa/qrcode?t=${Date.now()}`);
        if (!res.ok) { this.wa.qrImage = ''; return; }
        const data = await res.json();
        this.wa.qrImage = data.image || '';
      } catch { this.wa.qrImage = ''; }
    },

    async waLogout() {
      this.wa.toast   = null;
      this.wa.loading = true;
      try {
        const res  = await this.zapFetch('/zaplab/api/wa/logout', {
          method:  'POST',
          headers: { 'X-API-Token': this.apiToken },
        });
        const data = await res.json();
        this.wa.toast = { ok: res.ok, message: data.message || JSON.stringify(data) };
      } catch (err) {
        this.wa.toast = { ok: false, message: err.message };
      } finally {
        this.wa.loading = false;
      }
    },
  };
}
