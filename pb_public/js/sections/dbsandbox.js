// DB Sandbox — execute read-only SQL queries against the WhatsApp SQLite database.
function dbSandboxSection() {
  return {
    // ── state ──
    sbSQL:     "SELECT name, type FROM sqlite_master WHERE type = 'table' ORDER BY name",
    sbLoading: false,
    sbError:   '',
    sbColumns: [],
    sbRows:    [],
    sbCount:   0,
    sbExecMs:  0,

    // quick-access example queries
    sbExamples: [
      { label: 'Tables',       sql: "SELECT name, type FROM sqlite_master WHERE type = 'table' ORDER BY name" },
      { label: 'Device',       sql: "SELECT * FROM whatsmeow_device LIMIT 1" },
      { label: 'Contacts',     sql: "SELECT our_jid, their_jid, push_name, full_name FROM whatsmeow_contacts ORDER BY push_name LIMIT 50" },
      { label: 'Sessions',     sql: "SELECT our_jid, their_id FROM whatsmeow_sessions ORDER BY their_id LIMIT 50" },
      { label: 'Message keys', sql: "SELECT * FROM whatsmeow_message_secrets ORDER BY rowid DESC LIMIT 20" },
      { label: 'Chat settings',sql: "SELECT * FROM whatsmeow_chat_settings WHERE pinned = 1 OR muted_until > 0" },
      { label: 'Pre-keys',     sql: "SELECT key_id, uploaded FROM whatsmeow_pre_keys ORDER BY key_id LIMIT 20" },
    ],

    // ── init ──
    initDBSandbox() {},

    // ── query ──
    async sbRun() {
      if (!this.sbSQL.trim() || this.sbLoading) return;
      this.sbLoading = true;
      this.sbError   = '';
      this.sbRows    = [];
      this.sbColumns = [];
      this.sbCount   = 0;
      this.sbExecMs  = 0;
      try {
        const res = await fetch('/zaplab/api/db/query', {
          method:  'POST',
          headers: { ...apiHeaders(), 'Content-Type': 'application/json' },
          body:    JSON.stringify({ sql: this.sbSQL }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Query failed');
        this.sbColumns = data.columns || [];
        this.sbRows    = data.rows    || [];
        this.sbCount   = data.count   || 0;
        this.sbExecMs  = data.exec_ms || 0;
      } catch (err) {
        this.sbError = err.message || 'Query failed';
      } finally {
        this.sbLoading = false;
      }
    },

    sbLoadExample(sql) {
      this.sbSQL    = sql;
      this.sbError  = '';
      this.sbRows   = [];
      this.sbColumns = [];
    },
  };
}
