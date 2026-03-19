// Group Overview — rich analytics dashboard for a single group JID.
// Prefix: gov
function groupOverviewSection() {
  return {
    // ── state ──
    govLoading:      false,
    govError:        '',
    govPeriod:       30,
    govPeriodOpts:   [7, 30, 90, 365, 0],
    govJID:          '',
    govData:         null,

    // profile photo (live)
    govProfile:         null,
    govProfileLoading:  false,

    // group list sidebar
    govListLoading:  false,
    govListLoaded:   false,
    govListError:    '',
    govGroups:       [],
    govSearch:       '',

    // ui
    govActiveTab:    'overview', // overview | members | silent | evolution
    govMemberSort:   'messages', // 'name' | 'messages'
    govMemberSortDir: 'desc',    // 'asc' | 'desc'

    // ── init ──
    initGroupOverview() {},

    // ── group list ────────────────────────────────────────────────────────────
    async govLoadGroups() {
      if (this.govListLoading) return;
      this.govListLoading = true;
      this.govListError   = '';
      try {
        const [chatsRes, namesRes] = await Promise.all([
          fetch('/zaplab/api/conversation/chats?limit=500', { headers: apiHeaders() }),
          fetch('/zaplab/api/conversation/names',           { headers: apiHeaders() }),
        ]);
        const chatsData = chatsRes.ok ? await chatsRes.json() : {};
        const namesData = namesRes.ok ? await namesRes.json() : {};
        const namesMap  = namesData.names || {};
        const chats     = chatsData.chats || [];
        this.govGroups = chats
          .filter(c => c.chat && c.chat.endsWith('@g.us'))
          .map(c => ({ jid: c.chat, name: namesMap[c.chat] || c.chat.split('@')[0] }))
          .sort((a, b) => a.name.localeCompare(b.name));
      } catch (err) {
        this.govListError = err.message || 'Load failed';
      } finally {
        this.govListLoading = false;
        this.govListLoaded  = true;
      }
    },

    govFilteredGroups() {
      const q = (this.govSearch || '').toLowerCase();
      if (!q) return this.govGroups;
      return this.govGroups.filter(g =>
        g.jid.toLowerCase().includes(q) || (g.name || '').toLowerCase().includes(q)
      );
    },

    govSelectGroup(jid) {
      this.govJID       = jid;
      this.govData      = null;
      this.govProfile   = null;
      this.govActiveTab = 'overview';
      this.govLoad();
      this.govLoadProfile();
    },

    // ── overview load ─────────────────────────────────────────────────────────
    async govLoad() {
      if (!this.govJID || this.govLoading) return;
      this.govLoading = true;
      this.govError   = '';
      try {
        const jid = encodeURIComponent(this.govJID);
        const res = await fetch(
          `/zaplab/api/groups/${jid}/overview?period=${this.govPeriod}`,
          { headers: apiHeaders() }
        );
        if (!res.ok) {
          const d = await res.json().catch(() => ({}));
          this.govError = d.error || `HTTP ${res.status}`;
          return;
        }
        this.govData = await res.json();
      } catch (err) {
        this.govError = err.message || 'Load failed';
      } finally {
        this.govLoading = false;
      }
    },

    // ── profile photo (live, best-effort) ─────────────────────────────────────
    async govLoadProfile() {
      if (!this.govJID) return;
      this.govProfileLoading = true;
      try {
        const jid = encodeURIComponent(this.govJID);
        const res = await fetch(`/zaplab/api/contacts/${jid}`, { headers: apiHeaders() });
        if (res.ok) this.govProfile = await res.json();
      } catch (_) {}
      this.govProfileLoading = false;
    },

    // ── heatmap ───────────────────────────────────────────────────────────────
    govHeatValue(dow, hour) {
      if (!this.govData) return 0;
      const cell = (this.govData.heatmap || []).find(c => c.dow === dow && c.hour === hour);
      return cell ? cell.count : 0;
    },
    govHeatMax() {
      if (!this.govData) return 1;
      return Math.max(1, ...(this.govData.heatmap || []).map(c => c.count));
    },
    govHeatColor(dow, hour) {
      const v = this.govHeatValue(dow, hour);
      const pct = v / this.govHeatMax();
      return `rgba(63,185,80,${(0.07 + pct * 0.88).toFixed(2)})`;
    },
    govHeatDays:  ['Sun','Mon','Tue','Wed','Thu','Fri','Sat'],
    govHeatHours: ['00','01','02','03','04','05','06','07','08','09','10','11',
                   '12','13','14','15','16','17','18','19','20','21','22','23'],

    // ── sparkline ─────────────────────────────────────────────────────────────
    govSparklinePath() {
      const pts = (this.govData && this.govData.daily) || [];
      if (pts.length < 2) return '';
      const W = 400, H = 56;
      const maxV = Math.max(1, ...pts.map(p => p.count));
      return pts.map((p, i) => {
        const x = (i / (pts.length - 1)) * W;
        const y = H - (p.count / maxV) * H;
        return (i === 0 ? 'M' : 'L') + x.toFixed(1) + ',' + y.toFixed(1);
      }).join(' ');
    },
    govSparklineArea() {
      const pts = (this.govData && this.govData.daily) || [];
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

    // ── type distribution ─────────────────────────────────────────────────────
    govTypeColor(type) {
      return {
        text:     'bg-blue-500',
        image:    'bg-green-500',
        video:    'bg-purple-500',
        audio:    'bg-orange-400',
        document: 'bg-yellow-500',
        sticker:  'bg-pink-500',
        reaction: 'bg-red-400',
      }[type] || 'bg-gray-400';
    },
    govTypeMax() {
      if (!this.govData) return 1;
      return Math.max(1, ...(this.govData.type_distribution || []).map(t => t.count));
    },

    // ── sorted members (all known members) ────────────────────────────────────
    govSortedMembers() {
      const members = (this.govData && this.govData.current_members) || [];
      const dir = this.govMemberSortDir === 'asc' ? 1 : -1;
      return [...members].sort((a, b) => {
        if (this.govMemberSort === 'name') {
          return dir * (a.name || '').localeCompare(b.name || '');
        }
        return dir * ((a.msg_count || 0) - (b.msg_count || 0));
      });
    },
    govToggleMemberSort(col) {
      if (this.govMemberSort === col) {
        this.govMemberSortDir = this.govMemberSortDir === 'asc' ? 'desc' : 'asc';
      } else {
        this.govMemberSort = col;
        this.govMemberSortDir = col === 'name' ? 'asc' : 'desc';
      }
    },

    // ── member bar width (for top5) ───────────────────────────────────────────
    govMemberBarWidth(count) {
      const max = Math.max(1, ...(this.govData?.top5_active || []).map(m => m.msg_count));
      return Math.round((count / max) * 100);
    },

    // ── formatting ────────────────────────────────────────────────────────────
    govPeriodLabel: p => p === 0 ? 'All time' : `${p}d`,
    govFmt: n => (n || 0).toLocaleString(),
    govTimeAgo(ts) {
      if (!ts) return '—';
      const diff = Date.now() - new Date(ts).getTime();
      const m = Math.floor(diff / 60000);
      if (m < 1)  return 'just now';
      if (m < 60) return `${m}m ago`;
      const h = Math.floor(m / 60);
      if (h < 24) return `${h}h ago`;
      const d = Math.floor(h / 24);
      if (d < 30) return `${d}d ago`;
      return new Date(ts).toLocaleDateString();
    },
    govFmtDate(ts) {
      if (!ts) return '—';
      return new Date(ts).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
    },

    // ── navigation ────────────────────────────────────────────────────────────
    govOpenConversation() {
      if (!this.govJID) return;
      Alpine.store('nav').cvSelectedChat = this.govJID;
      setSection('conversation');
    },
    govOpenMembership() {
      if (!this.govJID) return;
      Alpine.store('nav').gmtJID = this.govJID;
      setSection('group-membership');
    },
    govOpenSearch() {
      if (!this.govJID) return;
      Alpine.store('nav').srQuery = '';
      Alpine.store('nav').srChat  = this.govJID;
      setSection('search');
    },
    govOpenNetworkGraph() {
      if (!this.govJID) return;
      setSection('networkgraph');
    },
    govOpenMemberProfile(jid) {
      Alpine.store('nav').coJID = jid;
      setSection('contact-overview');
    },
  };
}
