// Shared display helpers — used across all sections.
function utilsSection() {
  return {
    fmtTime(iso) {
      return new Date(iso).toLocaleTimeString('en-GB', { hour12: false });
    },

    typeClass(type) {
      if (!type) return 'bg-gray-100 text-gray-500 dark:bg-[#21262d] dark:text-[#8b949e]';
      const t = type.toLowerCase();
      if (t.includes('sentmessage'))  return 'bg-green-100 text-green-700 dark:bg-[#1d2d1c] dark:text-[#3fb950]';
      if (t.includes('message'))      return 'bg-blue-100  text-blue-700  dark:bg-[#1c3a5e] dark:text-[#58a6ff]';
      if (t.includes('receipt'))      return 'bg-green-100 text-green-700 dark:bg-[#1b3a2e] dark:text-[#3fb950]';
      if (t.includes('connected'))    return 'bg-green-100 text-green-700 dark:bg-[#1b3a2e] dark:text-[#3fb950]';
      if (t.includes('disconnect'))   return 'bg-red-100   text-red-700   dark:bg-[#3a1c1c] dark:text-[#f85149]';
      if (t.includes('history'))      return 'bg-orange-100 text-orange-700 dark:bg-[#2d2416] dark:text-[#ffa657]';
      if (t.includes('presence'))     return 'bg-purple-100 text-purple-700 dark:bg-[#261d3a] dark:text-[#bc8cff]';
      return 'bg-gray-100 text-gray-500 dark:bg-[#21262d] dark:text-[#8b949e]';
    },

    previewText(record) {
      try {
        const raw = record.raw;
        if (!raw) return '';
        if (typeof raw === 'object') {
          return Object.keys(raw).slice(0, 3)
            .map(k => `${k}: ${JSON.stringify(raw[k])}`)
            .join('   ');
        }
        return String(raw).slice(0, 140);
      } catch { return ''; }
    },

    highlight(obj) {
      const { _isNew, ...clean } = obj;
      const json = JSON.stringify(clean, null, 2)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
      return json.replace(
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

    escapeHtml(str) {
      return str
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
    },

    // Single-pass replace: sequential replaces would corrupt previously
    // inserted <span class="..."> tags when later regexes match "..." again.
    highlightCurl(str) {
      return this.escapeHtml(str).replace(
        /(^#.*$)|(\bcurl\b)|( )(-X|-H|-d)(?= )|(https?:\/\/[^\s\\]+)|"([^"]*)"|'([^']*)'|(\\$)/gm,
        (m, comment, curl, sp, flag, url, dq, sq, bs) => {
          if (comment !== undefined) return `<span class="cm">${comment}</span>`;
          if (curl)    return `<span class="ck">${curl}</span>`;
          if (flag)    return `${sp}<span class="ck">${flag}</span>`;
          if (url)     return `<span class="cu">${url}</span>`;
          if (dq !== undefined) return `"<span class="cq">${dq}</span>"`;
          if (sq !== undefined) return `'<span class="cq">${sq}</span>'`;
          if (bs)      return `<span class="cm">\\</span>`;
          return m;
        }
      );
    },
  };
}
