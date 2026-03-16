// DB Explorer section — browse, edit, and restore whatsmeow SQLite tables.
function dbExplorerSection() {
  // Column-level protocol documentation, keyed by "table.column".
  const columnDocs = {
    'whatsmeow_device.jid':               'JID (Jabber ID) — the WhatsApp address of this device (e.g. 5511…@s.whatsapp.net)',
    'whatsmeow_device.registration_id':   'Registration ID — random 32-bit integer assigned at registration, sent in every Signal pre-key bundle',
    'whatsmeow_device.noise_key':         'Noise Protocol static key pair (private+public) — used for the NK handshake that authenticates the WhatsApp server',
    'whatsmeow_device.identity_key':      'Signal Protocol long-term identity key pair — used in X3DH key agreement and session establishment',
    'whatsmeow_device.signed_pre_key':    'Signed pre-key — a medium-term key signed by the identity key, published to the server for others to start sessions',
    'whatsmeow_device.signed_pre_key_id': 'ID of the current signed pre-key uploaded to the server',
    'whatsmeow_device.signed_pre_key_sig':'Signature over the signed pre-key, verifiable with the identity key',
    'whatsmeow_device.adv_key':           'Advertised key — public key included in the multi-device advertisement sent to other devices in the same account',
    'whatsmeow_device.adv_details':       'Multi-device advertisement details (proto-serialised)',
    'whatsmeow_device.adv_account_sig':   'Account-level signature for the multi-device advertisement',
    'whatsmeow_device.adv_account_sig_key':'Key used to produce the account-level advertisement signature',
    'whatsmeow_device.adv_device_sig':    'Device-level signature for the multi-device advertisement',
    'whatsmeow_device.platform':          'Reported platform string sent to the server (e.g. "smba", "iphone")',
    'whatsmeow_device.business_name':     'WhatsApp Business display name (empty for personal accounts)',
    'whatsmeow_device.push_name':         'Push name — display name broadcast to contacts in message metadata',
    'whatsmeow_device.facebook_uuid':     'Facebook/Meta account UUID linked to this device (if Meta login was used)',
    'whatsmeow_identity_keys.their_id':   'The JID of the remote peer whose identity key is stored',
    'whatsmeow_identity_keys.key':        'The remote peer\'s Signal Protocol identity public key (32-byte Curve25519)',
    'whatsmeow_pre_keys.key_id':          'Pre-key ID — a 32-bit integer that identifies this key on the server',
    'whatsmeow_pre_keys.key':             'One-time pre-key key pair (private+public Curve25519). Consumed once during X3DH session establishment',
    'whatsmeow_pre_keys.uploaded':        'Whether this pre-key has been uploaded to the WhatsApp server (1 = yes)',
    'whatsmeow_sessions.their_id':        'JID of the remote peer this session belongs to',
    'whatsmeow_sessions.session':         'Serialised Signal Protocol Double Ratchet session state (protobuf). Contains chain keys, message keys, and ratchet counters',
    'whatsmeow_sender_keys.our_jid':      'Our JID in the group this sender key belongs to',
    'whatsmeow_sender_keys.chat_id':      'Group/chat JID this sender key is scoped to',
    'whatsmeow_sender_keys.sender_id':    'JID of the group member who distributed this sender key',
    'whatsmeow_sender_keys.key':          'Serialised SenderKeyRecord — contains the sender\'s key chain used to decrypt their group messages',
    'whatsmeow_app_state_sync_keys.key_id':       'App state sync key ID (bytes) — referenced by app state patches to identify which key to use for decryption',
    'whatsmeow_app_state_sync_keys.key_data':      'Serialised AppStateSyncKey proto — contains the 32-byte AES-256-CBC key and HMAC key for this sync key',
    'whatsmeow_app_state_sync_keys.timestamp':     'Unix timestamp when this sync key was created/received',
    'whatsmeow_app_state_sync_keys.fingerprint':   'Key fingerprint for integrity verification',
    'whatsmeow_app_state_sync_keys.recv_timestamp':'Unix timestamp when this key was received from the server',
    'whatsmeow_app_state_version.name':    'App state collection name (e.g. "critical", "regular", "critical_unblock_to_primary", "md_messaging", "md_contacts")',
    'whatsmeow_app_state_version.version': 'Current version index for this collection — incremented with each patch applied',
    'whatsmeow_app_state_version.hash':    'Current Merkle tree root hash for this collection — used to detect tampering and verify sync completeness',
    'whatsmeow_app_state_mutation_macs.name':       'App state collection name this MAC belongs to',
    'whatsmeow_app_state_mutation_macs.version':    'Version index at which this mutation was applied',
    'whatsmeow_app_state_mutation_macs.index_mac':  'HMAC over the mutation index (the path in the Merkle tree)',
    'whatsmeow_app_state_mutation_macs.value_mac':  'HMAC over the mutation value — used to verify the integrity of the stored state item',
    'whatsmeow_contacts.our_jid':    'Our JID — the account this contact record belongs to',
    'whatsmeow_contacts.their_jid':  'The contact\'s JID (WhatsApp address)',
    'whatsmeow_contacts.first_name': 'First name as stored in the device\'s address book (synced via app state)',
    'whatsmeow_contacts.full_name':  'Full name as stored in the device\'s address book',
    'whatsmeow_contacts.push_name':  'Push name — the display name the contact broadcasts in message metadata',
    'whatsmeow_contacts.business_name': 'WhatsApp Business display name (empty for personal accounts)',
    'whatsmeow_chat_settings.our_jid':       'Our JID — the account this chat setting belongs to',
    'whatsmeow_chat_settings.chat_jid':      'JID of the chat/contact/group this setting applies to',
    'whatsmeow_chat_settings.muted_until':   'Unix timestamp until which notifications are muted (0 = not muted)',
    'whatsmeow_chat_settings.pinned':        '1 if this chat is pinned at the top of the chat list',
    'whatsmeow_chat_settings.archived':      '1 if this chat is archived',
    'whatsmeow_message_secrets.our_jid':     'Our JID — the account this secret belongs to',
    'whatsmeow_message_secrets.chat_id':     'JID of the chat containing the message',
    'whatsmeow_message_secrets.sender_id':   'JID of the message sender',
    'whatsmeow_message_secrets.message_id':  'WhatsApp message ID this secret is associated with',
    'whatsmeow_message_secrets.key':         'Message-level secret key used for media decryption or ephemeral (view-once) message handling',
    'whatsmeow_privacy_tokens.our_jid':   'Our JID — the account this token belongs to',
    'whatsmeow_privacy_tokens.their_jid': 'JID of the contact being checked',
    'whatsmeow_privacy_tokens.token':     'Privacy token — sent to the server when performing a contact availability check to avoid leaking the full contact list',
    'whatsmeow_privacy_tokens.timestamp': 'Unix timestamp when this token was issued',
  };

  return {
    // ── state ──
    dbe: {
      // table list
      tables:        [],
      loadingTables: false,
      // current table
      table:         '',
      description:   '',
      columns:       [],  // includes '_rowid_' as [0]
      types:         [],  // SQLite type per column
      rows:          [],
      page:          1,
      limit:         50,
      total:         0,
      pages:         0,
      filter:        '',
      loading:       false,
      error:         null,
      // row detail
      selectedRow:   null,
      copied:        false,
      // edit mode
      editing:       false,
      editValues:    {},   // {col: value}
      saving:        false,
      saveError:     null,
      saveResult:    null,
      autoReconnect: true,
      // delete
      deleting:      false,
      confirmDelete: false,
      // backups panel
      showBackups:    false,
      backups:        [],
      backupsLoading: false,
      backupCreating: false,
      backupError:    null,
      restoring:      null, // backup name being restored
      reconnecting:   false,
    },

    // ── init ──
    initDBExplorer() {
      this.$watch('activeSection', val => {
        if (val === 'dbexplorer' && this.dbe.tables.length === 0) {
          this.dbeLoadTables();
        }
      });
      if (this.activeSection === 'dbexplorer') this.dbeLoadTables();
    },

    _dbeHeaders() {
      return {
        'Authorization': 'Bearer ' + pb.authStore.token,
        'X-API-Token':   this.apiToken,
      };
    },

    // ── table list ──
    async dbeLoadTables() {
      this.dbe.loadingTables = true;
      this.dbe.error = null;
      try {
        const res  = await this.zapFetch('/zaplab/api/db/tables', { headers: this._dbeHeaders() });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || res.statusText);
        this.dbe.tables = data.tables || [];
      } catch (err) {
        this.dbe.error = err.message;
      } finally {
        this.dbe.loadingTables = false;
      }
    },

    async dbeSelectTable(t) {
      this.dbe.table       = t.name;
      this.dbe.description = t.description;
      this.dbe.page        = 1;
      this.dbe.filter      = '';
      this.dbe.selectedRow = null;
      this.dbe.editing     = false;
      this.dbe.saveResult  = null;
      this.dbe.confirmDelete = false;
      this.dbe.showBackups   = false;
      await this.dbeLoadTable();
    },

    // ── table data ──
    async dbeLoadTable() {
      if (!this.dbe.table) return;
      this.dbe.loading     = true;
      this.dbe.error       = null;
      this.dbe.selectedRow = null;
      this.dbe.editing     = false;
      this.dbe.saveResult  = null;
      this.dbe.confirmDelete = false;
      try {
        const params = new URLSearchParams({ page: this.dbe.page, limit: this.dbe.limit, filter: this.dbe.filter });
        const res  = await this.zapFetch(`/zaplab/api/db/tables/${this.dbe.table}?${params}`, { headers: this._dbeHeaders() });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || res.statusText);
        this.dbe.columns = data.columns || [];
        this.dbe.types   = data.types   || [];
        this.dbe.rows    = data.rows    || [];
        this.dbe.total   = data.total   || 0;
        this.dbe.pages   = data.pages   || 0;
      } catch (err) {
        this.dbe.error = err.message;
      } finally {
        this.dbe.loading = false;
      }
    },

    async dbeNextPage() {
      if (this.dbe.page >= this.dbe.pages) return;
      this.dbe.page++;
      await this.dbeLoadTable();
    },
    async dbePrevPage() {
      if (this.dbe.page <= 1) return;
      this.dbe.page--;
      await this.dbeLoadTable();
    },
    async dbeSearch() {
      this.dbe.page = 1;
      await this.dbeLoadTable();
    },

    // ── row detail helpers ──
    dbeSelectRow(idx) {
      if (this.dbe.selectedRow === idx) { this.dbe.selectedRow = null; return; }
      this.dbe.selectedRow   = idx;
      this.dbe.editing       = false;
      this.dbe.saveResult    = null;
      this.dbe.saveError     = null;
      this.dbe.copied        = false;
      this.dbe.confirmDelete = false;
    },

    // Returns the data columns only (skip _rowid_ at index 0)
    dbeDataCols() {
      return this.dbe.columns.slice(1);
    },
    dbeDataTypes() {
      return this.dbe.types.slice(1);
    },
    // Row values without the rowid
    dbeDataVals(idx) {
      const row = this.dbe.rows[idx];
      return row ? row.slice(1) : [];
    },
    dbeRowID(idx) {
      const row = this.dbe.rows[idx];
      return row ? row[0] : null;
    },
    dbeRowObject(idx) {
      const cols = this.dbeDataCols();
      const vals = this.dbeDataVals(idx);
      return Object.fromEntries(cols.map((c, i) => [c, vals[i]]));
    },

    dbeColumnDoc(col) {
      return columnDocs[`${this.dbe.table}.${col}`] || '';
    },

    dbeIsBlob(val) {
      return typeof val === 'string' && val.length >= 32 && /^[0-9a-f]+$/i.test(val);
    },
    dbeIsBlобType(typeStr) {
      return (typeStr || '').includes('BLOB');
    },
    dbeFormatValue(val) {
      if (val === null || val === undefined) return 'NULL';
      if (typeof val === 'string' && val === '') return '(empty)';
      return String(val);
    },

    async dbeCopyRow(idx) {
      try {
        await navigator.clipboard.writeText(JSON.stringify(this.dbeRowObject(idx), null, 2));
        this.dbe.copied = true;
        setTimeout(() => { this.dbe.copied = false; }, 2000);
      } catch {}
    },
    async dbeCopyTable() {
      const arr = this.dbe.rows.map((_, i) => this.dbeRowObject(i));
      try {
        await navigator.clipboard.writeText(JSON.stringify(arr, null, 2));
        this.dbe.copied = true;
        setTimeout(() => { this.dbe.copied = false; }, 2000);
      } catch {}
    },

    // ── edit mode ──
    dbeStartEdit() {
      const cols = this.dbeDataCols();
      const vals = this.dbeDataVals(this.dbe.selectedRow);
      this.dbe.editValues = Object.fromEntries(cols.map((c, i) => [c, vals[i] === null ? '' : String(vals[i])]));
      this.dbe.editing    = true;
      this.dbe.saveResult = null;
      this.dbe.saveError  = null;
    },
    dbeCancelEdit() {
      this.dbe.editing   = false;
      this.dbe.saveError = null;
    },

    async dbeSaveRow() {
      this.dbe.saving    = true;
      this.dbe.saveError = null;
      this.dbe.saveResult = null;
      const rowid = this.dbeRowID(this.dbe.selectedRow);
      try {
        const res  = await this.zapFetch(`/zaplab/api/db/tables/${this.dbe.table}/${rowid}`, {
          method:  'PATCH',
          headers: { ...this._dbeHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({ values: this.dbe.editValues, reconnect: this.dbe.autoReconnect }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || res.statusText);
        this.dbe.saveResult = data;
        this.dbe.editing    = false;
        // Refresh table data and table counts
        await this.dbeLoadTable();
        await this.dbeLoadTables();
      } catch (err) {
        this.dbe.saveError = err.message;
      } finally {
        this.dbe.saving = false;
      }
    },

    // ── delete row ──
    async dbeDeleteRow() {
      this.dbe.deleting = true;
      this.dbe.saveError = null;
      this.dbe.saveResult = null;
      const rowid = this.dbeRowID(this.dbe.selectedRow);
      try {
        const res  = await this.zapFetch(`/zaplab/api/db/tables/${this.dbe.table}/${rowid}`, {
          method:  'DELETE',
          headers: { ...this._dbeHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({ reconnect: this.dbe.autoReconnect }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || res.statusText);
        this.dbe.saveResult    = data;
        this.dbe.selectedRow   = null;
        this.dbe.confirmDelete = false;
        await this.dbeLoadTable();
        await this.dbeLoadTables();
      } catch (err) {
        this.dbe.saveError     = err.message;
        this.dbe.confirmDelete = false;
      } finally {
        this.dbe.deleting = false;
      }
    },

    // ── reconnect ──
    async dbeReconnect(full) {
      this.dbe.reconnecting = true;
      this.dbe.saveResult   = null;
      try {
        const res  = await this.zapFetch('/zaplab/api/db/reconnect', {
          method:  'POST',
          headers: { ...this._dbeHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({ full: !!full }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || res.statusText);
        this.dbe.saveResult = { message: data.message };
      } catch (err) {
        this.dbe.saveError = err.message;
      } finally {
        this.dbe.reconnecting = false;
      }
    },

    // ── backups ──
    async dbeToggleBackups() {
      this.dbe.showBackups = !this.dbe.showBackups;
      if (this.dbe.showBackups) await this.dbeLoadBackups();
    },

    async dbeLoadBackups() {
      this.dbe.backupsLoading = true;
      this.dbe.backupError    = null;
      try {
        const res  = await this.zapFetch('/zaplab/api/db/backups', { headers: this._dbeHeaders() });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || res.statusText);
        this.dbe.backups = data.backups || [];
      } catch (err) {
        this.dbe.backupError = err.message;
      } finally {
        this.dbe.backupsLoading = false;
      }
    },

    async dbeCreateBackup() {
      this.dbe.backupCreating = true;
      this.dbe.backupError    = null;
      try {
        const res  = await this.zapFetch('/zaplab/api/db/backup', {
          method:  'POST',
          headers: { ...this._dbeHeaders(), 'Content-Type': 'application/json' },
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || res.statusText);
        await this.dbeLoadBackups();
      } catch (err) {
        this.dbe.backupError = err.message;
      } finally {
        this.dbe.backupCreating = false;
      }
    },

    async dbeRestoreBackup(name) {
      this.dbe.restoring   = name;
      this.dbe.backupError = null;
      try {
        const res  = await this.zapFetch('/zaplab/api/db/restore', {
          method:  'POST',
          headers: { ...this._dbeHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({ name }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || res.statusText);
        this.dbe.saveResult  = { message: data.message };
        this.dbe.showBackups = false;
        // Reload table list and data after restore
        await this.dbeLoadTables();
        if (this.dbe.table) await this.dbeLoadTable();
      } catch (err) {
        this.dbe.backupError = err.message;
      } finally {
        this.dbe.restoring = null;
      }
    },

    async dbeDeleteBackup(name) {
      this.dbe.backupError = null;
      try {
        const res  = await this.zapFetch(`/zaplab/api/db/backups/${encodeURIComponent(name)}`, {
          method:  'DELETE',
          headers: this._dbeHeaders(),
        });
        if (!res.ok) { const d = await res.json(); throw new Error(d.error || res.statusText); }
        await this.dbeLoadBackups();
      } catch (err) {
        this.dbe.backupError = err.message;
      }
    },

    dbeFormatSize(bytes) {
      if (!bytes) return '0 B';
      if (bytes < 1024) return bytes + ' B';
      if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
      return (bytes / 1024 / 1024).toFixed(2) + ' MB';
    },
  };
}
