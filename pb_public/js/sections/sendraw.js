// Send Raw section — state, init, preview helpers, submit.
function sendRawSection() {
  return {
    // ── state ──
    raw: {
      to:      '',
      json:    '{\n  "conversation": "Hello!"\n}',
      loading: false,
      toast:   null,
      result:  null,
      valid:   true,
      error:   '',
    },
    rawPreviewTab:    localStorage.getItem('zaplab-raw-preview-tab') || 'curl',
    rawPreviewCopied: false,

    // ── section init ──
    initSendRaw() {
      this.$watch('rawPreviewTab', val => {
        localStorage.setItem('zaplab-raw-preview-tab', val);
      });
    },

    // ── methods ──
    rawJsonHighlight() {
      const escaped = this.escapeHtml(this.raw.json);
      try {
        JSON.parse(this.raw.json);
        this.raw.valid = true;
        this.raw.error = '';
      } catch (e) {
        this.raw.valid = false;
        this.raw.error = e.message;
        return escaped;
      }
      return escaped.replace(
        /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,
        m => {
          if (/^"/.test(m)) return /:$/.test(m)
            ? `<span class="jk">${m}</span>`
            : `<span class="js">${m}</span>`;
          if (/true|false/.test(m)) return `<span class="jb">${m}</span>`;
          if (/null/.test(m))       return `<span class="jl">${m}</span>`;
          return `<span class="jn">${m}</span>`;
        }
      );
    },

    rawCurlPreview() {
      const token = this.apiToken || '<your-api-token>';
      const url   = `${window.location.origin}/sendraw`;
      let payload;
      try {
        const msg = JSON.parse(this.raw.json);
        payload = JSON.stringify({ to: this.raw.to || '<to>', message: msg });
      } catch {
        payload = `{"to":"${this.raw.to || '<to>'}","message":<invalid json>}`;
      }
      return [
        `# auth disabled — X-API-Token not required`,
        `curl -X POST \\`,
        `  ${url} \\`,
        `  -H "Content-Type: application/json" \\`,
        `  -H "X-API-Token: ${token}" \\`,
        `  -d '${payload}'`,
      ].join('\n');
    },

    rawResultPreview() {
      if (!this.raw.result) {
        return '<span style="color:var(--tw-prose-captions,#8b949e)">No response yet — send a raw message first.</span>';
      }
      return this.highlight(this.raw.result);
    },

    rawPreviewContent() {
      if (this.rawPreviewTab === 'response') return this.rawResultPreview();
      return this.highlightCurl(this.rawCurlPreview());
    },

    insertTab(event) {
      const ta    = event.target;
      const start = ta.selectionStart;
      const end   = ta.selectionEnd;
      ta.value    = ta.value.slice(0, start) + '  ' + ta.value.slice(end);
      ta.selectionStart = ta.selectionEnd = start + 2;
      this.raw.json = ta.value;
    },

    async copyRawPreview() {
      const text = this.rawPreviewTab === 'response'
        ? JSON.stringify(this.raw.result, null, 2)
        : this.rawCurlPreview();
      try {
        await navigator.clipboard.writeText(text);
        this.rawPreviewCopied = true;
        setTimeout(() => { this.rawPreviewCopied = false; }, 2000);
      } catch {}
    },

    async submitRaw() {
      this.raw.toast   = null;
      this.raw.loading = true;
      try {
        const msg = JSON.parse(this.raw.json);
        const res = await fetch('/sendraw', {
          method:  'POST',
          headers: { 'Content-Type': 'application/json', 'X-API-Token': this.apiToken },
          body:    JSON.stringify({ to: this.raw.to, message: msg }),
        });
        const data = await res.json();
        this.raw.toast = { ok: res.ok, message: data.message || JSON.stringify(data) };
        if (res.ok) {
          this.raw.result    = data;
          this.rawPreviewTab = 'response';
        }
      } catch (err) {
        this.raw.toast = { ok: false, message: err.message };
      } finally {
        this.raw.loading = false;
      }
    },
  };
}
