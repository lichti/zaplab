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

    _sseSource:       null,
    _sseRetryTimer:   null,
    _sseRetryDelay:   1000,

    // ── methods ──
    async loadInitialEvents() {
      try {
        const res  = await fetch('/zaplab/api/events/recent?limit=100', { headers: apiHeaders() });
        const data = await res.json();
        if (res.ok) {
          this.events = (data.events || []).map(r => ({ ...r, _isNew: false }));
        }
      } catch (err) {
        console.error('Failed to load events:', err);
      }
    },

    subscribeEvents() {
      this._sseConnect();
    },

    _sseConnect() {
      if (this._sseSource) {
        this._sseSource.close();
        this._sseSource = null;
      }

      const apiToken = localStorage.getItem('zaplab-api-token') || '';
      // EventSource doesn't support custom headers — pass token as query param.
      const url = `/zaplab/api/events/stream?token=${encodeURIComponent(apiToken)}`;

      this.connectionStatus = 'connecting';
      const es = new EventSource(url);
      this._sseSource = es;

      es.addEventListener('connected', () => {
        this.connectionStatus = 'connected';
        this._sseRetryDelay = 1000; // reset backoff on success
      });

      es.addEventListener('message', e => {
        try {
          const evt = JSON.parse(e.data);
          // Normalise: SSEEvent has {type, data}; we need a flat record-like object.
          const id = evt.data?.Info?.ID || evt.data?.id || crypto.randomUUID();
          const record = {
            id,
            type:    evt.type,
            raw:     evt.data,
            created: new Date().toISOString(),
            _isNew:  true,
          };
          // Use assignment (not unshift) for explicit Alpine.js reactivity.
          this.events = [record, ...this.events];
          setTimeout(() => {
            const idx = this.events.findIndex(x => x.id === id);
            if (idx !== -1) {
              const updated = [...this.events];
              updated[idx] = { ...updated[idx], _isNew: false };
              this.events = updated;
            }
          }, 3000);
        } catch (err) {
          console.error('SSE parse error:', err);
        }
      });

      es.onerror = () => {
        this.connectionStatus = 'disconnected';
        es.close();
        this._sseSource = null;
        // Exponential backoff, cap at 30s
        this._sseRetryTimer = setTimeout(() => {
          this._sseRetryDelay = Math.min(this._sseRetryDelay * 2, 30000);
          this._sseConnect();
        }, this._sseRetryDelay);
      };
    },

    _sseDisconnect() {
      if (this._sseRetryTimer) {
        clearTimeout(this._sseRetryTimer);
        this._sseRetryTimer = null;
      }
      if (this._sseSource) {
        this._sseSource.close();
        this._sseSource = null;
      }
      this.connectionStatus = 'disconnected';
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
