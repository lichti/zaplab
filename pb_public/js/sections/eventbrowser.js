// Event Browser section — search, filter, inspect, replay events stored in PocketBase.
function eventBrowserSection() {
  return {
    // ── state ──
    eb: {
      filterType:      '',
      filterDateFrom:  '',
      filterDateTo:    '',
      filterText:      '',
      filterMsgID:     '',
      filterSender:    '',
      filterRecipient: '',
      items:    [],
      total:    0,
      page:     1,
      perPage:  50,
      loading:  false,
      selected: null,
      replayTo:      '',
      replayLoading: false,
      replayToast:   null,
      exporting:     false,
    },
    ebCopied:      false,
    ebFiltersOpen: true,

    ebKnownTypes: [
      'Message', 'Message.TextMessage', 'Message.ExtendedTextMessage',
      'Message.ImageMessage', 'Message.VideoMessage', 'Message.AudioMessage',
      'Message.DocumentMessage', 'Message.StickerMessage', 'Message.LocationMessage',
      'Message.LiveLocationMessage', 'Message.ContactMessage', 'Message.ContactsArrayMessage',
      'Message.PollCreationMessage', 'Message.PollUpdateMessage', 'Message.ReactionMessage',
      'Message.RevokeMessage', 'Message.EditMessage', 'Message.ProtocolMessage',
      'Message.GroupUpdate', 'SentMessage', 'SimulationLocationUpdate', 'SimulationStopped',
      'HistorySync', 'Receipt', 'Presence', 'Connected', 'Disconnected',
    ],

    // ── init ──
    initEventBrowser() {},

    // ── filter helpers ──
    _ebEsc(s) {
      return String(s).replace(/\\/g, '\\\\').replace(/'/g, "\\'");
    },

    _ebBuildFilter() {
      const parts = [];
      const t = this.eb.filterType.trim();
      if (t)                              parts.push(`type = '${this._ebEsc(t)}'`);
      if (this.eb.filterDateFrom.trim())  parts.push(`created >= '${this.eb.filterDateFrom.trim()} 00:00:00'`);
      if (this.eb.filterDateTo.trim())    parts.push(`created <= '${this.eb.filterDateTo.trim()} 23:59:59'`);
      if (this.eb.filterMsgID.trim())     parts.push(`msgID = '${this._ebEsc(this.eb.filterMsgID.trim())}'`);
      if (this.eb.filterText.trim())      parts.push(`raw ~ '${this._ebEsc(this.eb.filterText.trim())}'`);
      if (this.eb.filterSender.trim())    parts.push(`raw ~ '${this._ebEsc(this.eb.filterSender.trim())}'`);
      if (this.eb.filterRecipient.trim()) parts.push(`raw ~ '${this._ebEsc(this.eb.filterRecipient.trim())}'`);
      return parts.join(' && ');
    },

    async ebSearch() {
      this.eb.loading  = true;
      this.eb.page     = 1;
      this.eb.selected = null;
      try {
        const res = await pb.collection('events').getList(
          1, this.eb.perPage, { sort: '-created', filter: this._ebBuildFilter() }
        );
        this.eb.items = res.items;
        this.eb.total = res.totalItems;
      } catch (err) {
        console.error('eb search:', err);
      } finally {
        this.eb.loading = false;
      }
    },

    async ebLoadMore() {
      this.eb.loading = true;
      this.eb.page++;
      try {
        const res = await pb.collection('events').getList(
          this.eb.page, this.eb.perPage, { sort: '-created', filter: this._ebBuildFilter() }
        );
        this.eb.items = [...this.eb.items, ...res.items];
      } catch (err) {
        this.eb.page--;
        console.error('eb load more:', err);
      } finally {
        this.eb.loading = false;
      }
    },

    ebReset() {
      Object.assign(this.eb, {
        filterType: '', filterDateFrom: '', filterDateTo: '',
        filterText: '', filterMsgID:    '', filterSender: '', filterRecipient: '',
        items: [], total: 0, page: 1, selected: null,
        replayTo: '', replayLoading: false, replayToast: null,
      });
      this.ebCopied = false;
    },

    ebSelect(item) {
      this.eb.selected    = item;
      this.eb.replayTo    = '';
      this.eb.replayToast = null;
      this.ebCopied       = false;
    },

    ebHasMore() {
      return this.eb.items.length < this.eb.total;
    },

    // ── raw accessor ──
    _ebRaw(item) {
      try {
        const r = item.raw;
        return typeof r === 'string' ? JSON.parse(r) : (r || {});
      } catch { return {}; }
    },

    // ── list preview helpers ──
    ebPreviewText(item) {
      try {
        const r   = this._ebRaw(item);
        const msg = r?.Message || r?.message;
        if (msg) {
          return msg.conversation
            || msg.extendedTextMessage?.text
            || (msg.imageMessage    ? (msg.imageMessage.caption    || '[image]')    : null)
            || (msg.videoMessage    ? (msg.videoMessage.caption    || '[video]')    : null)
            || (msg.audioMessage    ? '[audio]'    : null)
            || (msg.documentMessage ? (msg.documentMessage.fileName || '[document]') : null)
            || (msg.stickerMessage  ? '[sticker]'  : null)
            || (msg.locationMessage ? '[location]' : null)
            || (msg.pollCreationMessage ? '[poll: ' + msg.pollCreationMessage.name + ']' : null)
            || (msg.reactionMessage ? '[reaction: ' + msg.reactionMessage.text + ']' : null)
            || (msg.contactMessage  ? '[contact: ' + (msg.contactMessage.displayName || '') + ']' : null)
            || JSON.stringify(msg).slice(0, 80);
        }
        if (r?.Info?.PushName) return 'from ' + r.Info.PushName;
        return JSON.stringify(r).slice(0, 80);
      } catch { return ''; }
    },

    ebSenderLabel(item) {
      try {
        const r = this._ebRaw(item);
        const name = r?.Info?.PushName;
        const jid  = r?.Info?.MessageSource?.Sender;
        if (name) return name;
        if (jid)  return jid.split('@')[0];
        return '';
      } catch { return ''; }
    },

    ebFmtDateTime(iso) {
      return new Date(iso).toLocaleString('en-GB', { hour12: false });
    },

    // ── file / media ──
    ebFileUrl(item) {
      if (!item?.file) return '';
      return `${window.location.origin}/api/files/events/${item.id}/${item.file}`;
    },

    ebThumbUrl(item) {
      if (!item?.file) return '';
      return `${window.location.origin}/api/files/events/${item.id}/${item.file}?thumb=300x300`;
    },

    ebMediaType(item) {
      if (!item?.file) return null;
      const ext = item.file.split('.').pop().toLowerCase();
      if (['jpg','jpeg','png','gif','webp'].includes(ext)) return 'image';
      if (['mp4','webm','mov','mkv'].includes(ext))        return 'video';
      if (['mp3','ogg','opus','wav','m4a','aac'].includes(ext)) return 'audio';
      return 'file';
    },

    // ── detail / copy ──
    async ebCopyJSON() {
      if (!this.eb.selected) return;
      try {
        await navigator.clipboard.writeText(JSON.stringify(this.eb.selected, null, 2));
        this.ebCopied = true;
        setTimeout(() => { this.ebCopied = false; }, 2000);
      } catch {}
    },

    // ── CSV export ──

    // Export all records matching the current filter as a CSV file (up to 1 000 rows).
    async ebExportCSV() {
      this.eb.exporting = true;
      try {
        const filter = this._ebBuildFilter();
        const opts   = { sort: '-created', requestKey: null };
        if (filter) opts.filter = filter;

        // Fetch all pages up to a hard limit of 1 000 records
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

        const headers = ['id', 'type', 'msgID', 'created', 'sender', 'chat', 'preview', 'file'];
        const rows = all.map(item => {
          const r       = this._ebRaw(item);
          const sender  = this.ebSenderLabel(item);
          const chat    = (() => {
            try {
              const c = r?.Info?.MessageSource?.Chat || r?.Info?.Chat;
              if (!c) return '';
              return String(typeof c === 'object' ? (c.User || '') : c).split('@')[0];
            } catch { return ''; }
          })();
          const preview = this.ebPreviewText(item);
          return [item.id, item.type, item.msgID || '', item.created, sender, chat, preview, item.file || '']
            .map(esc).join(',');
        });

        const csv  = [headers.join(','), ...rows].join('\r\n');
        const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
        const url  = URL.createObjectURL(blob);
        const a    = document.createElement('a');
        a.href     = url;
        a.download = 'events_export.csv';
        a.click();
        URL.revokeObjectURL(url);
      } catch (err) {
        console.error('eb export csv:', err);
      } finally {
        this.eb.exporting = false;
      }
    },

    // ── replay ──
    _ebReplayMessage() {
      if (!this.eb.selected) return null;
      try {
        const r = this._ebRaw(this.eb.selected);
        return r?.Message || r?.message || r?.whatsapp_message || null;
      } catch { return null; }
    },

    ebReplayHighlight() {
      const msg = this._ebReplayMessage();
      if (!msg) return '<span style="color:var(--tw-prose-captions,#8b949e)">No Message field found in this event\'s raw data.</span>';
      return this.highlight(msg);
    },

    async ebReplay() {
      const msg = this._ebReplayMessage();
      if (!msg || !this.eb.replayTo.trim()) return;
      this.eb.replayLoading = true;
      this.eb.replayToast   = null;
      try {
        const res  = await fetch('/zaplab/api/sendraw', {
          method:  'POST',
          headers: { 'Content-Type': 'application/json', 'X-API-Token': this.apiToken },
          body:    JSON.stringify({ to: this.eb.replayTo.trim(), message: msg }),
        });
        const data = await res.json();
        this.eb.replayToast = { ok: res.ok, message: data.message || JSON.stringify(data) };
      } catch (err) {
        this.eb.replayToast = { ok: false, message: err.message };
      } finally {
        this.eb.replayLoading = false;
      }
    },
  };
}
