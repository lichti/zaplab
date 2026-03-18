// App State Inspector section
// Decodes and displays the three whatsmeow app state SQLite tables:
//   whatsmeow_app_state_version       — collection names, version indexes, hashes
//   whatsmeow_app_state_sync_keys     — symmetric keys for decrypting patches
//   whatsmeow_app_state_mutation_macs — per-mutation HMAC integrity codes
function appStateSection() {
  return {
    // ── state ──
    asLoading:        false,
    asError:          '',
    asTab:            'collections',  // 'collections' | 'synckeys' | 'mutations'

    // Collections tab
    asCollections:    [],
    asColFilter:      '',

    // Sync keys tab
    asSyncKeys:       [],

    // Mutations tab
    asMutCollection:  'critical',
    asMutCollections: ['critical', 'regular', 'critical_unblock_to_primary', 'critical_block', 'regular_low'],
    asMutations:      [],
    asMutLimit:       100,
    asMutLoading:     false,

    // ── init ──
    initAppState() {},

    // ── load ──
    async asLoad() {
      if (this.asLoading) return;
      this.asLoading = true;
      this.asError   = '';
      try {
        const [colRes, keyRes] = await Promise.all([
          fetch('/zaplab/api/appstate/collections', { headers: apiHeaders() }),
          fetch('/zaplab/api/appstate/synckeys',    { headers: apiHeaders() }),
        ]);
        if (colRes.ok) {
          const d = await colRes.json();
          this.asCollections = d.collections || [];
        } else {
          const d = await colRes.json();
          this.asError = d.error || 'Failed to load collections';
        }
        if (keyRes.ok) {
          const d = await keyRes.json();
          this.asSyncKeys = d.sync_keys || [];
        }
      } catch (err) {
        this.asError = err.message;
      } finally {
        this.asLoading = false;
      }
    },

    async asLoadMutations() {
      if (this.asMutLoading) return;
      this.asMutLoading = true;
      this.asError = '';
      try {
        const params = new URLSearchParams({ collection: this.asMutCollection, limit: this.asMutLimit });
        const res = await fetch(`/zaplab/api/appstate/mutations?${params}`, { headers: apiHeaders() });
        const d   = await res.json();
        if (!res.ok) { this.asError = d.error || 'Failed'; return; }
        this.asMutations = d.mutations || [];
      } catch (err) {
        this.asError = err.message;
      } finally {
        this.asMutLoading = false;
      }
    },

    // ── filters ──
    asFilteredCollections() {
      if (!this.asColFilter) return this.asCollections;
      const f = this.asColFilter.toLowerCase();
      return this.asCollections.filter(c =>
        c.name.toLowerCase().includes(f) || c.jid.toLowerCase().includes(f)
      );
    },

    // ── collection metadata ──
    asCollectionIcon(name) {
      return { critical: '🔒', regular: '📋', critical_unblock_to_primary: '🔓',
               critical_block: '🚫', regular_low: '📝' }[name] || '📦';
    },

    asCollectionDesc(name) {
      return {
        critical:                    'Privacy settings, disappearing messages timer, PIN, 2FA config. Synced with highest priority — changes here affect all linked devices immediately.',
        regular:                     'Contacts (stars, names), starred messages, chat archive/mute/pin state, labels. The most active collection; updated on almost every user action.',
        critical_unblock_to_primary: 'Critical settings that must flow from a secondary device back to the primary (e.g. unblocking a contact initiated from a linked device).',
        critical_block:              'Blocked contacts list and spam reports. Kept separate so blocking changes propagate quickly without touching the broader critical collection.',
        regular_low:                 'Low-priority UI preferences (e.g. wallpaper, chat background, notification overrides). Deferred sync — may lag behind the other collections.',
      }[name] || 'Custom or unknown app state collection. May be introduced by a newer WhatsApp version.';
    },

    asCollectionBadgeClass(name) {
      return {
        critical:                    'bg-red-100    text-red-700    dark:bg-red-900/30    dark:text-red-400',
        regular:                     'bg-blue-100   text-blue-700   dark:bg-blue-900/30   dark:text-blue-400',
        critical_unblock_to_primary: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
        critical_block:              'bg-gray-100   text-gray-700   dark:bg-gray-800      dark:text-gray-400',
        regular_low:                 'bg-green-100  text-green-700  dark:bg-green-900/30  dark:text-green-400',
      }[name] || 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400';
    },

    // ── helpers ──
    asTsLabel(ts) {
      if (!ts) return '—';
      const d = new Date(ts * 1000);
      return isNaN(d) ? String(ts) : d.toLocaleString();
    },

    asHexShort(hex) {
      if (!hex || hex.length <= 16) return hex || '—';
      return hex.slice(0, 8) + '…' + hex.slice(-8);
    },
  };
}
