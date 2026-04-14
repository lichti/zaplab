// Contact Cache — browse cached contact metadata and manage cache population.
function contactCacheSection() {
  return {
    cc: {
      contacts: [],
      total:    0,
      loading:  false,
      error:    '',
      toast:    null,
      search:   '',
    },
    ccRefreshJid: '',

    async ccLoad() {
      this.cc.loading = true;
      this.cc.error   = '';
      try {
        const p = new URLSearchParams();
        if (this.cc.search) p.set('q', this.cc.search);
        const res  = await fetch(`/zaplab/api/contact-cache?${p}`, { headers: apiHeaders() });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.cc.contacts = data.contacts || [];
        this.cc.total    = data.total    || 0;
      } catch (err) {
        this.cc.error = err.message;
      } finally {
        this.cc.loading = false;
      }
    },

    async ccPopulate() {
      this.cc.loading = true;
      this.cc.toast   = null;
      try {
        const res  = await fetch('/zaplab/api/contact-cache/populate', {
          method: 'POST', headers: apiHeaders(),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.cc.toast = { ok: true, message: 'Cache population started in background' };
        setTimeout(() => this.ccLoad(), 3000);
      } catch (err) {
        this.cc.toast = { ok: false, message: err.message };
      } finally {
        this.cc.loading = false;
      }
    },

    async ccRefresh() {
      if (!this.ccRefreshJid.trim()) return;
      this.cc.loading = true;
      this.cc.toast   = null;
      try {
        const res  = await fetch(`/zaplab/api/contact-cache/refresh?jid=${encodeURIComponent(this.ccRefreshJid.trim())}`, {
          method: 'POST', headers: apiHeaders(),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.cc.toast    = { ok: true, message: `Refreshed: ${data.jid}` };
        this.ccRefreshJid = '';
        await this.ccLoad();
      } catch (err) {
        this.cc.toast = { ok: false, message: err.message };
      } finally {
        this.cc.loading = false;
      }
    },

    async ccDelete(id) {
      this.cc.loading = true;
      try {
        const res = await fetch(`/zaplab/api/contact-cache/${id}`, {
          method: 'DELETE', headers: apiHeaders(),
        });
        if (!res.ok) { const d = await res.json(); throw new Error(d.error || 'Failed'); }
        await this.ccLoad();
      } catch (err) {
        this.cc.toast = { ok: false, message: err.message };
      } finally {
        this.cc.loading = false;
      }
    },
  };
}
