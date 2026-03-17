// Frame Capture section — real-time whatsmeow log stream browser
// showing all captured log entries from the in-memory ring buffer
// and the persistent PocketBase `frames` collection (INFO+).
function frameCaptureSection() {
  return {
    // ── state ──
    fcEntries:       [],
    fcSelected:      null,
    fcModuleFilter:  '',
    fcLevelFilter:   '',
    fcSearch:        '',
    fcPaused:        false,
    fcLiveMode:      true,   // true = ring buffer (all levels); false = DB (INFO+)
    fcLoading:       false,
    fcConnStatus:    'disconnected',
    fcSubscription:  null,
    fcTotalDB:       0,
    fcPageDB:        1,
    fcModules:       [],

    // ── level config ──
    fcLevelConfig: {
      DEBUG: { color: 'gray',   bg: 'bg-gray-100 text-gray-600 dark:bg-gray-700/40 dark:text-gray-400'   },
      INFO:  { color: 'blue',   bg: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'   },
      WARN:  { color: 'yellow', bg: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400' },
      ERROR: { color: 'red',    bg: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'       },
    },

    // ── init ──
    initFrameCapture() {},

    async fcLoad() {
      if (this.fcLoading) return;
      this.fcLoading = true;
      try {
        if (this.fcLiveMode) {
          await this.fcLoadRing();
        } else {
          await this.fcLoadDB();
        }
      } finally {
        this.fcLoading = false;
      }
    },

    async fcLoadRing() {
      const params = new URLSearchParams({ limit: 500 });
      if (this.fcModuleFilter) params.set('module', this.fcModuleFilter);
      if (this.fcLevelFilter)  params.set('level',  this.fcLevelFilter);
      const res = await fetch(`/zaplab/api/frames/ring?${params}`, { headers: this.apiHeaders() });
      if (!res.ok) return;
      const data = await res.json();
      this.fcEntries = (data.entries || []).reverse(); // newest first
    },

    async fcLoadDB() {
      const params = new URLSearchParams({ per_page: 100, page: this.fcPageDB });
      if (this.fcModuleFilter) params.set('module', this.fcModuleFilter);
      if (this.fcLevelFilter)  params.set('level',  this.fcLevelFilter);
      if (this.fcSearch)       params.set('search', this.fcSearch);
      const res = await fetch(`/zaplab/api/frames?${params}`, { headers: this.apiHeaders() });
      if (!res.ok) return;
      const data = await res.json();
      this.fcEntries = data.items || [];
      this.fcTotalDB = data.total || 0;
    },

    async fcLoadModules() {
      try {
        const res = await fetch('/zaplab/api/frames/modules', { headers: this.apiHeaders() });
        if (res.ok) {
          const data = await res.json();
          this.fcModules = (data.modules || []).sort();
        }
      } catch {}
    },

    fcSubscribe() {
      if (this.fcSubscription) return;
      this.fcConnStatus = 'connecting';
      this.fcSubscription = true;
      pb.collection('frames').subscribe('*', evt => {
        if (evt.action === 'create' && !this.fcPaused) {
          this.fcEntries.unshift(evt.record);
          if (this.fcEntries.length > 1000) this.fcEntries.pop();
        }
      }).then(() => {
        this.fcConnStatus = 'connected';
      }).catch(() => {
        this.fcConnStatus = 'disconnected';
        this.fcSubscription = null;
        setTimeout(() => this.fcSubscribe(), 3000);
      });
    },

    async fcRefresh() {
      this.fcEntries = [];
      this.fcPageDB = 1;
      await this.fcLoad();
      if (this.fcLiveMode) await this.fcLoadModules();
    },

    fcSwitchMode(live) {
      if (this.fcLiveMode === live) return;
      this.fcLiveMode = live;
      this.fcEntries = [];
      this.fcPageDB = 1;
      this.fcLoad();
    },

    // ── filtered entries (ring mode: client-side; DB mode: server-side) ──
    fcFiltered() {
      if (!this.fcLiveMode) return this.fcEntries;
      let items = this.fcEntries;
      if (this.fcModuleFilter) items = items.filter(e => (e.module || e.Module) === this.fcModuleFilter);
      if (this.fcLevelFilter)  items = items.filter(e => (e.level  || e.Level)  === this.fcLevelFilter);
      if (this.fcSearch.trim()) {
        const q = this.fcSearch.trim().toLowerCase();
        items = items.filter(e => ((e.msg || e.Message || '') + (e.module || e.Module || '')).toLowerCase().includes(q));
      }
      return items;
    },

    // ── PCAP export ──
    async fcExportPCAP() {
      const params = new URLSearchParams({ limit: 1000 });
      if (this.fcModuleFilter) params.set('module', this.fcModuleFilter);
      if (this.fcLevelFilter)  params.set('level',  this.fcLevelFilter);
      if (this.fcSearch.trim()) params.set('search', this.fcSearch.trim());
      try {
        const res = await fetch(`/zaplab/api/frames/pcap?${params}`, { headers: this.apiHeaders() });
        if (!res.ok) return;
        const blob = await res.blob();
        const url  = URL.createObjectURL(blob);
        const a    = document.createElement('a');
        a.href     = url;
        a.download = `zaplab_frames_${new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)}.pcap`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
      } catch (_) {}
    },

    // ── helpers ──
    fcMsg(e) { return e.msg || e.Message || ''; },
    fcModule(e) { return e.module || e.Module || ''; },
    fcLevel(e) { return e.level || e.Level || 'DEBUG'; },
    fcTime(e) {
      const ts = e.created || e.time || e.Time;
      if (!ts) return '';
      return new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', fractionalSecondDigits: 3 });
    },
    fcLevelBadge(e) {
      return (this.fcLevelConfig[this.fcLevel(e)] || this.fcLevelConfig.DEBUG).bg;
    },
    fcModuleShort(e) {
      const m = this.fcModule(e);
      // "Client/Socket" → "Socket", "Database" → "DB"
      const parts = m.split('/');
      return parts[parts.length - 1] || m;
    },
  };
}
