// Cron Scheduler — manage cron-based scheduled execution of scripts.
function cronSchedulerSection() {
  return {
    // ── state ──
    csLoading:   false,
    csError:     '',
    csSchedules: [],
    csScripts:   [],
    csEditId:    null,   // script ID currently being edited
    csEditExpr:  '',     // cron expression in the edit input

    // ── init ──
    initCronScheduler() {},

    // ── data ──
    async csLoad() {
      if (this.csLoading) return;
      this.csLoading = true;
      this.csError   = '';
      try {
        const [schedRes, scriptsRes] = await Promise.all([
          fetch('/zaplab/api/scripts/cron',  { headers: this.apiHeaders() }),
          fetch('/zaplab/api/scripts',        { headers: this.apiHeaders() }),
        ]);
        if (schedRes.ok)   this.csSchedules = ((await schedRes.json()).schedules) || [];
        if (scriptsRes.ok) this.csScripts   = ((await scriptsRes.json()).scripts)  || [];
      } catch (err) {
        this.csError = err.message || 'Load failed';
      } finally {
        this.csLoading = false;
      }
    },

    // ── edit ──
    csStartEdit(script) {
      this.csEditId   = script.id;
      this.csEditExpr = script.cron_expression || '';
    },

    async csSave() {
      if (!this.csEditId) return;
      this.csError = '';
      try {
        const res = await fetch(`/zaplab/api/scripts/${this.csEditId}`, {
          method:  'PATCH',
          headers: { ...this.apiHeaders(), 'Content-Type': 'application/json' },
          body:    JSON.stringify({ cron_expression: this.csEditExpr }),
        });
        if (!res.ok) { const t = await res.text(); throw new Error(t); }
        this.csEditId = null;
        await this.csLoad();
      } catch (err) {
        this.csError = err.message || 'Save failed';
      }
    },

    async csClear(scriptId) {
      this.csError = '';
      try {
        const res = await fetch(`/zaplab/api/scripts/${scriptId}`, {
          method:  'PATCH',
          headers: { ...this.apiHeaders(), 'Content-Type': 'application/json' },
          body:    JSON.stringify({ cron_expression: '' }),
        });
        if (!res.ok) { const t = await res.text(); throw new Error(t); }
        await this.csLoad();
      } catch (err) {
        this.csError = err.message || 'Clear failed';
      }
    },

    // ── helpers ──
    csNextRun(scriptId) {
      const s = this.csSchedules.find(e => e.script_id === scriptId);
      if (!s || !s.next_run_at || s.next_run_at === '0001-01-01T00:00:00Z') return '—';
      return new Date(s.next_run_at).toLocaleString();
    },

    csPrevRun(scriptId) {
      const s = this.csSchedules.find(e => e.script_id === scriptId);
      if (!s || !s.prev_run_at || s.prev_run_at === '0001-01-01T00:00:00Z') return '—';
      return new Date(s.prev_run_at).toLocaleString();
    },

    csIsActive(scriptId) {
      return this.csSchedules.some(e => e.script_id === scriptId);
    },

    csScriptsWithCron() {
      return this.csScripts.filter(s => s.cron_expression);
    },

    csScriptsWithoutCron() {
      return this.csScripts.filter(s => !s.cron_expression);
    },

    csAllScripts() {
      return this.csScripts;
    },

    // Quick-insert common cron expressions
    csQuickExprs: [
      { label: 'Every minute',    expr: '* * * * *'     },
      { label: 'Every 5 min',     expr: '*/5 * * * *'   },
      { label: 'Every 15 min',    expr: '*/15 * * * *'  },
      { label: 'Every hour',      expr: '0 * * * *'     },
      { label: 'Every 6 hours',   expr: '0 */6 * * *'   },
      { label: 'Daily at 09:00',  expr: '0 9 * * *'     },
      { label: 'Weekdays 08:00',  expr: '0 8 * * 1-5'   },
      { label: 'Weekly Sunday',   expr: '0 0 * * 0'     },
    ],
  };
}
