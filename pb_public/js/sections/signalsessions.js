// Signal Session Visualizer — shows decoded Double Ratchet session states
// and group SenderKey states from the whatsmeow SQLite database.
function signalSessionsSection() {
  return {
    // ── state ──
    ssLoading:       false,
    ssError:         '',
    ssSessions:      [],
    ssSenderKeys:    [],
    ssActiveTab:     'sessions',   // 'sessions' | 'senderkeys'
    ssSelected:      null,
    ssFilter:        '',

    // ── init ──
    initSignalSessions() {},

    async ssLoad() {
      if (this.ssLoading) return;
      this.ssLoading = true;
      this.ssError = '';
      try {
        const [sessRes, skRes] = await Promise.all([
          fetch('/zaplab/api/signal/sessions',   { headers: apiHeaders() }),
          fetch('/zaplab/api/signal/senderkeys', { headers: apiHeaders() }),
        ]);
        if (sessRes.ok) {
          const d = await sessRes.json();
          this.ssSessions = d.sessions || [];
        } else {
          this.ssError = `Sessions: ${sessRes.status}`;
        }
        if (skRes.ok) {
          const d = await skRes.json();
          this.ssSenderKeys = d.sender_keys || [];
        }
      } catch (err) {
        this.ssError = err.message || 'Load failed';
      } finally {
        this.ssLoading = false;
      }
    },

    async ssRefresh() {
      this.ssSessions = [];
      this.ssSenderKeys = [];
      this.ssSelected = null;
      await this.ssLoad();
    },

    // ── filtered lists ──
    ssFilteredSessions() {
      if (!this.ssFilter.trim()) return this.ssSessions;
      const q = this.ssFilter.trim().toLowerCase();
      return this.ssSessions.filter(s => s.address.toLowerCase().includes(q));
    },

    ssFilteredSenderKeys() {
      if (!this.ssFilter.trim()) return this.ssSenderKeys;
      const q = this.ssFilter.trim().toLowerCase();
      return this.ssSenderKeys.filter(k =>
        k.chat_id.toLowerCase().includes(q) || k.sender_id.toLowerCase().includes(q));
    },

    // ── helpers ──
    ssVersionLabel(v) {
      const map = { 2: 'v2 (legacy)', 3: 'v3 (current)', 4: 'v4 (multi-device)' };
      return map[v] || `v${v}`;
    },

    ssAddressShort(addr) {
      // "15551234567.0:1" — strip device suffix for display
      return addr.replace(/\.\d+$/, '');
    },

    ssKeyShort(hex) {
      if (!hex) return '—';
      return hex.slice(0, 8) + '…' + hex.slice(-8);
    },

    ssSessionHealthColor(s) {
      if (s.decode_error) return 'text-red-500';
      if (!s.has_sender_chain) return 'text-yellow-500';
      return 'text-green-500';
    },

    ssSessionHealthIcon(s) {
      if (s.decode_error) return '✗';
      if (!s.has_sender_chain) return '⚠';
      return '✓';
    },

    ssSenderKeyHealthColor(k) {
      if (k.decode_error) return 'text-red-500';
      if (k.iteration === 0) return 'text-yellow-500';
      return 'text-green-500';
    },
  };
}
