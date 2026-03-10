// Live Events section — state, realtime subscription, resizer, copy helpers.
function eventsSection() {
  return {
    // ── state ──
    connectionStatus: 'connecting',
    events:           [],
    selectedEvent:    null,
    copied:           false,

    eventsHeight:  300,
    resizing:      false,
    _resizeStartY: 0,
    _resizeStartH: 0,

    // ── methods ──
    async loadInitialEvents() {
      try {
        const res = await pb.collection('events').getList(1, 100, { sort: '-created', requestKey: null });
        this.events = res.items.map(r => ({ ...r, _isNew: false }));
      } catch (err) {
        console.error('Failed to load events:', err);
      }
    },

    subscribeEvents() {
      this.connectionStatus = 'connecting';
      pb.collection('events').subscribe('*', e => {
        if (e.action === 'create') {
          this.events.unshift({ ...e.record, _isNew: true });
          setTimeout(() => {
            const ev = this.events.find(x => x.id === e.record.id);
            if (ev) ev._isNew = false;
          }, 3000);
        }
      }).then(() => {
        this.connectionStatus = 'connected';
      }).catch(() => {
        this.connectionStatus = 'disconnected';
        setTimeout(() => this.subscribeEvents(), 3000);
      });
    },

    selectEvent(ev) {
      this.selectedEvent = ev;
      this.copied = false;
    },
    clearEvents() {
      this.events = [];
      this.selectedEvent = null;
    },
    async copyJSON() {
      if (!this.selectedEvent) return;
      try {
        const { _isNew, ...clean } = this.selectedEvent;
        await navigator.clipboard.writeText(JSON.stringify(clean, null, 2));
        this.copied = true;
        setTimeout(() => { this.copied = false; }, 2000);
      } catch {}
    },

    startResize(e) {
      this.resizing      = true;
      this._resizeStartY = e.clientY;
      this._resizeStartH = this.eventsHeight;
    },
    onMouseMove(e) {
      if (!this.resizing) return;
      const newH = Math.max(80, this._resizeStartH + (e.clientY - this._resizeStartY));
      this.eventsHeight = newH;
    },
    stopResize() {
      this.resizing = false;
    },
  };
}
