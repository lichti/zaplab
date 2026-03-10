// Error Browser section — search and inspect errors stored in the PocketBase errors collection.
function errorBrowserSection() {
  return {
    // ── state ──
    errbr: {
      filterType:    '',
      filterText:    '',
      filterDateFrom:'',
      filterDateTo:  '',
      items:    [],
      total:    0,
      page:     1,
      perPage:  50,
      loading:  false,
      selected: null,
      exporting:false,
    },
    errbrCopied: false,

    // ── init ──
    initErrorBrowser() {},

    // ── filter helpers ──
    _errbrEsc(s) {
      return String(s).replace(/\\/g, '\\\\').replace(/'/g, "\\'");
    },

    _errbrBuildFilter() {
      const parts = [];
      if (this.errbr.filterType.trim())     parts.push(`type = '${this._errbrEsc(this.errbr.filterType.trim())}'`);
      if (this.errbr.filterText.trim())     parts.push(`(raw ~ '${this._errbrEsc(this.errbr.filterText.trim())}' || EvtError ~ '${this._errbrEsc(this.errbr.filterText.trim())}')`);
      if (this.errbr.filterDateFrom.trim()) parts.push(`created >= '${this.errbr.filterDateFrom.trim()} 00:00:00'`);
      if (this.errbr.filterDateTo.trim())   parts.push(`created <= '${this.errbr.filterDateTo.trim()} 23:59:59'`);
      return parts.join(' && ');
    },

    async errbrSearch() {
      this.errbr.loading  = true;
      this.errbr.page     = 1;
      this.errbr.selected = null;
      this.errbrCopied    = false;
      try {
        const res = await pb.collection('errors').getList(
          1, this.errbr.perPage, { sort: '-created', filter: this._errbrBuildFilter() }
        );
        this.errbr.items = res.items;
        this.errbr.total = res.totalItems;
      } catch (err) {
        console.error('errbr search:', err);
      } finally {
        this.errbr.loading = false;
      }
    },

    async errbrLoadMore() {
      this.errbr.loading = true;
      this.errbr.page++;
      try {
        const res = await pb.collection('errors').getList(
          this.errbr.page, this.errbr.perPage, { sort: '-created', filter: this._errbrBuildFilter() }
        );
        this.errbr.items = [...this.errbr.items, ...res.items];
      } catch (err) {
        this.errbr.page--;
        console.error('errbr load more:', err);
      } finally {
        this.errbr.loading = false;
      }
    },

    errbrReset() {
      Object.assign(this.errbr, {
        filterType: '', filterText: '', filterDateFrom: '', filterDateTo: '',
        items: [], total: 0, page: 1, selected: null, exporting: false,
      });
      this.errbrCopied = false;
    },

    errbrSelect(item) {
      this.errbr.selected = item;
      this.errbrCopied    = false;
    },

    errbrHasMore() {
      return this.errbr.items.length < this.errbr.total;
    },

    // ── raw accessor ──
    _errbrRaw(item) {
      try {
        const r = item?.raw;
        return typeof r === 'string' ? JSON.parse(r) : (r || {});
      } catch { return {}; }
    },

    // ── preview helpers ──
    errbrFmtDateTime(iso) {
      return new Date(iso).toLocaleString('en-GB', { hour12: false });
    },

    errbrErrorLabel(item) {
      return item?.EvtError || item?.type || '';
    },

    errbrPreviewText(item) {
      try {
        const r = this._errbrRaw(item);
        return r?.Error?.message || r?.message || r?.error || item?.EvtError || '';
      } catch { return ''; }
    },

    // ── copy ──
    async errbrCopyJSON() {
      if (!this.errbr.selected) return;
      try {
        await navigator.clipboard.writeText(JSON.stringify(this.errbr.selected, null, 2));
        this.errbrCopied = true;
        setTimeout(() => { this.errbrCopied = false; }, 2000);
      } catch {}
    },

    // ── CSV export ──
    async errbrExportCSV() {
      this.errbr.exporting = true;
      try {
        const filter = this._errbrBuildFilter();
        const opts   = { sort: '-created', requestKey: null };
        if (filter) opts.filter = filter;

        const perPage = 200;
        let page = 1, all = [];
        while (all.length < 1000) {
          const res = await pb.collection('errors').getList(page, perPage, opts);
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

        const headers = ['id', 'type', 'EvtError', 'created', 'preview'];
        const rows = all.map(item => {
          const preview = this.errbrPreviewText(item);
          return [item.id, item.type || '', item.EvtError || '', item.created, preview]
            .map(esc).join(',');
        });

        const csv  = [headers.join(','), ...rows].join('\r\n');
        const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
        const url  = URL.createObjectURL(blob);
        const a    = document.createElement('a');
        a.href     = url;
        a.download = 'errors_export.csv';
        a.click();
        URL.revokeObjectURL(url);
      } catch (err) {
        console.error('errbr export csv:', err);
      } finally {
        this.errbr.exporting = false;
      }
    },
  };
}
