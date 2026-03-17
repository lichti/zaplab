// Plugin System / Scripting Engine
// Provides a JavaScript sandbox (goja) for scripting WhatsApp automation.
// Scripts are persisted in the `scripts` PocketBase collection.
// Available sandbox APIs: console.log, wa.sendText, wa.status, http.get,
//   http.post, db.query, sleep.
function scriptingSection() {
  return {
    // ── state ──
    scLoading:    false,
    scError:      '',
    scScripts:    [],
    scSelected:   null,   // script being edited
    scRunning:    false,
    scRunOutput:  '',
    scRunError:   '',
    scRunDurationMs: 0,
    scRunStatus:  '',
    // adhoc editor
    scAdhocCode:  '// Type JavaScript here and click Run\nconsole.log("Hello from zaplab!");\nconsole.log("WA status:", wa.status());',
    scAdhocRunning: false,
    scAdhocOutput:  '',
    scAdhocError:   '',
    scAdhocStatus:  '',
    scAdhocDurationMs: 0,
    // new script form
    scNewName:    '',
    scNewDesc:    '',
    scNewCode:    '// Script code\nconsole.log("Hello!");',
    scNewTimeout: 10,
    scShowNew:    false,

    // ── init ──
    initScripting() {
      this.$watch('activeSection', val => {
        if (val === 'scripting' && this.scScripts.length === 0) this.scLoad();
      });
    },

    // ── load ──
    async scLoad() {
      if (this.scLoading) return;
      this.scLoading = true;
      this.scError   = '';
      try {
        const res = await fetch('/zaplab/api/scripts', { headers: this.apiHeaders() });
        if (res.ok) {
          const d = await res.json();
          this.scScripts = d.scripts || [];
        } else {
          this.scError = `HTTP ${res.status}`;
        }
      } catch (err) {
        this.scError = err.message || 'Load failed';
      } finally {
        this.scLoading = false;
      }
    },

    // ── create ──
    async scCreate() {
      if (!this.scNewName.trim()) return;
      try {
        const res = await fetch('/zaplab/api/scripts', {
          method: 'POST',
          headers: { ...this.apiHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name:         this.scNewName.trim(),
            description:  this.scNewDesc.trim(),
            code:         this.scNewCode,
            enabled:      true,
            timeout_secs: this.scNewTimeout || 10,
          }),
        });
        if (res.ok) {
          const d = await res.json();
          this.scScripts.unshift(d);
          this.scNewName = '';
          this.scNewDesc = '';
          this.scNewCode = '// Script code\nconsole.log("Hello!");';
          this.scNewTimeout = 10;
          this.scShowNew = false;
          this.scSelected = d;
        } else {
          const d = await res.json();
          this.scError = d.message || `HTTP ${res.status}`;
        }
      } catch (err) {
        this.scError = err.message || 'Create failed';
      }
    },

    // ── update ──
    async scSave(script) {
      if (!script) return;
      try {
        const res = await fetch(`/zaplab/api/scripts/${script.id}`, {
          method: 'PATCH',
          headers: { ...this.apiHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name:         script.name,
            description:  script.description,
            code:         script.code,
            enabled:      script.enabled,
            timeout_secs: script.timeout_secs,
          }),
        });
        if (res.ok) {
          const d = await res.json();
          const idx = this.scScripts.findIndex(s => s.id === d.id);
          if (idx >= 0) this.scScripts[idx] = d;
        } else {
          const d = await res.json();
          this.scError = d.message || `HTTP ${res.status}`;
        }
      } catch (err) {
        this.scError = err.message || 'Save failed';
      }
    },

    // ── delete ──
    async scDelete(script) {
      if (!script) return;
      if (!confirm(`Delete script "${script.name}"?`)) return;
      try {
        const res = await fetch(`/zaplab/api/scripts/${script.id}`, {
          method: 'DELETE',
          headers: this.apiHeaders(),
        });
        if (res.ok) {
          this.scScripts = this.scScripts.filter(s => s.id !== script.id);
          if (this.scSelected && this.scSelected.id === script.id) this.scSelected = null;
        } else {
          this.scError = `HTTP ${res.status}`;
        }
      } catch (err) {
        this.scError = err.message || 'Delete failed';
      }
    },

    // ── run persisted script ──
    async scRun(script) {
      if (!script || this.scRunning) return;
      this.scRunning    = true;
      this.scRunOutput  = '';
      this.scRunError   = '';
      this.scRunStatus  = '';
      this.scRunDurationMs = 0;
      try {
        const res = await fetch(`/zaplab/api/scripts/${script.id}/run`, {
          method: 'POST',
          headers: this.apiHeaders(),
        });
        const d = await res.json();
        this.scRunStatus      = d.status || '';
        this.scRunOutput      = d.output || '';
        this.scRunError       = d.error  || '';
        this.scRunDurationMs  = d.duration_ms || 0;
        // refresh list entry
        const idx = this.scScripts.findIndex(s => s.id === script.id);
        if (idx >= 0) {
          this.scScripts[idx].last_run_status      = d.status;
          this.scScripts[idx].last_run_output      = d.output;
          this.scScripts[idx].last_run_error       = d.error;
          this.scScripts[idx].last_run_duration_ms = d.duration_ms;
        }
        if (this.scSelected && this.scSelected.id === script.id) {
          this.scSelected.last_run_status      = d.status;
          this.scSelected.last_run_output      = d.output;
          this.scSelected.last_run_error       = d.error;
          this.scSelected.last_run_duration_ms = d.duration_ms;
        }
      } catch (err) {
        this.scRunError  = err.message || 'Run failed';
        this.scRunStatus = 'error';
      } finally {
        this.scRunning = false;
      }
    },

    // ── run adhoc ──
    async scRunAdhoc() {
      if (!this.scAdhocCode.trim() || this.scAdhocRunning) return;
      this.scAdhocRunning    = true;
      this.scAdhocOutput     = '';
      this.scAdhocError      = '';
      this.scAdhocStatus     = '';
      this.scAdhocDurationMs = 0;
      try {
        const res = await fetch('/zaplab/api/scripts/run', {
          method: 'POST',
          headers: { ...this.apiHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({ code: this.scAdhocCode, timeout_secs: 15 }),
        });
        const d = await res.json();
        this.scAdhocStatus     = d.status || '';
        this.scAdhocOutput     = d.output || '';
        this.scAdhocError      = d.error  || '';
        this.scAdhocDurationMs = d.duration_ms || 0;
      } catch (err) {
        this.scAdhocError  = err.message || 'Run failed';
        this.scAdhocStatus = 'error';
      } finally {
        this.scAdhocRunning = false;
      }
    },

    // ── helpers ──
    scStatusClass(status) {
      if (status === 'success') return 'text-green-500 dark:text-green-400';
      if (status === 'error')   return 'text-red-500  dark:text-red-400';
      return 'text-gray-400';
    },
    scStatusIcon(status) {
      if (status === 'success') return '✓';
      if (status === 'error')   return '✗';
      return '—';
    },
  };
}
