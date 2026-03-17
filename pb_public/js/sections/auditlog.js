// Audit Log — records API calls for script runs, imports, and message sends.
function auditLogSection() {
  return {
    // ── state ──
    alog: {
      loading:    false,
      toast:      null,
      entries:    null,
      days:       7,
      method:     '',
      pathFilter: '',
    },

    // ── init ──
    initAuditLog() {
      this.$watch('activeSection', val => {
        if (val === 'audit-log' && !this.alog.entries) {
          this.loadAuditLog();
        }
      });
    },

    // ── load ──
    async loadAuditLog() {
      this.alog.loading = true;
      this.alog.toast   = null;
      try {
        const params = new URLSearchParams({ days: this.alog.days, limit: 500 });
        if (this.alog.method)     params.set('method', this.alog.method);
        if (this.alog.pathFilter) params.set('path', this.alog.pathFilter);
        const res  = await this.zapFetch(`/zaplab/api/audit?${params}`,
          { headers: { 'X-API-Token': this.apiToken } });
        const data = await res.json();
        if (res.ok) {
          this.alog.entries = data;
        } else {
          this.alog.toast = { ok: false, message: data.error || 'Failed to load audit log' };
        }
      } catch (err) {
        this.alog.toast = { ok: false, message: err.message };
      } finally {
        this.alog.loading = false;
      }
    },

    alogMethodBadge(method) {
      const map = {
        POST:   'bg-green-700 text-green-200',
        PUT:    'bg-blue-700 text-blue-200',
        PATCH:  'bg-yellow-700 text-yellow-200',
        DELETE: 'bg-red-700 text-red-200',
      };
      return map[method] || 'bg-gray-700 text-gray-200';
    },

    alogPathShort(path) {
      return path.replace('/zaplab/api/', '');
    },
  };
}
