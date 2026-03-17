// Annotations — attach research notes to WhatsApp protocol events.
function annotationsSection() {
  return {
    // ── state ──
    anLoading:    false,
    anError:      '',
    anItems:      [],
    anTotal:      0,
    anPage:       1,
    anPerPage:    50,
    anFilterEvt:  '',
    anFilterJID:  '',

    // ── inline editor (used from both this section and event browser) ──
    anEditOpen:   false,
    anEditID:     null,       // null = new, string = existing id
    anEditEventID:'',
    anEditEventType: '',
    anEditJID:    '',
    anEditNote:   '',
    anEditTags:   '',         // comma-separated string for the input
    anEditSaving: false,
    anEditError:  '',

    // ── realtime subscription ──
    anSubscription: null,
    anConnStatus:   'disconnected',

    // ── init ──
    initAnnotations() {
      this.anSubscribe();
    },

    anSubscribe() {
      if (this.anSubscription) return;
      this.anConnStatus = 'connecting';
      this.anSubscription = true;
      pb.collection('annotations').subscribe('*', evt => {
        if (evt.action === 'create') {
          this.anItems.unshift(this._anMapRecord(evt.record));
          this.anTotal++;
        } else if (evt.action === 'update') {
          const idx = this.anItems.findIndex(a => a.id === evt.record.id);
          if (idx >= 0) this.anItems[idx] = this._anMapRecord(evt.record);
        } else if (evt.action === 'delete') {
          this.anItems = this.anItems.filter(a => a.id !== evt.record.id);
          this.anTotal--;
        }
      }).then(() => {
        this.anConnStatus = 'connected';
      }).catch(() => {
        this.anConnStatus = 'disconnected';
        this.anSubscription = null;
        setTimeout(() => this.anSubscribe(), 3000);
      });
    },

    _anMapRecord(r) {
      return {
        id:         r.id,
        event_id:   r.event_id   || '',
        event_type: r.event_type || '',
        jid:        r.jid        || '',
        note:       r.note       || '',
        tags:       Array.isArray(r.tags) ? r.tags : [],
        created:    r.created    || '',
        updated:    r.updated    || '',
      };
    },

    async anLoad() {
      if (this.anLoading) return;
      this.anLoading = true;
      this.anError = '';
      try {
        const params = new URLSearchParams({ page: this.anPage, per_page: this.anPerPage });
        if (this.anFilterEvt) params.set('event_id', this.anFilterEvt);
        if (this.anFilterJID) params.set('jid', this.anFilterJID);
        const res = await fetch(`/zaplab/api/annotations?${params}`, { headers: this.apiHeaders() });
        if (!res.ok) { this.anError = `HTTP ${res.status}`; return; }
        const data = await res.json();
        this.anItems = (data.items || []).map(this._anMapRecord.bind(this));
        this.anTotal = data.total || 0;
      } catch (err) {
        this.anError = err.message || 'Load failed';
      } finally {
        this.anLoading = false;
      }
    },

    async anRefresh() {
      this.anPage = 1;
      this.anItems = [];
      await this.anLoad();
    },

    // ── open editor ──
    anOpenNew(eventID = '', eventType = '', jid = '') {
      this.anEditID = null;
      this.anEditEventID = eventID;
      this.anEditEventType = eventType;
      this.anEditJID = jid;
      this.anEditNote = '';
      this.anEditTags = '';
      this.anEditError = '';
      this.anEditOpen = true;
    },

    anOpenEdit(ann) {
      this.anEditID = ann.id;
      this.anEditEventID = ann.event_id;
      this.anEditEventType = ann.event_type;
      this.anEditJID = ann.jid;
      this.anEditNote = ann.note;
      this.anEditTags = (ann.tags || []).join(', ');
      this.anEditError = '';
      this.anEditOpen = true;
    },

    anCloseEdit() {
      this.anEditOpen = false;
    },

    async anSave() {
      if (this.anEditSaving) return;
      if (!this.anEditNote.trim()) { this.anEditError = 'Note is required.'; return; }
      this.anEditSaving = true;
      this.anEditError = '';
      const tags = this.anEditTags.split(',').map(t => t.trim()).filter(Boolean);
      try {
        let res;
        if (this.anEditID) {
          res = await fetch(`/zaplab/api/annotations/${this.anEditID}`, {
            method: 'PATCH',
            headers: { ...this.apiHeaders(), 'Content-Type': 'application/json' },
            body: JSON.stringify({ note: this.anEditNote, tags }),
          });
        } else {
          res = await fetch('/zaplab/api/annotations', {
            method: 'POST',
            headers: { ...this.apiHeaders(), 'Content-Type': 'application/json' },
            body: JSON.stringify({
              event_id:   this.anEditEventID,
              event_type: this.anEditEventType,
              jid:        this.anEditJID,
              note:       this.anEditNote,
              tags,
            }),
          });
        }
        if (!res.ok) {
          const d = await res.json().catch(() => ({}));
          this.anEditError = d.message || d.error || `HTTP ${res.status}`;
          return;
        }
        this.anEditOpen = false;
      } catch (err) {
        this.anEditError = err.message || 'Save failed';
      } finally {
        this.anEditSaving = false;
      }
    },

    async anDelete(id) {
      if (!confirm('Delete this annotation?')) return;
      try {
        await fetch(`/zaplab/api/annotations/${id}`, {
          method: 'DELETE',
          headers: this.apiHeaders(),
        });
      } catch {}
    },

    // ── helpers ──
    anFormatTime(ts) {
      if (!ts) return '';
      return new Date(ts).toLocaleString([], {
        year: '2-digit', month: '2-digit', day: '2-digit',
        hour: '2-digit', minute: '2-digit', second: '2-digit',
      });
    },

    anTagColor(tag) {
      const colors = [
        'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
        'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
        'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
        'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300',
        'bg-teal-100 text-teal-700 dark:bg-teal-900/40 dark:text-teal-300',
      ];
      let h = 0;
      for (let i = 0; i < tag.length; i++) h = (h * 31 + tag.charCodeAt(i)) & 0xffff;
      return colors[h % colors.length];
    },
  };
}
