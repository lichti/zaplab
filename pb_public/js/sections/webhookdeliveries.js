// Webhook Delivery Log — browse per-attempt webhook delivery records.
function webhookDeliveriesSection() {
  return {
    wd: {
      deliveries: [],
      total:      0,
      loading:    false,
      error:      '',
      toast:      null,
      statusFilter: '',  // delivered | failed | ''
      urlFilter:    '',
    },

    async wdLoad() {
      this.wd.loading = true;
      this.wd.error   = '';
      try {
        const p = new URLSearchParams({ limit: 200 });
        if (this.wd.statusFilter) p.set('status', this.wd.statusFilter);
        if (this.wd.urlFilter)    p.set('url',    this.wd.urlFilter);
        const res  = await fetch(`/zaplab/api/webhook/deliveries?${p}`, { headers: apiHeaders() });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.wd.deliveries = data.deliveries || [];
        this.wd.total      = data.total      || 0;
      } catch (err) {
        this.wd.error = err.message;
      } finally {
        this.wd.loading = false;
      }
    },

    async wdPurge(days) {
      this.wd.loading = true;
      this.wd.toast   = null;
      try {
        const res  = await fetch(`/zaplab/api/webhook/deliveries?days=${days}`, {
          method: 'DELETE', headers: apiHeaders(),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.wd.toast = { ok: true, message: `Deleted ${data.deleted} records older than ${days} days` };
        await this.wdLoad();
      } catch (err) {
        this.wd.toast = { ok: false, message: err.message };
      } finally {
        this.wd.loading = false;
      }
    },

    wdStatusBadge(status) {
      return status === 'delivered'
        ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
        : 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400';
    },

    wdHttpClass(code) {
      if (!code || code === 0) return 'text-gray-400';
      if (code < 300)           return 'text-green-600 dark:text-green-400';
      if (code < 500)           return 'text-yellow-600 dark:text-yellow-400';
      return 'text-red-500 dark:text-red-400';
    },
  };
}
