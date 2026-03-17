// Protocol Timeline section — chronological timeline of WhatsApp protocol events
// with color-coded event types, real-time updates, and expandable protocol detail.
function protocolTimelineSection() {
  return {
    // ── state ──
    ptEvents:        [],
    ptSelected:      null,
    ptFilter:        '',
    ptTypeFilter:    '',
    ptPaused:        false,
    ptMaxItems:      200,
    ptConnStatus:    'connecting',
    ptCopied:        false,
    ptLoading:       false,
    ptSubscription:  null,

    // ── event type config ──
    ptEventTypes: {
      'Message':              { color: 'blue',   label: 'Message',        icon: 'M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z' },
      'Receipt':              { color: 'green',  label: 'Receipt',        icon: 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z' },
      'Presence':             { color: 'yellow', label: 'Presence',       icon: 'M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z' },
      'HistorySync':          { color: 'purple', label: 'History Sync',   icon: 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z' },
      'AppStateSyncComplete': { color: 'indigo', label: 'App State Sync', icon: 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15' },
      'Connected':            { color: 'green',  label: 'Connected',      icon: 'M13 10V3L4 14h7v7l9-11h-7z' },
      'Disconnected':         { color: 'red',    label: 'Disconnected',   icon: 'M18.364 5.636l-3.536 3.536m0 5.656l3.536 3.536M9.172 9.172L5.636 5.636m3.536 9.9l-3.536 3.536M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-5 0a4 4 0 11-8 0 4 4 0 018 0z' },
      'LoggedOut':            { color: 'red',    label: 'Logged Out',     icon: 'M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1' },
      'QR':                   { color: 'orange', label: 'QR Code',        icon: 'M12 4v1m6 11h2m-6 0h-2v4m0-11v3m0 0h.01M12 12h4.01M16 20h4M4 12h4m12 0h.01M5 8h2a1 1 0 001-1V5a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1zm12 0h2a1 1 0 001-1V5a1 1 0 00-1-1h-2a1 1 0 00-1 1v2a1 1 0 001 1zM5 20h2a1 1 0 001-1v-2a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1z' },
      'PairSuccess':          { color: 'green',  label: 'Pair Success',   icon: 'M9 12l2 2 4-4M7.835 4.697a3.42 3.42 0 001.946-.806 3.42 3.42 0 014.438 0 3.42 3.42 0 001.946.806 3.42 3.42 0 013.138 3.138 3.42 3.42 0 00.806 1.946 3.42 3.42 0 010 4.438 3.42 3.42 0 00-.806 1.946 3.42 3.42 0 01-3.138 3.138 3.42 3.42 0 00-1.946.806 3.42 3.42 0 01-4.438 0 3.42 3.42 0 00-1.946-.806 3.42 3.42 0 01-3.138-3.138 3.42 3.42 0 00-.806-1.946 3.42 3.42 0 010-4.438 3.42 3.42 0 00.806-1.946 3.42 3.42 0 013.138-3.138z' },
      'StreamReplaced':       { color: 'orange', label: 'Stream Replaced','icon': 'M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4' },
      'KeepAliveTimeout':     { color: 'yellow', label: 'Keepalive',      icon: 'M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z' },
      'CallOffer':            { color: 'pink',   label: 'Call Offer',     icon: 'M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z' },
      'GroupInfo':            { color: 'teal',   label: 'Group Info',     icon: 'M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z' },
      'Contact':              { color: 'cyan',   label: 'Contact',        icon: 'M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z' },
      'NewsletterMessage':    { color: 'violet', label: 'Newsletter',     icon: 'M19 20H5a2 2 0 01-2-2V6a2 2 0 012-2h10a2 2 0 012 2v1m2 13a2 2 0 01-2-2V7m2 13a2 2 0 002-2V9a2 2 0 00-2-2h-2m-4-3H9M7 16h6M7 8h6v4H7V8z' },
    },

    ptDefaultType: { color: 'gray', label: 'Event', icon: 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z' },

    // ── init ──
    initProtocolTimeline() {
      // nothing to do on init — data loaded when section becomes active
    },

    async ptLoad() {
      if (this.ptLoading || this.ptEvents.length > 0) return;
      this.ptLoading = true;
      try {
        const res = await pb.collection('events').getList(1, this.ptMaxItems, {
          sort: '-created',
          requestKey: null,
        });
        this.ptEvents = res.items.map(r => ({ ...r, _isNew: false }));
      } catch (err) {
        console.error('Protocol Timeline: failed to load events', err);
      } finally {
        this.ptLoading = false;
      }
    },

    ptSubscribe() {
      if (this.ptSubscription) return; // already subscribed
      this.ptConnStatus = 'connecting';
      this.ptSubscription = true; // mark as in-progress to prevent re-entry
      pb.collection('events').subscribe('*', e => {
        if (e.action === 'create' && !this.ptPaused) {
          const ev = { ...e.record, _isNew: true };
          this.ptEvents.unshift(ev);
          if (this.ptEvents.length > this.ptMaxItems) {
            this.ptEvents = this.ptEvents.slice(0, this.ptMaxItems);
          }
          setTimeout(() => {
            const found = this.ptEvents.find(x => x.id === e.record.id);
            if (found) found._isNew = false;
          }, 2000);
        }
      }).then(() => {
        this.ptConnStatus = 'connected';
      }).catch(() => {
        this.ptConnStatus = 'disconnected';
        setTimeout(() => this.ptSubscribe(), 3000);
      });
    },

    // ── computed helpers ──
    ptFilteredEvents() {
      let evs = this.ptEvents;
      if (this.ptTypeFilter) {
        evs = evs.filter(e => e.type === this.ptTypeFilter);
      }
      if (this.ptFilter.trim()) {
        const q = this.ptFilter.trim().toLowerCase();
        evs = evs.filter(e =>
          (e.type || '').toLowerCase().includes(q) ||
          JSON.stringify(e.data || {}).toLowerCase().includes(q)
        );
      }
      return evs;
    },

    ptDistinctTypes() {
      const seen = new Set();
      this.ptEvents.forEach(e => { if (e.type) seen.add(e.type); });
      return Array.from(seen).sort();
    },

    ptTypeConfig(typeName) {
      return this.ptEventTypes[typeName] || this.ptDefaultType;
    },

    ptColorClasses(color) {
      const map = {
        blue:   { dot: 'bg-blue-500',   badge: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',   line: 'border-blue-200 dark:border-blue-900/50'   },
        green:  { dot: 'bg-green-500',  badge: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400', line: 'border-green-200 dark:border-green-900/50' },
        yellow: { dot: 'bg-yellow-500', badge: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400', line: 'border-yellow-200 dark:border-yellow-900/50' },
        red:    { dot: 'bg-red-500',    badge: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',       line: 'border-red-200 dark:border-red-900/50'     },
        orange: { dot: 'bg-orange-500', badge: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400', line: 'border-orange-200 dark:border-orange-900/50' },
        purple: { dot: 'bg-purple-500', badge: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400', line: 'border-purple-200 dark:border-purple-900/50' },
        indigo: { dot: 'bg-indigo-500', badge: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400', line: 'border-indigo-200 dark:border-indigo-900/50' },
        pink:   { dot: 'bg-pink-500',   badge: 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400',   line: 'border-pink-200 dark:border-pink-900/50'   },
        teal:   { dot: 'bg-teal-500',   badge: 'bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-400',   line: 'border-teal-200 dark:border-teal-900/50'   },
        cyan:   { dot: 'bg-cyan-500',   badge: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400',   line: 'border-cyan-200 dark:border-cyan-900/50'   },
        violet: { dot: 'bg-violet-500', badge: 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400', line: 'border-violet-200 dark:border-violet-900/50' },
        gray:   { dot: 'bg-gray-500',   badge: 'bg-gray-100 text-gray-700 dark:bg-gray-700/30 dark:text-gray-400',   line: 'border-gray-200 dark:border-gray-700/50'   },
      };
      return map[color] || map.gray;
    },

    ptFormatTime(ts) {
      if (!ts) return '';
      const d = new Date(ts);
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', fractionalSecondDigits: 3 });
    },

    ptFormatDate(ts) {
      if (!ts) return '';
      return new Date(ts).toLocaleDateString([], { month: 'short', day: 'numeric' });
    },

    ptSelectEvent(ev) {
      this.ptSelected = this.ptSelected?.id === ev.id ? null : ev;
      this.ptCopied = false;
    },

    ptClearEvents() {
      this.ptEvents = [];
      this.ptSelected = null;
    },

    async ptCopyJSON() {
      if (!this.ptSelected) return;
      try {
        const { _isNew, ...clean } = this.ptSelected;
        await navigator.clipboard.writeText(JSON.stringify(clean, null, 2));
        this.ptCopied = true;
        setTimeout(() => { this.ptCopied = false; }, 2000);
      } catch {}
    },

    ptSummary(ev) {
      const data = ev.data || {};
      switch (ev.type) {
        case 'Message': {
          const info = data.Info || {};
          const from = info.Sender || info.Chat || '';
          const body = data.Message?.conversation || data.Message?.extendedTextMessage?.text || '';
          return (from ? `from ${from}` : '') + (body ? ` — "${body.slice(0, 60)}"` : '');
        }
        case 'Receipt': {
          const src = data.SourceString || '';
          const type = data.Type || '';
          return `${type} from ${src}`;
        }
        case 'Presence': {
          return `${data.From || ''} → ${data.State || ''}`;
        }
        case 'HistorySync': {
          const cnt = data.Data?.conversations?.length || 0;
          return `${cnt} conversation(s), type: ${data.Data?.syncType || '?'}`;
        }
        case 'Connected':
          return 'WebSocket connected';
        case 'Disconnected':
          return data.Err || 'stream closed';
        case 'LoggedOut':
          return data.OnConnect ? 'on reconnect' : 'during session';
        case 'CallOffer':
          return `from ${data.From || '?'}`;
        case 'GroupInfo':
          return `${data.JID || ''} — ${data.Type || ''}`;
        default:
          return '';
      }
    },
  };
}
