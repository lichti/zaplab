// Notification Center — persistent in-app alerts for mentions, tracker state
// changes, and webhook failures.
function notificationsSection() {
  return {
    // ── state ──
    nc: {
      notifications: [],
      unreadCount:   0,
      loading:       false,
      error:         '',
      tab:           'unread', // 'unread' | 'all'
      typeFilter:    '',
    },

    // ── init ──
    initNotifications() {
      // Auto-load when section becomes active.
      this.$watch('activeSection', val => {
        if (val === 'notifications') this.ncLoad();
      });

      // Listen for real-time Notification SSE events.
      // eventsSection fires SSE messages; we intercept type='Notification'.
      const _origSubscribe = this.subscribeEvents?.bind(this);
      // Patch: intercept new events arriving via the shared SSE source.
      // We watch this.events (from eventsSection) for new items with type 'Notification'.
      this.$watch('events', (newEvents, oldEvents) => {
        const oldLen = (oldEvents || []).length;
        const newLen = (newEvents || []).length;
        if (newLen <= oldLen) return;
        // Check freshly prepended items (SSE pushes to front).
        for (let i = 0; i < newLen - oldLen; i++) {
          const ev = newEvents[i];
          if (ev?.type === 'Notification' && ev?.raw) {
            const n = ev.raw;
            this.nc.notifications = [{
              id:         n.id         || ev.id,
              type:       n.type       || 'Notification',
              title:      n.title      || '',
              body:       n.body       || '',
              entity_id:  n.entity_id  || '',
              entity_jid: n.entity_jid || '',
              read_at:    '',
              created:    n.created    || ev.created,
            }, ...this.nc.notifications];
            this.nc.unreadCount = (this.nc.unreadCount || 0) + 1;
          }
        }
      });
    },

    // ── methods ──
    async ncLoad() {
      this.nc.loading = true;
      this.nc.error   = '';
      try {
        const p = new URLSearchParams({
          status: this.nc.tab,
          limit:  200,
        });
        if (this.nc.typeFilter) p.set('type', this.nc.typeFilter);
        const res  = await fetch(`/zaplab/api/notifications?${p}`, { headers: apiHeaders() });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.nc.notifications = data.notifications || [];
        this.nc.unreadCount   = data.unread_count  || 0;
      } catch (err) {
        this.nc.error = err.message;
      } finally {
        this.nc.loading = false;
      }
    },

    async ncMarkRead(id) {
      try {
        const res = await fetch(`/zaplab/api/notifications/${id}/read`, {
          method: 'PUT', headers: apiHeaders(),
        });
        if (!res.ok) throw new Error('Failed');
        const idx = this.nc.notifications.findIndex(n => n.id === id);
        if (idx !== -1) {
          const updated = [...this.nc.notifications];
          updated[idx] = { ...updated[idx], read_at: new Date().toISOString() };
          this.nc.notifications = updated;
          if (this.nc.tab === 'unread') {
            this.nc.notifications = this.nc.notifications.filter(n => n.id !== id);
          }
          this.nc.unreadCount = Math.max(0, (this.nc.unreadCount || 0) - 1);
        }
      } catch (err) {
        this.nc.error = err.message;
      }
    },

    async ncMarkAllRead() {
      try {
        const res = await fetch('/zaplab/api/notifications/read-all', {
          method: 'POST', headers: apiHeaders(),
        });
        if (!res.ok) throw new Error('Failed');
        this.nc.unreadCount = 0;
        if (this.nc.tab === 'unread') {
          this.nc.notifications = [];
        } else {
          const now = new Date().toISOString();
          this.nc.notifications = this.nc.notifications.map(n =>
            n.read_at ? n : { ...n, read_at: now }
          );
        }
      } catch (err) {
        this.nc.error = err.message;
      }
    },

    async ncDelete(id) {
      try {
        const res = await fetch(`/zaplab/api/notifications/${id}`, {
          method: 'DELETE', headers: apiHeaders(),
        });
        if (!res.ok) throw new Error('Failed');
        const removed = this.nc.notifications.find(n => n.id === id);
        this.nc.notifications = this.nc.notifications.filter(n => n.id !== id);
        if (removed && !removed.read_at) {
          this.nc.unreadCount = Math.max(0, (this.nc.unreadCount || 0) - 1);
        }
      } catch (err) {
        this.nc.error = err.message;
      }
    },

    async ncPurge() {
      try {
        const res = await fetch('/zaplab/api/notifications/purge', {
          method: 'POST', headers: apiHeaders(),
        });
        if (!res.ok) throw new Error('Failed');
        await this.ncLoad();
      } catch (err) {
        this.nc.error = err.message;
      }
    },

    // Icon class per notification type
    ncTypeIcon(type) {
      switch (type) {
        case 'mention':         return 'text-blue-500';
        case 'tracker_state':   return 'text-purple-500';
        case 'webhook_failure': return 'text-red-500';
        default:                return 'text-gray-400';
      }
    },
    ncTypeLabel(type) {
      switch (type) {
        case 'mention':         return 'Mention';
        case 'tracker_state':   return 'Tracker';
        case 'webhook_failure': return 'Webhook';
        default:                return type;
      }
    },
  };
}
