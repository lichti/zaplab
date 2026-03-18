// Group Membership Tracker — history of join/leave/promote/demote events.
function groupMembershipSection() {
  return {
    // ── state ──
    gmt: {
      loading:   false,
      toast:     null,
      history:   null,
      jidFilter: '',
      action:    '',
    },

    // ── init ──
    initGroupMembership() {
      this.$watch('activeSection', val => {
        if (val === 'group-membership' && !this.gmt.history) {
          this.loadGroupMembership();
        }
      });
      this.$watch(() => Alpine.store('nav').gmtJID, jid => {
        if (jid) {
          this.gmt.jidFilter = jid;
          Alpine.store('nav').gmtJID = '';
          this.loadGroupHistory(jid);
        }
      });
    },

    // ── load ──
    async loadGroupMembership() {
      this.gmt.loading = true;
      this.gmt.toast   = null;
      try {
        let url = '/zaplab/api/groups/membership?limit=500';
        if (this.gmt.action) url += `&action=${encodeURIComponent(this.gmt.action)}`;
        const res  = await this.zapFetch(url,
          { headers: { 'X-API-Token': this.apiToken } });
        const data = await res.json();
        if (res.ok) {
          this.gmt.history = data;
        } else {
          this.gmt.toast = { ok: false, message: data.error || 'Failed to load history' };
        }
      } catch (err) {
        this.gmt.toast = { ok: false, message: err.message };
      } finally {
        this.gmt.loading = false;
      }
    },

    async loadGroupHistory(jid) {
      this.gmt.loading = true;
      this.gmt.toast   = null;
      try {
        const res  = await this.zapFetch(`/zaplab/api/groups/${encodeURIComponent(jid)}/history?limit=500`,
          { headers: { 'X-API-Token': this.apiToken } });
        const data = await res.json();
        if (res.ok) {
          this.gmt.history = data;
          this.gmt.jidFilter = jid;
        } else {
          this.gmt.toast = { ok: false, message: data.error || 'Failed to load group history' };
        }
      } catch (err) {
        this.gmt.toast = { ok: false, message: err.message };
      } finally {
        this.gmt.loading = false;
      }
    },

    gmtFiltered() {
      if (!this.gmt.history) return [];
      let rows = this.gmt.history.history || [];
      if (this.gmt.jidFilter) {
        const q = this.gmt.jidFilter.toLowerCase();
        rows = rows.filter(r =>
          (r.group_jid  || '').toLowerCase().includes(q) ||
          (r.group_name || '').toLowerCase().includes(q) ||
          (r.member_jid || '').toLowerCase().includes(q)
        );
      }
      return rows;
    },

    gmtActionBadge(action) {
      const map = {
        join:    'bg-green-700 text-green-200',
        leave:   'bg-red-700 text-red-200',
        promote: 'bg-blue-700 text-blue-200',
        demote:  'bg-yellow-700 text-yellow-200',
      };
      return map[action] || 'bg-gray-700 text-gray-200';
    },
  };
}
