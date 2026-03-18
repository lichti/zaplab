// Protobuf Schema Browser section — browse all WhatsApp proto types registered in the binary.
function protoSchemaSection() {
  return {
    // ── state ──
    psLoading:       false,
    psError:         '',
    psSchema:        null,      // full schema response from API
    psPackageFilter: '',        // selected package ('' = all)
    psSearch:        '',        // search query
    psSelected:      null,      // currently selected message/enum descriptor
    psSelectedKind:  '',        // 'message' or 'enum'
    psNavStack:      [],        // breadcrumb navigation stack [{kind, name}]
    psView:          'list',    // 'list' or 'detail'

    // ── init ──
    initProtoSchema() {
      // nothing — data loaded on demand when section is first shown
    },

    async psLoad() {
      if (this.psLoading || this.psSchema) return;
      this.psLoading = true;
      this.psError = '';
      try {
        const res = await fetch('/zaplab/api/proto/schema', {
          headers: apiHeaders(),
        });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        this.psSchema = await res.json();
      } catch (err) {
        this.psError = err.message || 'Failed to load proto schema';
      } finally {
        this.psLoading = false;
      }
    },

    async psReload() {
      this.psSchema = null;
      await this.psLoad();
    },

    // ── filtered lists ──
    psFilteredMessages() {
      if (!this.psSchema) return [];
      let msgs = this.psSchema.messages;
      if (this.psPackageFilter) msgs = msgs.filter(m => m.package === this.psPackageFilter);
      if (this.psSearch.trim()) {
        const q = this.psSearch.trim().toLowerCase();
        msgs = msgs.filter(m => m.full_name.toLowerCase().includes(q));
      }
      return msgs;
    },

    psFilteredEnums() {
      if (!this.psSchema) return [];
      let enums = this.psSchema.enums;
      if (this.psPackageFilter) enums = enums.filter(e => e.package === this.psPackageFilter);
      if (this.psSearch.trim()) {
        const q = this.psSearch.trim().toLowerCase();
        enums = enums.filter(e => e.full_name.toLowerCase().includes(q));
      }
      return enums;
    },

    psPackages() {
      return this.psSchema?.packages || [];
    },

    // ── navigation ──
    psSelectMessage(msg) {
      this.psSelected = msg;
      this.psSelectedKind = 'message';
      this.psView = 'detail';
      this.psNavStack = [{ kind: 'message', name: msg.full_name }];
    },

    psSelectEnum(en) {
      this.psSelected = en;
      this.psSelectedKind = 'enum';
      this.psView = 'detail';
      this.psNavStack = [{ kind: 'enum', name: en.full_name }];
    },

    psBackToList() {
      this.psView = 'list';
      this.psSelected = null;
      this.psNavStack = [];
    },

    // Navigate to a type reference from a field (message or enum type_ref)
    async psNavigateTo(typeRef, kind) {
      if (!typeRef || !this.psSchema) return;
      if (kind === 'message' || kind === undefined) {
        const found = this.psSchema.messages.find(m => m.full_name === typeRef);
        if (found) {
          this.psNavStack.push({ kind: 'message', name: typeRef });
          this.psSelected = found;
          this.psSelectedKind = 'message';
          return;
        }
      }
      if (kind === 'enum' || kind === undefined) {
        const found = this.psSchema.enums.find(e => e.full_name === typeRef);
        if (found) {
          this.psNavStack.push({ kind: 'enum', name: typeRef });
          this.psSelected = found;
          this.psSelectedKind = 'enum';
          return;
        }
      }
      // Not found locally — try fetching via API (for nested types not in top-level lists)
      try {
        const res = await fetch(`/zaplab/api/proto/message?name=${encodeURIComponent(typeRef)}`, {
          headers: apiHeaders(),
        });
        if (res.ok) {
          const msg = await res.json();
          this.psNavStack.push({ kind: 'message', name: typeRef });
          this.psSelected = msg;
          this.psSelectedKind = 'message';
        }
      } catch {}
    },

    async psNavBack() {
      if (this.psNavStack.length <= 1) {
        this.psBackToList();
        return;
      }
      this.psNavStack.pop();
      const prev = this.psNavStack[this.psNavStack.length - 1];
      if (prev.kind === 'message') {
        const found = this.psSchema?.messages.find(m => m.full_name === prev.name);
        if (found) { this.psSelected = found; this.psSelectedKind = 'message'; return; }
        // try API
        try {
          const res = await fetch(`/zaplab/api/proto/message?name=${encodeURIComponent(prev.name)}`, {
            headers: apiHeaders(),
          });
          if (res.ok) { this.psSelected = await res.json(); this.psSelectedKind = 'message'; }
        } catch {}
      } else {
        const found = this.psSchema?.enums.find(e => e.full_name === prev.name);
        if (found) { this.psSelected = found; this.psSelectedKind = 'enum'; }
      }
    },

    // ── helpers ──
    psShortName(fullName) {
      const parts = fullName.split('.');
      return parts.slice(1).join('.') || fullName;
    },

    psFieldTypeLabel(field) {
      if (field.type_ref) {
        return field.type_ref.split('.').slice(1).join('.') || field.type_ref;
      }
      return field.type;
    },

    psFieldTypeColor(field) {
      const scalarColors = {
        string: 'text-green-600 dark:text-green-400',
        bytes:  'text-yellow-600 dark:text-yellow-400',
        bool:   'text-orange-600 dark:text-orange-400',
        int32: 'text-blue-600 dark:text-blue-400',  int64: 'text-blue-600 dark:text-blue-400',
        uint32: 'text-blue-600 dark:text-blue-400', uint64: 'text-blue-600 dark:text-blue-400',
        sint32: 'text-blue-600 dark:text-blue-400', sint64: 'text-blue-600 dark:text-blue-400',
        fixed32: 'text-blue-600 dark:text-blue-400', fixed64: 'text-blue-600 dark:text-blue-400',
        sfixed32: 'text-blue-600 dark:text-blue-400', sfixed64: 'text-blue-600 dark:text-blue-400',
        float: 'text-cyan-600 dark:text-cyan-400', double: 'text-cyan-600 dark:text-cyan-400',
      };
      if (field.type === 'message' || field.type === 'group') return 'text-purple-600 dark:text-purple-400 cursor-pointer hover:underline';
      if (field.type === 'enum') return 'text-pink-600 dark:text-pink-400 cursor-pointer hover:underline';
      return scalarColors[field.type] || 'text-gray-600 dark:text-gray-400';
    },

    psIsNavigable(field) {
      return (field.type === 'message' || field.type === 'group' || field.type === 'enum') && !!field.type_ref;
    },

    // Generate apiHeaders helper (uses PocketBase auth token or API token)
    apiHeaders() {
      const headers = {};
      if (pb.authStore.token) {
        headers['Authorization'] = pb.authStore.token;
      } else if (this.apiToken) {
        headers['X-API-Token'] = this.apiToken;
      }
      return headers;
    },
  };
}
