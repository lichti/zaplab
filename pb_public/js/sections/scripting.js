// Plugin System / Scripting Engine
// Provides a JavaScript sandbox (goja) for scripting WhatsApp automation.
// Scripts are persisted in the `scripts` PocketBase collection.
// Available sandbox APIs: console.log, wa.sendText, wa.status, http.get,
//   http.post, db.query, sleep.
function scriptingSection() {
  return {
    // ── state ──
    scLoading:    false,
    scError:      '',
    scScripts:    [],
    scSelected:   null,   // script being edited
    scRunning:    false,
    scRunOutput:  '',
    scRunError:   '',
    scRunDurationMs: 0,
    scRunStatus:  '',
    // adhoc editor
    scAdhocCode:  '// Type JavaScript here and click Run\nconsole.log("Hello from zaplab!");\nconsole.log("WA status:", wa.status());',
    scAdhocRunning: false,
    scAdhocOutput:  '',
    scAdhocError:   '',
    scAdhocStatus:  '',
    scAdhocDurationMs: 0,
    // new script form
    scNewName:    '',
    scNewDesc:    '',
    scNewCode:    '// Script code\nconsole.log("Hello!");',
    scNewTimeout: 10,
    scShowNew:    false,

    // ── example scripts ──
    scExamples: [
      {
        name: 'WA Connection Status',
        description: 'Check current WhatsApp connection status',
        code: `// Check WhatsApp connection status
const status = wa.status();
console.log("Connection status:", status);`,
      },
      {
        name: 'List DB Tables',
        description: 'Show all SQLite tables in the database',
        code: `// List all tables in the PocketBase SQLite database
const rows = db.query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name");
rows.forEach(r => console.log(r.name));
console.log("Total tables:", rows.length);`,
      },
      {
        name: 'Recent Events',
        description: 'Show the 10 most recent stored events',
        code: `// List the 10 most recent events (all types)
const rows = db.query(\`
  SELECT id, type, msgID, created
  FROM events
  ORDER BY created DESC
  LIMIT 10
\`);
rows.forEach(r => console.log(r.created, "|", r.type, "|", r.msgID || '—'));
console.log("Total rows:", rows.length);`,
      },
      {
        name: 'Event Types Count',
        description: 'Count events grouped by type',
        code: `// Count events grouped by type
const rows = db.query(\`
  SELECT type, COUNT(*) as cnt
  FROM events
  GROUP BY type
  ORDER BY cnt DESC
\`);
rows.forEach(r => console.log(r.cnt, "×", r.type));`,
      },
      {
        name: 'Messages per Chat',
        description: 'Count messages per chat using json_extract (top 10)',
        code: `// Count Message events per chat using SQLite json_extract
const rows = db.query(\`
  SELECT json_extract(raw, '$.Info.Chat') AS chat,
         COUNT(*) AS cnt
  FROM events
  WHERE type = 'Message'
  GROUP BY chat
  ORDER BY cnt DESC
  LIMIT 10
\`);
rows.forEach(r => console.log(r.cnt, "msgs |", r.chat));`,
      },
      {
        name: 'Send Test Message',
        description: 'Send a WhatsApp message to a JID (edit JID first)',
        code: `// Send a test message — replace JID with a real number
const jid = "5511999999999@s.whatsapp.net";
const text = "Hello from zaplab scripting! 🚀";
wa.sendText(jid, text);
console.log("Sent to:", jid);`,
      },
      {
        name: 'HTTP GET Example',
        description: 'Fetch data from an external URL',
        code: `// Make an outbound HTTP GET request
const res = http.get("https://httpbin.org/get");
console.log("Status:", res.status);
const data = JSON.parse(res.body);
console.log("Origin IP:", data.origin);
console.log("User-Agent:", data.headers["User-Agent"]);`,
      },
      {
        name: 'HTTP POST Webhook',
        description: 'Send a JSON payload to a webhook URL',
        code: `// POST a JSON payload to a webhook (edit URL)
const url = "https://httpbin.org/post";
const payload = JSON.stringify({
  event: "zaplab-test",
  status: wa.status(),
  ts: new Date().toISOString(),
});
const res = http.post(url, payload);
console.log("Response status:", res.status);
console.log(res.body.substring(0, 200));`,
      },
      {
        name: 'Recent Messages (parsed)',
        description: 'Show last 10 Message events with chat and sender extracted',
        code: `// Last 10 Message events with chat/sender from raw JSON
const rows = db.query(\`
  SELECT msgID,
         json_extract(raw, '$.Info.Chat')     AS chat,
         json_extract(raw, '$.Info.Sender')   AS sender,
         json_extract(raw, '$.Info.IsFromMe') AS from_me,
         created
  FROM events
  WHERE type = 'Message'
  ORDER BY created DESC
  LIMIT 10
\`);
rows.forEach(r => {
  const dir = r.from_me === '1' ? '→' : '←';
  console.log(r.created, dir, r.chat, "|", r.sender);
});`,
      },
      {
        name: 'Sleep & Loop',
        description: 'Demonstrate sleep and iteration',
        code: `// Sleep between iterations (max 5000 ms per call)
for (let i = 1; i <= 3; i++) {
  console.log("Step", i, "- status:", wa.status());
  sleep(500);
}
console.log("Done.");`,
      },
    ],

    scLoadExample(ex) {
      this.scAdhocCode = ex.code;
    },

    // ── init ──
    initScripting() {
      this.$watch('activeSection', val => {
        if (val === 'scripting' && this.scScripts.length === 0) this.scLoad();
      });
    },

    // ── load ──
    async scLoad() {
      if (this.scLoading) return;
      this.scLoading = true;
      this.scError   = '';
      try {
        const res = await fetch('/zaplab/api/scripts', { headers: apiHeaders() });
        if (res.ok) {
          const d = await res.json();
          this.scScripts = d.scripts || [];
        } else {
          this.scError = `HTTP ${res.status}`;
        }
      } catch (err) {
        this.scError = err.message || 'Load failed';
      } finally {
        this.scLoading = false;
      }
    },

    // ── create ──
    async scCreate() {
      if (!this.scNewName.trim()) return;
      try {
        const res = await fetch('/zaplab/api/scripts', {
          method: 'POST',
          headers: { ...apiHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name:         this.scNewName.trim(),
            description:  this.scNewDesc.trim(),
            code:         this.scNewCode,
            enabled:      true,
            timeout_secs: this.scNewTimeout || 10,
          }),
        });
        if (res.ok) {
          const d = await res.json();
          this.scScripts.unshift(d);
          this.scNewName = '';
          this.scNewDesc = '';
          this.scNewCode = '// Script code\nconsole.log("Hello!");';
          this.scNewTimeout = 10;
          this.scShowNew = false;
          this.scSelected = d;
        } else {
          const d = await res.json();
          this.scError = d.message || `HTTP ${res.status}`;
        }
      } catch (err) {
        this.scError = err.message || 'Create failed';
      }
    },

    // ── update ──
    async scSave(script) {
      if (!script) return;
      try {
        const res = await fetch(`/zaplab/api/scripts/${script.id}`, {
          method: 'PATCH',
          headers: { ...apiHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name:         script.name,
            description:  script.description,
            code:         script.code,
            enabled:      script.enabled,
            timeout_secs: script.timeout_secs,
          }),
        });
        if (res.ok) {
          const d = await res.json();
          const idx = this.scScripts.findIndex(s => s.id === d.id);
          if (idx >= 0) this.scScripts[idx] = d;
        } else {
          const d = await res.json();
          this.scError = d.message || `HTTP ${res.status}`;
        }
      } catch (err) {
        this.scError = err.message || 'Save failed';
      }
    },

    // ── delete ──
    async scDelete(script) {
      if (!script) return;
      if (!confirm(`Delete script "${script.name}"?`)) return;
      try {
        const res = await fetch(`/zaplab/api/scripts/${script.id}`, {
          method: 'DELETE',
          headers: apiHeaders(),
        });
        if (res.ok) {
          this.scScripts = this.scScripts.filter(s => s.id !== script.id);
          if (this.scSelected && this.scSelected.id === script.id) this.scSelected = null;
        } else {
          this.scError = `HTTP ${res.status}`;
        }
      } catch (err) {
        this.scError = err.message || 'Delete failed';
      }
    },

    // ── run persisted script ──
    async scRun(script) {
      if (!script || this.scRunning) return;
      this.scRunning    = true;
      this.scRunOutput  = '';
      this.scRunError   = '';
      this.scRunStatus  = '';
      this.scRunDurationMs = 0;
      try {
        const res = await fetch(`/zaplab/api/scripts/${script.id}/run`, {
          method: 'POST',
          headers: apiHeaders(),
        });
        const d = await res.json();
        this.scRunStatus      = d.status || '';
        this.scRunOutput      = d.output || '';
        this.scRunError       = d.error  || '';
        this.scRunDurationMs  = d.duration_ms || 0;
        // refresh list entry
        const idx = this.scScripts.findIndex(s => s.id === script.id);
        if (idx >= 0) {
          this.scScripts[idx].last_run_status      = d.status;
          this.scScripts[idx].last_run_output      = d.output;
          this.scScripts[idx].last_run_error       = d.error;
          this.scScripts[idx].last_run_duration_ms = d.duration_ms;
        }
        if (this.scSelected && this.scSelected.id === script.id) {
          this.scSelected.last_run_status      = d.status;
          this.scSelected.last_run_output      = d.output;
          this.scSelected.last_run_error       = d.error;
          this.scSelected.last_run_duration_ms = d.duration_ms;
        }
      } catch (err) {
        this.scRunError  = err.message || 'Run failed';
        this.scRunStatus = 'error';
      } finally {
        this.scRunning = false;
      }
    },

    // ── run adhoc ──
    async scRunAdhoc() {
      if (!this.scAdhocCode.trim() || this.scAdhocRunning) return;
      this.scAdhocRunning    = true;
      this.scAdhocOutput     = '';
      this.scAdhocError      = '';
      this.scAdhocStatus     = '';
      this.scAdhocDurationMs = 0;
      try {
        const res = await fetch('/zaplab/api/scripts/run', {
          method: 'POST',
          headers: { ...apiHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({ code: this.scAdhocCode, timeout_secs: 15 }),
        });
        const d = await res.json();
        this.scAdhocStatus     = d.status || '';
        this.scAdhocOutput     = d.output || '';
        this.scAdhocError      = d.error  || '';
        this.scAdhocDurationMs = d.duration_ms || 0;
      } catch (err) {
        this.scAdhocError  = err.message || 'Run failed';
        this.scAdhocStatus = 'error';
      } finally {
        this.scAdhocRunning = false;
      }
    },

    // ── helpers ──
    scStatusClass(status) {
      if (status === 'success') return 'text-green-500 dark:text-green-400';
      if (status === 'error')   return 'text-red-500  dark:text-red-400';
      return 'text-gray-400';
    },
    scStatusIcon(status) {
      if (status === 'success') return '✓';
      if (status === 'error')   return '✗';
      return '—';
    },
  };
}
