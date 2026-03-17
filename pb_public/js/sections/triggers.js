// Script Triggers (Event Hooks)
// Automatically execute scripts when WhatsApp events arrive.
// GET/POST/PATCH/DELETE /zaplab/api/script-triggers
function triggersSection() {
  return {
    // ── state ──
    trLoading:       false,
    trError:         '',
    trTriggers:      [],
    trSelected:      null,
    trShowNew:       false,
    trEventTypes:    [],
    trScriptOptions: [],
    // new trigger form
    trNewScriptId:    '',
    trNewEventType:   'Message',
    trNewJidFilter:   '',
    trNewTextPattern: '',
    trNewEnabled:     true,

    // ── init ──
    initTriggers() {
      this.$watch('activeSection', val => {
        if (val === 'triggers' && this.trTriggers.length === 0) this.trLoad();
      });
    },

    // ── load ──
    async trLoad() {
      if (this.trLoading) return;
      this.trLoading = true;
      this.trError   = '';
      try {
        const [triRes, typRes, scrRes] = await Promise.all([
          fetch('/zaplab/api/script-triggers',             { headers: this.apiHeaders() }),
          fetch('/zaplab/api/script-triggers/event-types', { headers: this.apiHeaders() }),
          fetch('/zaplab/api/scripts',                     { headers: this.apiHeaders() }),
        ]);
        if (triRes.ok) this.trTriggers      = (await triRes.json()).triggers || [];
        if (typRes.ok) this.trEventTypes    = (await typRes.json()).types    || [];
        if (scrRes.ok) this.trScriptOptions = (await scrRes.json()).scripts  || [];
      } catch (err) {
        this.trError = err.message || 'Load failed';
      } finally {
        this.trLoading = false;
      }
    },

    // ── create ──
    async trCreate() {
      if (!this.trNewScriptId || !this.trNewEventType) return;
      try {
        const res = await fetch('/zaplab/api/script-triggers', {
          method: 'POST',
          headers: { ...this.apiHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({
            script_id:    this.trNewScriptId,
            event_type:   this.trNewEventType,
            jid_filter:   this.trNewJidFilter.trim(),
            text_pattern: this.trNewTextPattern.trim(),
            enabled:      this.trNewEnabled,
          }),
        });
        if (res.ok) {
          const d = await res.json();
          this.trTriggers.unshift(d);
          this.trNewScriptId    = '';
          this.trNewEventType   = 'Message';
          this.trNewJidFilter   = '';
          this.trNewTextPattern = '';
          this.trNewEnabled     = true;
          this.trShowNew        = false;
        } else {
          const d = await res.json().catch(() => ({}));
          this.trError = d.error || d.message || `HTTP ${res.status}`;
        }
      } catch (err) {
        this.trError = err.message || 'Create failed';
      }
    },

    // ── save ──
    async trSave(t) {
      if (!t) return;
      try {
        const res = await fetch(`/zaplab/api/script-triggers/${t.id}`, {
          method: 'PATCH',
          headers: { ...this.apiHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({
            script_id:    t.script_id,
            event_type:   t.event_type,
            jid_filter:   t.jid_filter,
            text_pattern: t.text_pattern,
            enabled:      t.enabled,
          }),
        });
        if (res.ok) {
          const d = await res.json();
          const idx = this.trTriggers.findIndex(x => x.id === d.id);
          if (idx >= 0) this.trTriggers[idx] = d;
          if (this.trSelected && this.trSelected.id === d.id) this.trSelected = d;
        } else {
          const d = await res.json().catch(() => ({}));
          this.trError = d.error || d.message || `HTTP ${res.status}`;
        }
      } catch (err) {
        this.trError = err.message || 'Save failed';
      }
    },

    // ── delete ──
    async trDelete(t) {
      if (!t || !confirm(`Delete trigger for "${t.event_type}"?`)) return;
      try {
        const res = await fetch(`/zaplab/api/script-triggers/${t.id}`, {
          method: 'DELETE',
          headers: this.apiHeaders(),
        });
        if (res.ok) {
          this.trTriggers = this.trTriggers.filter(x => x.id !== t.id);
          if (this.trSelected && this.trSelected.id === t.id) this.trSelected = null;
        } else {
          this.trError = `HTTP ${res.status}`;
        }
      } catch (err) {
        this.trError = err.message || 'Delete failed';
      }
    },

    // ── helpers ──
    trScriptName(scriptId) {
      const s = this.trScriptOptions.find(x => x.id === scriptId);
      return s ? s.name : scriptId;
    },
    trToggleEnabled(t) {
      t.enabled = !t.enabled;
      this.trSave(t);
    },
  };
}
