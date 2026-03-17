// Script Import/Export — bulk backup and restore scripts.
function scriptsIOSection() {
  return {
    // ── state ──
    sioImporting:   false,
    sioToast:       null,
    sioImportResult: null,

    // ── export ──
    async exportScripts() {
      try {
        const res  = await this.zapFetch('/zaplab/api/scripts/export',
          { headers: { 'X-API-Token': this.apiToken } });
        const data = await res.json();
        if (!res.ok) {
          this.sioToast = { ok: false, message: data.error || 'Export failed' };
          return;
        }
        const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
        const a    = document.createElement('a');
        a.href     = URL.createObjectURL(blob);
        a.download = `zaplab-scripts-${Date.now()}.json`;
        a.click();
        this.sioToast = { ok: true, message: `${data.length} script(s) exported` };
      } catch (err) {
        this.sioToast = { ok: false, message: err.message };
      }
    },

    // ── import ──
    async importScripts(fileInput) {
      const file = fileInput?.files?.[0];
      if (!file) return;
      this.sioImporting = true;
      this.sioToast     = null;
      try {
        const text    = await file.text();
        const payload = JSON.parse(text);
        if (!Array.isArray(payload)) throw new Error('Expected a JSON array');
        const res  = await this.zapFetch('/zaplab/api/scripts/import', {
          method:  'POST',
          headers: { 'Content-Type': 'application/json', 'X-API-Token': this.apiToken },
          body:    JSON.stringify(payload),
        });
        const data = await res.json();
        this.sioToast = { ok: res.ok, message: data.message || (res.ok ? 'Done' : 'Import failed') };
        if (res.ok) {
          this.sioImportResult = data;
          // Refresh script list if we're already on the scripting section
          if (typeof this.loadScripts === 'function') this.loadScripts();
        }
      } catch (err) {
        this.sioToast = { ok: false, message: err.message };
      } finally {
        this.sioImporting = false;
        if (fileInput) fileInput.value = '';
      }
    },
  };
}
