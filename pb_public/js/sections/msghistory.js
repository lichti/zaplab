// Message History section — list edited and deleted messages stored in PocketBase.
// Filters for events where IsEdit:true (edited) or protocolMessage is present (revoked/deleted).
// On selection: shows the action event payload AND looks up the original message by its ID.
function msgHistorySection() {
  return {
    // ── state ──
    mh: {
      filterKind:     'all',  // 'all' | 'edit' | 'delete'
      filterSender:   '',
      filterChat:     '',
      filterDateFrom: '',
      filterDateTo:   '',
      items:          [],
      total:          0,
      page:           1,
      perPage:        50,
      loading:        false,
      selected:       null,
      origLoading:    false,
      origEvent:      null,     // original message record (looked up by targetId)
      origNotFound:   false,
      exporting:      false,
    },
    mhCopied:     false,
    mhOrigCopied: false,

    // ── init ──
    initMsgHistory() {},

    // ── filter helpers ──
    _mhEsc(s) {
      return String(s).replace(/\\/g, '\\\\').replace(/'/g, "\\'");
    },

    _mhBuildFilter() {
      const parts = ["type = 'Message'"];
      const kind  = this.mh.filterKind;
      if (kind === 'edit') {
        parts.push('raw ~ \'"IsEdit":true\'');
      } else if (kind === 'delete') {
        parts.push('raw ~ \'protocolMessage\'');
        parts.push('raw !~ \'"IsEdit":true\'');
      } else {
        // all: edited OR protocol message (revoke / delete)
        parts.push('(raw ~ \'"IsEdit":true\' || raw ~ \'protocolMessage\')');
      }
      if (this.mh.filterSender.trim())   parts.push(`raw ~ '${this._mhEsc(this.mh.filterSender.trim())}'`);
      if (this.mh.filterChat.trim())     parts.push(`raw ~ '${this._mhEsc(this.mh.filterChat.trim())}'`);
      if (this.mh.filterDateFrom.trim()) parts.push(`created >= '${this.mh.filterDateFrom.trim()} 00:00:00'`);
      if (this.mh.filterDateTo.trim())   parts.push(`created <= '${this.mh.filterDateTo.trim()} 23:59:59'`);
      return parts.join(' && ');
    },

    // ── list actions ──
    async mhSearch() {
      this.mh.loading      = true;
      this.mh.page         = 1;
      this.mh.selected     = null;
      this.mh.origEvent    = null;
      this.mh.origNotFound = false;
      try {
        const res = await pb.collection('events').getList(
          1, this.mh.perPage, { sort: '-created', filter: this._mhBuildFilter() }
        );
        this.mh.items = res.items;
        this.mh.total = res.totalItems;
      } catch (err) {
        console.error('mh search:', err);
      } finally {
        this.mh.loading = false;
      }
    },

    async mhLoadMore() {
      this.mh.loading = true;
      this.mh.page++;
      try {
        const res = await pb.collection('events').getList(
          this.mh.page, this.mh.perPage, { sort: '-created', filter: this._mhBuildFilter() }
        );
        this.mh.items = [...this.mh.items, ...res.items];
      } catch (err) {
        this.mh.page--;
        console.error('mh load more:', err);
      } finally {
        this.mh.loading = false;
      }
    },

    mhReset() {
      Object.assign(this.mh, {
        filterKind: 'all', filterSender: '', filterChat: '',
        filterDateFrom: '', filterDateTo: '',
        items: [], total: 0, page: 1,
        selected: null, origLoading: false, origEvent: null, origNotFound: false,
        exporting: false,
      });
      this.mhCopied     = false;
      this.mhOrigCopied = false;
    },

    async mhSelect(item) {
      this.mh.selected     = item;
      this.mh.origEvent    = null;
      this.mh.origNotFound = false;
      this.mhCopied        = false;
      this.mhOrigCopied    = false;

      const origId = this._mhOriginalId(item);
      if (!origId) {
        this.mh.origNotFound = true;
        return;
      }
      this.mh.origLoading = true;
      try {
        const res = await pb.collection('events').getList(1, 5, {
          sort:   '-created',
          filter: `msgID = '${this._mhEsc(origId)}'`,
        });
        if (res.items.length > 0) {
          this.mh.origEvent = res.items[0];
        } else {
          this.mh.origNotFound = true;
        }
      } catch {
        this.mh.origNotFound = true;
      } finally {
        this.mh.origLoading = false;
      }
    },

    mhHasMore() {
      return this.mh.items.length < this.mh.total;
    },

    // ── raw accessor ──
    _mhRaw(item) {
      try {
        const r = item?.raw;
        return typeof r === 'string' ? JSON.parse(r) : (r || {});
      } catch { return {}; }
    },

    // Returns the ID of the original message being edited or deleted.
    // whatsmeow stores it in Message.protocolMessage.key.ID for both REVOKE (type=0)
    // and MESSAGE_EDIT (type=14). Falls back to lowercase 'id' for robustness.
    _mhOriginalId(item) {
      const r   = this._mhRaw(item);
      const key = r?.Message?.protocolMessage?.key;
      if (!key) return null;
      return key.ID || key.Id || key.id || null;
    },

    // ── kind classification ──
    // 'edit'   — IsEdit:true or protocolMessage.type === 14 (MESSAGE_EDIT)
    // 'delete' — protocolMessage.type === 0 (REVOKE) or any other protocolMessage
    mhKind(item) {
      const r     = this._mhRaw(item);
      const proto = r?.Message?.protocolMessage;
      if (r?.IsEdit || (proto && proto.type === 14)) return 'edit';
      if (proto) return 'delete';
      return 'unknown';
    },

    mhKindLabel(kind) {
      if (kind === 'edit')   return 'Edited';
      if (kind === 'delete') return 'Deleted';
      return 'Unknown';
    },

    mhKindBadgeClass(kind) {
      if (kind === 'edit')   return 'bg-blue-100 text-blue-700 dark:bg-blue-500/20 dark:text-blue-300';
      if (kind === 'delete') return 'bg-red-100 text-red-700 dark:bg-red-500/20 dark:text-red-400';
      return 'bg-gray-100 text-gray-500 dark:bg-gray-500/20 dark:text-gray-400';
    },

    // ── list row helpers ──
    mhSenderLabel(item) {
      try {
        const r    = this._mhRaw(item);
        const name = r?.Info?.PushName;
        const jid  = r?.Info?.MessageSource?.Sender;
        if (name) return name;
        if (jid)  return String(typeof jid === 'object' ? (jid.User || '') : jid).split('@')[0];
        return '';
      } catch { return ''; }
    },

    mhChatLabel(item) {
      try {
        const r    = this._mhRaw(item);
        const chat = r?.Info?.MessageSource?.Chat || r?.Info?.Chat;
        if (!chat) return '';
        return String(typeof chat === 'object' ? (chat.User || '') : chat).split('@')[0];
      } catch { return ''; }
    },

    // ID of the message this action targets (the original that was edited/deleted)
    mhTargetId(item) {
      return this._mhOriginalId(item) || '';
    },

    // For edit events: extract the new (replacement) content from the edit payload
    mhNewContent(item) {
      try {
        const r     = this._mhRaw(item);
        const proto = r?.Message?.protocolMessage;
        const edited = proto?.editedMessage || r?.Message?.editedMessage;
        if (!edited) return null;
        const msg = edited.message || edited;
        return msg.conversation
          || msg.extendedTextMessage?.text
          || (msg.imageMessage    ? (msg.imageMessage.caption    || '[image]')    : null)
          || (msg.videoMessage    ? (msg.videoMessage.caption    || '[video]')    : null)
          || (msg.audioMessage    ? '[audio]'    : null)
          || (msg.documentMessage ? (msg.documentMessage.fileName || '[document]') : null)
          || null;
      } catch { return null; }
    },

    mhFmtDateTime(iso) {
      return new Date(iso).toLocaleString('en-GB', { hour12: false });
    },

    // ── word-level diff ──

    // Tokenize text into words and whitespace runs for word-level diffing.
    _mhTokenize(text) {
      return String(text).match(/\S+|\s+/g) || [];
    },

    // LCS-based diff between two token arrays.
    // Returns an array of { type: 'eq'|'del'|'ins', val } operations.
    _mhLCS(a, b) {
      const m = a.length, n = b.length;
      // Flat DP table (row-major) for memory efficiency
      const dp  = new Int32Array((m + 1) * (n + 1));
      const row = n + 1;
      for (let i = 1; i <= m; i++) {
        for (let j = 1; j <= n; j++) {
          dp[i * row + j] = a[i - 1] === b[j - 1]
            ? dp[(i - 1) * row + (j - 1)] + 1
            : Math.max(dp[(i - 1) * row + j], dp[i * row + (j - 1)]);
        }
      }
      // Backtrack
      const ops = [];
      let i = m, j = n;
      while (i > 0 || j > 0) {
        if (i > 0 && j > 0 && a[i - 1] === b[j - 1]) {
          ops.unshift({ type: 'eq',  val: a[i - 1] }); i--; j--;
        } else if (j > 0 && (i === 0 || dp[i * row + (j - 1)] >= dp[(i - 1) * row + j])) {
          ops.unshift({ type: 'ins', val: b[j - 1] }); j--;
        } else {
          ops.unshift({ type: 'del', val: a[i - 1] }); i--;
        }
      }
      return ops;
    },

    // Returns syntax-highlighted HTML showing word-level diff between the
    // original message text and the edited (new) content.
    // Returns null when either side is unavailable.
    mhDiffHtml(item) {
      const newText  = this.mhNewContent(item);
      const origText = this.mh.origEvent ? this.ebPreviewText(this.mh.origEvent) : null;
      if (!origText || !newText) return null;

      const esc = s => String(s)
        .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

      if (origText === newText) return esc(origText);

      const tokOld = this._mhTokenize(origText);
      const tokNew = this._mhTokenize(newText);

      // Fall back to block diff for very long texts (avoid O(n²) freeze)
      if (tokOld.length > 400 || tokNew.length > 400) {
        return `<span class="diff-del">${esc(origText)}</span>\n<span class="diff-ins">${esc(newText)}</span>`;
      }

      return this._mhLCS(tokOld, tokNew).map(op => {
        const e = esc(op.val);
        if (op.type === 'del') return `<span class="diff-del">${e}</span>`;
        if (op.type === 'ins') return `<span class="diff-ins">${e}</span>`;
        return e;
      }).join('');
    },

    // ── CSV export ──

    // Exports all records matching the current filter as a CSV file (up to 1 000 rows).
    async mhExportCSV() {
      this.mh.exporting = true;
      try {
        const filter = this._mhBuildFilter();
        const opts   = { sort: '-created', requestKey: null };
        if (filter) opts.filter = filter;

        const perPage = 200;
        let page = 1, all = [];
        while (all.length < 1000) {
          const res = await pb.collection('events').getList(page, perPage, opts);
          all = all.concat(res.items);
          if (all.length >= res.totalItems || res.items.length < perPage) break;
          page++;
        }
        all = all.slice(0, 1000);

        const esc = v => {
          const s = v == null ? '' : String(v);
          return s.includes(',') || s.includes('"') || s.includes('\n')
            ? '"' + s.replace(/"/g, '""') + '"'
            : s;
        };

        const headers = ['id', 'kind', 'msgID', 'created', 'sender', 'chat', 'targetID', 'newContent'];
        const rows = all.map(item => {
          const kind       = this.mhKind(item);
          const sender     = this.mhSenderLabel(item);
          const chat       = this.mhChatLabel(item);
          const targetID   = this.mhTargetId(item);
          const newContent = this.mhNewContent(item) || '';
          return [item.id, kind, item.msgID || '', item.created, sender, chat, targetID, newContent]
            .map(esc).join(',');
        });

        const csv  = [headers.join(','), ...rows].join('\r\n');
        const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
        const url  = URL.createObjectURL(blob);
        const a    = document.createElement('a');
        a.href     = url;
        a.download = 'msg_history_export.csv';
        a.click();
        URL.revokeObjectURL(url);
      } catch (err) {
        console.error('mh export csv:', err);
      } finally {
        this.mh.exporting = false;
      }
    },

    // ── copy helpers ──
    async mhCopyJSON() {
      if (!this.mh.selected) return;
      try {
        await navigator.clipboard.writeText(JSON.stringify(this.mh.selected, null, 2));
        this.mhCopied = true;
        setTimeout(() => { this.mhCopied = false; }, 2000);
      } catch {}
    },

    async mhCopyOrigJSON() {
      if (!this.mh.origEvent) return;
      try {
        await navigator.clipboard.writeText(JSON.stringify(this.mh.origEvent, null, 2));
        this.mhOrigCopied = true;
        setTimeout(() => { this.mhOrigCopied = false; }, 2000);
      } catch {}
    },
  };
}
