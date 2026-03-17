// Media Gallery
// GET /zaplab/api/media/gallery?type=...&chat=...&limit=50&offset=0
function gallerySection() {
  return {
    // ── state ──
    glLoading:      false,
    glError:        '',
    glItems:        [],
    glTotal:        0,
    glLimit:        50,
    glOffset:       0,
    glTypeFilter:   '',   // 'image'|'video'|'audio'|'document'|'sticker'|''
    glChatFilter:   '',
    glLightboxItem: null,

    // ── init ──
    initGallery() {
      this.$watch('activeSection', val => {
        if (val === 'gallery' && this.glItems.length === 0 && !this.glLoading) this.glLoad();
      });
      // Close lightbox on Escape
      document.addEventListener('keydown', e => {
        if (e.key === 'Escape' && this.glLightboxItem) this.glLightboxItem = null;
      });
    },

    // ── load ──
    async glLoad(resetOffset = true) {
      if (this.glLoading) return;
      if (resetOffset) this.glOffset = 0;
      this.glLoading = true;
      this.glError   = '';
      try {
        const params = new URLSearchParams({ limit: this.glLimit, offset: this.glOffset });
        if (this.glTypeFilter) params.set('type', this.glTypeFilter);
        if (this.glChatFilter) params.set('chat', this.glChatFilter);
        const res = await fetch('/zaplab/api/media/gallery?' + params, { headers: this.apiHeaders() });
        if (res.ok) {
          const d = await res.json();
          this.glItems = d.items || [];
          this.glTotal = d.total || 0;
        } else {
          const d = await res.json().catch(() => ({}));
          this.glError = d.error || `HTTP ${res.status}`;
        }
      } catch (err) {
        this.glError = err.message || 'Load failed';
      } finally {
        this.glLoading = false;
      }
    },

    glApplyFilters() {
      this.glLoad(true);
    },

    glNextPage() {
      if (this.glOffset + this.glLimit < this.glTotal) {
        this.glOffset += this.glLimit;
        this.glLoad(false);
      }
    },
    glPrevPage() {
      if (this.glOffset > 0) {
        this.glOffset = Math.max(0, this.glOffset - this.glLimit);
        this.glLoad(false);
      }
    },

    glOpenLightbox(item) {
      this.glLightboxItem = item;
    },
    glCloseLightbox() {
      this.glLightboxItem = null;
    },

    glPageLabel() {
      if (this.glTotal === 0) return 'No media';
      const from = this.glOffset + 1;
      const to   = Math.min(this.glOffset + this.glLimit, this.glTotal);
      return `${from}–${to} of ${this.glTotal}`;
    },

    glTypeIcon(type) {
      const icons = { image: '🖼', video: '▶', audio: '🎵', document: '📄', sticker: '🔖' };
      return icons[type] || '?';
    },

    glFormatDate(created) {
      if (!created) return '';
      return new Date(created.replace(' ', 'T') + 'Z').toLocaleString();
    },
    glShortJID(jid) {
      if (!jid) return '?';
      return jid.split('@')[0];
    },
  };
}
