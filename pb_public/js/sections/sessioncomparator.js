// Multi-session Comparator
// Loads all Signal Protocol session blobs and lets the user select up to 6
// to compare side-by-side.  Differences relative to the first selected
// session are highlighted.  No new backend endpoints required — reuses the
// existing /zaplab/api/signal/sessions and /zaplab/api/signal/senderkeys.
function sessionComparatorSection() {
  return {
    // ── state ──
    mcLoading:    false,
    mcError:      '',
    mcSessions:   [],      // all Double Ratchet sessions
    mcSenderKeys: [],      // all group SenderKey records
    mcSelected:   [],      // addresses of selected sessions (max 6)
    mcTab:        'sessions',  // 'sessions' | 'senderkeys'
    mcFilter:     '',

    // ── init ──
    initSessionComparator() {},

    // ── load ──
    async mcLoad() {
      if (this.mcLoading) return;
      this.mcLoading = true;
      this.mcError   = '';
      try {
        const [sesRes, skRes] = await Promise.all([
          fetch('/zaplab/api/signal/sessions',   { headers: this.apiHeaders() }),
          fetch('/zaplab/api/signal/senderkeys', { headers: this.apiHeaders() }),
        ]);
        if (sesRes.ok) { const d = await sesRes.json(); this.mcSessions   = d.sessions   || []; }
        else           { this.mcError = `Sessions: ${sesRes.status}`; }
        if (skRes.ok)  { const d = await skRes.json();  this.mcSenderKeys = d.sender_keys || []; }
      } catch (err) {
        this.mcError = err.message || 'Load failed';
      } finally {
        this.mcLoading = false;
      }
    },

    // ── selection ──
    mcToggle(id) {
      const idx = this.mcSelected.indexOf(id);
      if (idx >= 0) {
        this.mcSelected.splice(idx, 1);
      } else if (this.mcSelected.length < 6) {
        this.mcSelected.push(id);
      }
    },
    mcIsSelected(id) { return this.mcSelected.includes(id); },
    mcClearSelected() { this.mcSelected = []; },

    // ── filtered list ──
    mcFiltered() {
      const list = this.mcTab === 'sessions' ? this.mcSessions : this.mcSenderKeys;
      if (!this.mcFilter.trim()) return list;
      const q = this.mcFilter.trim().toLowerCase();
      return list.filter(s => {
        const key = this.mcTab === 'sessions'
          ? (s.address || '')
          : ((s.chat_id || '') + '|' + (s.sender_id || ''));
        return key.toLowerCase().includes(q);
      });
    },

    // ── comparison data ──
    mcSelectedSessions() {
      return this.mcSelected
        .map(id => this.mcSessions.find(s => s.address === id))
        .filter(Boolean);
    },

    mcSelectedSenderKeys() {
      return this.mcSelected
        .map(id => this.mcSenderKeys.find(k => (k.chat_id + '|' + k.sender_id) === id))
        .filter(Boolean);
    },

    // Property rows for session comparison table
    mcSessionRows() {
      return [
        { label: 'Version',          key: 'version',          fmt: v => `v${v}` },
        { label: 'Has Sender Chain', key: 'has_sender_chain', fmt: v => v ? '✓ Yes' : '✗ No' },
        { label: 'Sender Counter',   key: 'sender_counter',   fmt: v => String(v) },
        { label: 'Receiver Chains',  key: 'receiver_chains',  fmt: v => String(v) },
        { label: 'Previous Counter', key: 'previous_counter', fmt: v => String(v) },
        { label: 'Previous States',  key: 'previous_states',  fmt: v => String(v) },
        { label: 'Raw Size',         key: 'raw_size_bytes',   fmt: v => v + ' B' },
        { label: 'Remote Identity',  key: 'remote_identity',  fmt: v => v ? v.slice(0,16)+'…' : '—' },
        { label: 'Local Identity',   key: 'local_identity',   fmt: v => v ? v.slice(0,16)+'…' : '—' },
        { label: 'Decode Error',     key: 'decode_error',     fmt: v => v || '—' },
      ];
    },

    // Property rows for sender key comparison table
    mcSenderKeyRows() {
      return [
        { label: 'Key ID',        key: 'key_id',      fmt: v => String(v) },
        { label: 'Iteration',     key: 'iteration',   fmt: v => String(v) },
        { label: 'Raw Size',      key: 'raw_size_bytes', fmt: v => v + ' B' },
        { label: 'Signing Key',   key: 'signing_key', fmt: v => v ? v.slice(0,16)+'…' : '—' },
        { label: 'Decode Error',  key: 'decode_error', fmt: v => v || '—' },
      ];
    },

    // Is this value different from the reference (first selected)?
    mcIsDiff(items, key, idx) {
      if (idx === 0 || items.length < 2) return false;
      return String(items[idx][key]) !== String(items[0][key]);
    },

    mcDiffCount(items, rows) {
      if (items.length < 2) return 0;
      let n = 0;
      for (const row of rows) {
        for (let i = 1; i < items.length; i++) {
          if (this.mcIsDiff(items, row.key, i)) { n++; break; }
        }
      }
      return n;
    },

    // ── helpers ──
    mcHealthIcon(s) {
      if (s.decode_error)     return '✗';
      if (!s.has_sender_chain) return '⚠';
      return '✓';
    },
    mcHealthClass(s) {
      if (s.decode_error)     return 'text-red-500 dark:text-red-400';
      if (!s.has_sender_chain) return 'text-yellow-500 dark:text-yellow-400';
      return 'text-green-500 dark:text-green-400';
    },
    mcAddrShort(addr) {
      return (addr || '').replace(/\.\d+:\d+$/, '');
    },
    mcKeyShort(hex) {
      if (!hex) return '—';
      return hex.slice(0, 8) + '…' + hex.slice(-8);
    },
    mcVersionLabel(v) {
      return { 2: 'v2 (legacy)', 3: 'v3 (current)', 4: 'v4 (multi-device)' }[v] || `v${v}`;
    },
  };
}
