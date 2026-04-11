// Device Activity Tracker — RTT-based device state inference (Online/Standby/Offline).
function activityTrackerSection() {
  return {
    // ── state ──
    at: {
      enabled:     false,
      trackers:    [],
      loading:     false,
      toast:       null,
      jid:         '',
      probeMethod: 'delete',
    },
    atTab:        'trackers',   // 'trackers' | 'history'
    atHist: {
      jid:    '',
      days:   1,
      probes: [],
      total:  0,
      loading: false,
      error:  '',
    },
    atPollTimer: null,

    // ── init ──
    initActivityTracker() {
      this.$watch('activeSection', val => {
        if (val === 'activity-tracker') {
          this.atLoadStatus();
          this.atStartPoll();
        } else {
          this.atStopPoll();
        }
      });
    },

    // ── polling ──
    atStartPoll() {
      this.atStopPoll();
      this.atPollTimer = setInterval(() => {
        if (this.activeSection === 'activity-tracker') this.atLoadStatus();
      }, 5000);
    },
    atStopPoll() {
      if (this.atPollTimer) { clearInterval(this.atPollTimer); this.atPollTimer = null; }
    },

    // ── API calls ──
    async atLoadStatus() {
      try {
        const res = await fetch('/zaplab/api/activity-tracker/status', { headers: apiHeaders() });
        if (!res.ok) return;
        const data = await res.json();
        this.at.enabled  = data.enabled  || false;
        this.at.trackers = data.trackers || [];
      } catch {}
    },

    async atEnable() {
      this.at.loading = true;
      this.at.toast   = null;
      try {
        const res = await fetch('/zaplab/api/activity-tracker/enable', {
          method: 'POST', headers: apiHeaders(),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.at.enabled = true;
        this.at.toast   = { ok: true, message: 'Activity Tracker enabled' };
      } catch (err) {
        this.at.toast = { ok: false, message: err.message };
      } finally {
        this.at.loading = false;
      }
    },

    async atDisable() {
      this.at.loading = true;
      this.at.toast   = null;
      try {
        const res = await fetch('/zaplab/api/activity-tracker/disable', {
          method: 'POST', headers: apiHeaders(),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.at.enabled  = false;
        this.at.trackers = [];
        this.at.toast    = { ok: true, message: 'Activity Tracker disabled — all trackers stopped' };
      } catch (err) {
        this.at.toast = { ok: false, message: err.message };
      } finally {
        this.at.loading = false;
      }
    },

    async atStart() {
      if (!this.at.jid.trim()) return;
      this.at.loading = true;
      this.at.toast   = null;
      try {
        const res = await fetch('/zaplab/api/activity-tracker/start', {
          method:  'POST',
          headers: { ...apiHeaders(), 'Content-Type': 'application/json' },
          body:    JSON.stringify({ jid: this.at.jid.trim(), probe_method: this.at.probeMethod }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.at.jid   = '';
        this.at.toast = { ok: true, message: `Tracking ${data.jid}` };
        await this.atLoadStatus();
      } catch (err) {
        this.at.toast = { ok: false, message: err.message };
      } finally {
        this.at.loading = false;
      }
    },

    async atStop(jid) {
      this.at.loading = true;
      this.at.toast   = null;
      try {
        const res = await fetch('/zaplab/api/activity-tracker/stop', {
          method:  'POST',
          headers: { ...apiHeaders(), 'Content-Type': 'application/json' },
          body:    JSON.stringify({ jid }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.at.toast = { ok: true, message: `Stopped tracking ${jid}` };
        await this.atLoadStatus();
      } catch (err) {
        this.at.toast = { ok: false, message: err.message };
      } finally {
        this.at.loading = false;
      }
    },

    async atLoadHistory() {
      if (!this.atHist.jid.trim()) return;
      this.atHist.loading = true;
      this.atHist.error   = '';
      try {
        const p = new URLSearchParams({ jid: this.atHist.jid.trim(), days: this.atHist.days, limit: 500 });
        const res = await fetch(`/zaplab/api/activity-tracker/history?${p}`, { headers: apiHeaders() });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.atHist.probes = data.probes || [];
        this.atHist.total  = data.total  || 0;
      } catch (err) {
        this.atHist.error = err.message;
      } finally {
        this.atHist.loading = false;
      }
    },

    atViewHistory(jid) {
      this.atHist.jid    = jid;
      this.atHist.probes = [];
      this.atHist.total  = 0;
      this.atTab         = 'history';
      this.atLoadHistory();
    },

    // ── helpers ──
    atStateBadge(state) {
      const map = {
        Online:  'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
        Standby: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
        Offline: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
      };
      return map[state] || 'bg-gray-100 text-gray-600 dark:bg-[#21262d] dark:text-[#8b949e]';
    },

    atRttLabel(rtt) {
      if (rtt < 0) return 'timeout';
      return rtt + ' ms';
    },

    atRttClass(rtt) {
      if (rtt < 0) return 'text-red-500 dark:text-red-400';
      if (rtt < 500) return 'text-green-600 dark:text-green-400';
      if (rtt < 1500) return 'text-yellow-600 dark:text-yellow-400';
      return 'text-red-500 dark:text-red-400';
    },
  };
}
