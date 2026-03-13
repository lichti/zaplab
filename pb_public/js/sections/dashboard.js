// Dashboard section — connection status, account summary, event statistics, recent events.
// Queries the PocketBase events and errors collections directly from the browser.
function dashboardSection() {
  return {
    // ── state ──
    dash: {
      loading:       false,
      lastFetch:     null,
      fetchError:    null,
      // all-time counters
      totalEvents:   0,
      totalReceived: 0,
      totalSent:     0,
      totalEdited:   0,
      totalDeleted:  0,
      totalErrors:   0,
      // last 24 h counters
      h24Events:     0,
      h24Received:   0,
      h24Sent:       0,
      h24Edited:     0,
      h24Deleted:    0,
      h24Errors:     0,
      // recent events list
      recentEvents:  [],
    },
    _dashCountdown: 60,

    // ── init ──
    initDashboard() {
      // Fetch whenever user navigates to dashboard
      this.$watch('activeSection', val => {
        if (val === 'dashboard') {
          this._dashCountdown = 60;
          this.dashFetch();
        }
      });

      // Single global ticker — counts down only while on dashboard
      setInterval(() => {
        if (this.activeSection !== 'dashboard') {
          this._dashCountdown = 60;
          return;
        }
        this._dashCountdown--;
        if (this._dashCountdown <= 0) {
          this._dashCountdown = 60;
          this.dashFetch();
        }
      }, 1000);

      // Fetch immediately if dashboard is the active section on page load
      if (this.activeSection === 'dashboard') this.dashFetch();
    },

    // ── data fetch ──
    async dashFetch() {
      if (this.dash.loading) return;

      this.dash.loading    = true;
      this.dash.fetchError = null;

      try {
        const since24h = new Date(Date.now() - 86400000)
          .toISOString().slice(0, 19).replace('T', ' ');

        const fRecv    = "type ~ 'Message' && type != 'SentMessage' && raw !~ '\"IsFromMe\":true'";
        const fSent    = "(type = 'SentMessage' || (type ~ 'Message' && type != 'SentMessage' && raw ~ '\"IsFromMe\":true'))";
        const fEdited  = "type = 'Message' && raw ~ '\"Edit\":\"1\"'";
        const fDeleted = "type = 'Message' && (raw ~ '\"Edit\":\"7\"' || raw ~ '\"Edit\":\"8\"')";
        const t24      = f => `(${f}) && created >= '${since24h}'`;

        // count helper — requestKey: null disables PocketBase SDK auto-cancellation
        // (parallel requests to the same collection would otherwise cancel each other)
        const count = (col, filter) =>
          pb.collection(col)
            .getList(1, 1, filter ? { filter, requestKey: null } : { requestKey: null })
            .then(r => r.totalItems);

        // Use allSettled so one failing query doesn't block the rest
        const results = await Promise.allSettled([
          count('events', ''),                         // 0  totalEvents
          count('events', fRecv),                      // 1  totalReceived
          count('events', fSent),                      // 2  totalSent
          count('events', fEdited),                    // 3  totalEdited
          count('events', fDeleted),                   // 4  totalDeleted
          count('errors', ''),                         // 5  totalErrors
          count('events', `created >= '${since24h}'`), // 6  h24Events
          count('events', t24(fRecv)),                 // 7  h24Received
          count('events', t24(fSent)),                 // 8  h24Sent
          count('events', t24(fEdited)),               // 9  h24Edited
          count('events', t24(fDeleted)),              // 10 h24Deleted
          count('errors', `created >= '${since24h}'`), // 11 h24Errors
          pb.collection('events').getList(1, 10, { sort: '-created', requestKey: null }), // 12 recent
        ]);

        // Extract value from settled result, fallback to previous value or 0
        const val = (i, prev) =>
          results[i].status === 'fulfilled' ? results[i].value : prev;

        // Log any failures to console for debugging
        results.forEach((r, i) => {
          if (r.status === 'rejected') console.warn(`dashboard query[${i}] failed:`, r.reason);
        });

        // Assign each property individually — most reliable way to trigger Alpine reactivity
        this.dash.totalEvents   = val(0, this.dash.totalEvents);
        this.dash.totalReceived = val(1, this.dash.totalReceived);
        this.dash.totalSent     = val(2, this.dash.totalSent);
        this.dash.totalEdited   = val(3, this.dash.totalEdited);
        this.dash.totalDeleted  = val(4, this.dash.totalDeleted);
        this.dash.totalErrors   = val(5, this.dash.totalErrors);
        this.dash.h24Events     = val(6, this.dash.h24Events);
        this.dash.h24Received   = val(7, this.dash.h24Received);
        this.dash.h24Sent       = val(8, this.dash.h24Sent);
        this.dash.h24Edited     = val(9, this.dash.h24Edited);
        this.dash.h24Deleted    = val(10, this.dash.h24Deleted);
        this.dash.h24Errors     = val(11, this.dash.h24Errors);

        const recentResult = results[12];
        if (recentResult.status === 'fulfilled') {
          this.dash.recentEvents = recentResult.value.items;
        }

        this.dash.lastFetch = new Date();
      } catch (err) {
        console.error('dashboard fetch error:', err);
        this.dash.fetchError = err.message || 'Fetch failed';
      } finally {
        this.dash.loading = false;
      }
    },

    dashRefresh() {
      this._dashCountdown = 60;
      this.dash.loading = false; // reset guard in case it got stuck
      this.dashFetch();
    },

    // ── helpers ──
    dashFmt(n) {
      return Number(n).toLocaleString();
    },

    dashLastFetch() {
      if (!this.dash.lastFetch) return '';
      return this.dash.lastFetch.toLocaleTimeString('en-GB', { hour12: false });
    },

    dashConnLabel() {
      const map = {
        connected:    'Connected',
        connecting:   'Connecting…',
        qr:           'Awaiting QR scan',
        timeout:      'QR timeout',
        disconnected: 'Disconnected',
        loggedout:    'Logged out',
        unknown:      'Unknown',
      };
      return map[this.wa.status] || this.wa.status;
    },

    dashConnDotClass() {
      if (this.wa.status === 'connected')                                      return 'bg-green-500';
      if (['connecting', 'qr'].includes(this.wa.status))                      return 'bg-yellow-400 animate-pulse';
      if (['disconnected', 'loggedout', 'timeout'].includes(this.wa.status))  return 'bg-red-500';
      return 'bg-gray-400';
    },

    // Navigate to Event Browser and select the given event record.
    dashGoToEvent(item) {
      this.eb.selected = item;
      this.setSection('eventbrowser');
    },

    dashConnCardClass() {
      if (this.wa.status === 'connected')                                      return 'border-green-300  dark:border-green-800  bg-green-50  dark:bg-green-950/30';
      if (['connecting', 'qr'].includes(this.wa.status))                      return 'border-yellow-300 dark:border-yellow-800 bg-yellow-50 dark:bg-yellow-950/30';
      if (['disconnected', 'loggedout', 'timeout'].includes(this.wa.status))  return 'border-red-300   dark:border-red-900    bg-red-50    dark:bg-red-950/30';
      return 'border-gray-200 dark:border-[#30363d] bg-white dark:bg-[#161b22]';
    },
  };
}
