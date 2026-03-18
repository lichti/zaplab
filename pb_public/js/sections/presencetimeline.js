// Presence Timeline — visualises contact online/offline/typing events over time.
function presenceTimelineSection() {
  return {
    // ── state ──
    ptLoading:  false,
    ptError:    '',
    ptJid:      '',
    ptDays:     7,
    ptRows:     [],
    ptSummary:  [],
    ptTotal:    0,
    ptSelected: null,

    // ── init ──
    initPresenceTimeline() {},

    // ── data ──
    async ptLoad() {
      if (this.ptLoading) return;
      this.ptLoading = true;
      this.ptError   = '';
      try {
        const p = new URLSearchParams({ days: this.ptDays, limit: 500 });
        if (this.ptJid.trim()) p.set('jid', this.ptJid.trim());
        const res = await fetch(`/zaplab/api/presence/timeline?${p}`, { headers: apiHeaders() });
        if (!res.ok) { const t = await res.text(); throw new Error(t); }
        const data    = await res.json();
        this.ptRows    = data.events  || [];
        this.ptSummary = data.summary || [];
        this.ptTotal   = data.total   || 0;
      } catch (err) {
        this.ptError = err.message || 'Load failed';
      } finally {
        this.ptLoading = false;
      }
    },

    // ── helpers ──
    ptTypeClass(type) {
      if (type === 'Presence.Online')           return 'bg-green-100 text-green-800 dark:bg-green-900/20 dark:text-green-400';
      if (type.startsWith('Presence.Offline'))  return 'bg-red-100 text-red-800 dark:bg-red-900/20 dark:text-red-400';
      if (type.startsWith('ChatPresence.comp')) return 'bg-blue-100 text-blue-800 dark:bg-blue-900/20 dark:text-blue-400';
      return 'bg-gray-100 text-gray-700 dark:bg-[#21262d] dark:text-[#8b949e]';
    },

    ptTypeLabel(type) {
      if (type === 'Presence.Online')                return 'Online';
      if (type === 'Presence.Offline')               return 'Offline';
      if (type === 'Presence.OfflineLastSeen')       return 'Last Seen';
      if (type === 'ChatPresence.composing')         return 'Typing';
      if (type === 'ChatPresence.composing.audio')   return 'Recording';
      if (type === 'ChatPresence.paused')            return 'Paused';
      return type;
    },

    ptFromJid(raw) {
      try { const o = JSON.parse(raw); return o.From || o.Chat || ''; } catch { return ''; }
    },

    ptLastSeen(raw) {
      try {
        const o = JSON.parse(raw);
        if (o.LastSeen && o.LastSeen !== '0001-01-01T00:00:00Z') return new Date(o.LastSeen).toLocaleString();
        return '';
      } catch { return ''; }
    },
  };
}
