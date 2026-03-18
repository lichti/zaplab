// Contact Overview — rich analytics profile for a single contact JID.
// Prefix: co
function contactOverviewSection() {
  return {
    // ── state ──
    coLoading:      false,
    coError:        '',
    coPeriod:       30,
    coPeriodOpts:   [7, 30, 90, 365, 0],
    coJID:          '',
    coData:         null,

    // profile (from /contacts/{jid} — live, may be offline)
    coProfile:      null,
    coProfileLoading: false,

    // contact list sidebar
    coListLoading:  false,
    coListLoaded:   false,
    coListError:    '',
    coContacts:     [],
    coSearch:       '',

    // ui
    coActiveTab:    'overview', // overview | presence | groups

    // ── init ──
    initContactOverview() {
      this.$watch(() => Alpine.store('nav').coJID, jid => {
        if (jid) { this.coSelectContact(jid); Alpine.store('nav').coJID = ''; }
      });
    },

    // ── contact list ─────────────────────────────────────────────────────────
    async coLoadContacts() {
      if (this.coListLoading) return;
      this.coListLoading = true;
      this.coListError   = '';
      try {
        const [chatsRes, namesRes] = await Promise.all([
          fetch('/zaplab/api/conversation/chats?limit=500', { headers: apiHeaders() }),
          fetch('/zaplab/api/conversation/names',           { headers: apiHeaders() }),
        ]);
        const chatsData = chatsRes.ok ? await chatsRes.json() : {};
        const namesData = namesRes.ok ? await namesRes.json() : {};
        const namesMap  = namesData.names || {};
        const chats     = chatsData.chats || [];
        this.coContacts = chats
          .filter(c => c.chat && !c.chat.endsWith('@g.us') && !c.chat.endsWith('@broadcast'))
          .map(c => ({ jid: c.chat, name: namesMap[c.chat] || c.chat.split('@')[0] }))
          .sort((a, b) => a.name.localeCompare(b.name));
      } catch (err) {
        this.coListError = err.message || 'Load failed';
      } finally {
        this.coListLoading = false;
        this.coListLoaded  = true;
      }
    },

    coFilteredContacts() {
      const q = (this.coSearch || '').toLowerCase();
      if (!q) return this.coContacts;
      return this.coContacts.filter(c =>
        c.jid.toLowerCase().includes(q) || (c.name || '').toLowerCase().includes(q)
      );
    },

    coSelectContact(jid) {
      this.coJID       = jid;
      this.coData      = null;
      this.coProfile   = null;
      this.coActiveTab = 'overview';
      this.coLoad();
      this.coLoadProfile();
    },

    // ── overview load ─────────────────────────────────────────────────────────
    async coLoad() {
      if (!this.coJID || this.coLoading) return;
      this.coLoading = true;
      this.coError   = '';
      try {
        const jid = encodeURIComponent(this.coJID);
        const res = await fetch(
          `/zaplab/api/contacts/${jid}/overview?period=${this.coPeriod}`,
          { headers: apiHeaders() }
        );
        if (!res.ok) {
          const d = await res.json().catch(() => ({}));
          this.coError = d.error || `HTTP ${res.status}`;
          return;
        }
        this.coData = await res.json();
      } catch (err) {
        this.coError = err.message || 'Load failed';
      } finally {
        this.coLoading = false;
      }
    },

    // ── profile photo + status (live) ─────────────────────────────────────────
    async coLoadProfile() {
      if (!this.coJID) return;
      this.coProfileLoading = true;
      try {
        const jid = encodeURIComponent(this.coJID);
        const res = await fetch(`/zaplab/api/contacts/${jid}`, { headers: apiHeaders() });
        if (res.ok) this.coProfile = await res.json();
      } catch (_) {}
      this.coProfileLoading = false;
    },

    // ── heatmap ───────────────────────────────────────────────────────────────
    coHeatValue(dow, hour) {
      if (!this.coData) return 0;
      const cell = (this.coData.heatmap || []).find(c => c.dow === dow && c.hour === hour);
      return cell ? cell.count : 0;
    },
    coHeatMax() {
      if (!this.coData) return 1;
      return Math.max(1, ...(this.coData.heatmap || []).map(c => c.count));
    },
    coHeatColor(dow, hour) {
      const v = this.coHeatValue(dow, hour);
      const pct = v / this.coHeatMax();
      return `rgba(88,166,255,${(0.07 + pct * 0.88).toFixed(2)})`;
    },
    coHeatDays:  ['Sun','Mon','Tue','Wed','Thu','Fri','Sat'],
    coHeatHours: ['00','01','02','03','04','05','06','07','08','09','10','11',
                  '12','13','14','15','16','17','18','19','20','21','22','23'],

    // ── sparkline SVG ─────────────────────────────────────────────────────────
    coSparklinePath() {
      const pts = (this.coData && this.coData.daily) || [];
      if (pts.length < 2) return '';
      const W = 400, H = 56;
      const maxV = Math.max(1, ...pts.map(p => p.count));
      return pts.map((p, i) => {
        const x = (i / (pts.length - 1)) * W;
        const y = H - (p.count / maxV) * H;
        return (i === 0 ? 'M' : 'L') + x.toFixed(1) + ',' + y.toFixed(1);
      }).join(' ');
    },
    coSparklineArea() {
      const pts = (this.coData && this.coData.daily) || [];
      if (pts.length < 2) return '';
      const W = 400, H = 56;
      const maxV = Math.max(1, ...pts.map(p => p.count));
      const line = pts.map((p, i) => {
        const x = (i / (pts.length - 1)) * W;
        const y = H - (p.count / maxV) * H;
        return x.toFixed(1) + ',' + y.toFixed(1);
      }).join(' L');
      return `M${line} L${W},${H} L0,${H} Z`;
    },

    // ── presence helpers ──────────────────────────────────────────────────────
    coPresenceIcon(type) {
      if (type === 'Presence.Online' || type === 'ChatPresence.Composing' || type === 'ChatPresence.Recording')
        return '🟢';
      return '⚫';
    },
    coPresenceLabel(type) {
      return {
        'Presence.Online':          'Online',
        'Presence.Offline':         'Offline',
        'Presence.OfflineLastSeen': 'Last seen',
        'ChatPresence.Composing':   'Typing…',
        'ChatPresence.Recording':   'Recording…',
        'ChatPresence.Paused':      'Paused',
      }[type] || type;
    },
    coPresenceColor(type) {
      if (type === 'Presence.Online') return 'text-green-500';
      if (type.includes('Chat')) return 'text-blue-400';
      return 'text-gray-400 dark:text-[#484f58]';
    },

    // ── formatting ────────────────────────────────────────────────────────────
    coPeriodLabel: p => p === 0 ? 'All time' : `${p}d`,
    coFmt: n => (n || 0).toLocaleString(),
    coTimeAgo(ts) {
      if (!ts) return '—';
      const diff = Date.now() - new Date(ts).getTime();
      const m = Math.floor(diff / 60000);
      if (m < 1)   return 'just now';
      if (m < 60)  return `${m}m ago`;
      const h = Math.floor(m / 60);
      if (h < 24)  return `${h}h ago`;
      const d = Math.floor(h / 24);
      if (d < 30)  return `${d}d ago`;
      return new Date(ts).toLocaleDateString();
    },
    coFmtDate(ts) {
      if (!ts) return '—';
      return new Date(ts).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
    },

    // ── navigation ────────────────────────────────────────────────────────────
    coOpenConversation() {
      if (!this.coJID) return;
      Alpine.store('nav').cvSelectedChat = this.coJID;
      setSection('conversation');
    },
    coOpenSearch() {
      if (!this.coJID) return;
      Alpine.store('nav').srQuery = '';
      Alpine.store('nav').srChat  = this.coJID;
      setSection('search');
    },
    coOpenNetworkGraph() {
      if (!this.coJID) return;
      setSection('networkgraph');
    },
  };
}
