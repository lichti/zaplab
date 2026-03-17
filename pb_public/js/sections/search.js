// Full-text Message Search
// GET /zaplab/api/search?q=...&type=...&chat=...&limit=50&offset=0
function searchSection() {
  return {
    // ── state ──
    srQuery:       '',
    srType:        '',
    srChat:        '',
    srLoading:     false,
    srError:       '',
    srResults:     [],
    srTotal:       0,
    srLimit:       50,
    srOffset:      0,
    srSelectedEvent: null,

    // ── init ──
    initSearch() {
      this.$watch('activeSection', val => {
        if (val !== 'search') this.srSelectedEvent = null;
      });
    },

    // ── search ──
    async srSearch(resetOffset = true) {
      if (!this.srQuery.trim()) return;
      if (resetOffset) this.srOffset = 0;
      this.srLoading = true;
      this.srError   = '';
      try {
        const params = new URLSearchParams({ q: this.srQuery.trim(), limit: this.srLimit, offset: this.srOffset });
        if (this.srType) params.set('type', this.srType);
        if (this.srChat) params.set('chat', this.srChat);
        const res = await fetch('/zaplab/api/search?' + params, { headers: this.apiHeaders() });
        if (res.ok) {
          const d = await res.json();
          this.srResults = d.results || [];
          this.srTotal   = d.total   || 0;
        } else {
          const d = await res.json().catch(() => ({}));
          this.srError = d.error || `HTTP ${res.status}`;
        }
      } catch (err) {
        this.srError = err.message || 'Search failed';
      } finally {
        this.srLoading = false;
      }
    },

    srClear() {
      this.srQuery   = '';
      this.srType    = '';
      this.srChat    = '';
      this.srResults = [];
      this.srTotal   = 0;
      this.srOffset  = 0;
      this.srError   = '';
      this.srSelectedEvent = null;
    },

    srNextPage() {
      if (this.srOffset + this.srLimit < this.srTotal) {
        this.srOffset += this.srLimit;
        this.srSearch(false);
      }
    },
    srPrevPage() {
      if (this.srOffset > 0) {
        this.srOffset = Math.max(0, this.srOffset - this.srLimit);
        this.srSearch(false);
      }
    },

    srOpenEvent(ev) {
      this.srSelectedEvent = ev;
    },

    srPageLabel() {
      if (this.srTotal === 0) return '';
      const from = this.srOffset + 1;
      const to   = Math.min(this.srOffset + this.srLimit, this.srTotal);
      return `${from}–${to} of ${this.srTotal}`;
    },

    // Navigate to conversation view for a chat
    srOpenConversation(chat) {
      this.cvSelectedChat = chat;
      this.setSection('conversation');
    },
  };
}
