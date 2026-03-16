// DB Explorer section — browse whatsmeow SQLite tables for protocol research.
function dbExplorerSection() {
  // Column descriptions for each whatsmeow table, keyed by "table.column".
  const columnDocs = {
    // whatsmeow_device
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

    // whatsmeow_identity_keys
    'whatsmeow_identity_keys.their_id':   'The JID of the remote peer whose identity key is stored',
    'whatsmeow_identity_keys.key':        'The remote peer\'s Signal Protocol identity public key (32-byte Curve25519)',

    // whatsmeow_pre_keys
    'whatsmeow_pre_keys.key_id':          'Pre-key ID — a 32-bit integer that identifies this key on the server',
    'whatsmeow_pre_keys.key':             'One-time pre-key key pair (private+public Curve25519). Consumed once during X3DH session establishment',
    'whatsmeow_pre_keys.uploaded':        'Whether this pre-key has been uploaded to the WhatsApp server (1 = yes)',

    // whatsmeow_sessions
    'whatsmeow_sessions.their_id':        'JID of the remote peer this session belongs to',
    'whatsmeow_sessions.session':         'Serialised Signal Protocol Double Ratchet session state (protobuf). Contains chain keys, message keys, and ratchet counters',

    // whatsmeow_sender_keys
    'whatsmeow_sender_keys.our_jid':      'Our JID in the group this sender key belongs to',
    'whatsmeow_sender_keys.chat_id':      'Group/chat JID this sender key is scoped to',
    'whatsmeow_sender_keys.sender_id':    'JID of the group member who distributed this sender key',
    'whatsmeow_sender_keys.key':          'Serialised SenderKeyRecord — contains the sender\'s key chain used to decrypt their group messages',

    // whatsmeow_app_state_sync_keys
    'whatsmeow_app_state_sync_keys.key_id':       'App state sync key ID (bytes) — referenced by app state patches to identify which key to use for decryption',
    'whatsmeow_app_state_sync_keys.key_data':      'Serialised AppStateSyncKey proto — contains the 32-byte AES-256-CBC key and HMAC key for this sync key',
    'whatsmeow_app_state_sync_keys.timestamp':     'Unix timestamp when this sync key was created/received',
    'whatsmeow_app_state_sync_keys.fingerprint':   'Key fingerprint for integrity verification',
    'whatsmeow_app_state_sync_keys.recv_timestamp':'Unix timestamp when this key was received from the server',

    // whatsmeow_app_state_version
    'whatsmeow_app_state_version.name':    'App state collection name (e.g. "critical", "regular", "critical_unblock_to_primary", "md_messaging", "md_contacts")',
    'whatsmeow_app_state_version.version': 'Current version index for this collection — incremented with each patch applied',
    'whatsmeow_app_state_version.hash':    'Current Merkle tree root hash for this collection — used to detect tampering and verify sync completeness',

    // whatsmeow_app_state_mutation_macs
    'whatsmeow_app_state_mutation_macs.name':       'App state collection name this MAC belongs to',
    'whatsmeow_app_state_mutation_macs.version':    'Version index at which this mutation was applied',
    'whatsmeow_app_state_mutation_macs.index_mac':  'HMAC over the mutation index (the path in the Merkle tree)',
    'whatsmeow_app_state_mutation_macs.value_mac':  'HMAC over the mutation value — used to verify the integrity of the stored state item',

    // whatsmeow_contacts
    'whatsmeow_contacts.our_jid':    'Our JID — the account this contact record belongs to',
    'whatsmeow_contacts.their_jid':  'The contact\'s JID (WhatsApp address)',
    'whatsmeow_contacts.first_name': 'First name as stored in the device\'s address book (synced via app state)',
    'whatsmeow_contacts.full_name':  'Full name as stored in the device\'s address book',
    'whatsmeow_contacts.push_name':  'Push name — the display name the contact broadcasts in message metadata',
    'whatsmeow_contacts.business_name': 'WhatsApp Business display name (empty for personal accounts)',

    // whatsmeow_chat_settings
    'whatsmeow_chat_settings.our_jid':       'Our JID — the account this chat setting belongs to',
    'whatsmeow_chat_settings.chat_jid':      'JID of the chat/contact/group this setting applies to',
    'whatsmeow_chat_settings.muted_until':   'Unix timestamp until which notifications are muted (0 = not muted)',
    'whatsmeow_chat_settings.pinned':        '1 if this chat is pinned at the top of the chat list',
    'whatsmeow_chat_settings.archived':      '1 if this chat is archived',

    // whatsmeow_message_secrets
    'whatsmeow_message_secrets.our_jid':     'Our JID — the account this secret belongs to',
    'whatsmeow_message_secrets.chat_id':     'JID of the chat containing the message',
    'whatsmeow_message_secrets.sender_id':   'JID of the message sender',
    'whatsmeow_message_secrets.message_id':  'WhatsApp message ID this secret is associated with',
    'whatsmeow_message_secrets.key':         'Message-level secret key used for media decryption or ephemeral (view-once) message handling',

    // whatsmeow_privacy_tokens
    'whatsmeow_privacy_tokens.our_jid':   'Our JID — the account this token belongs to',
    'whatsmeow_privacy_tokens.their_jid': 'JID of the contact being checked',
    'whatsmeow_privacy_tokens.token':     'Privacy token — sent to the server when performing a contact availability check to avoid leaking the full contact list',
    'whatsmeow_privacy_tokens.timestamp': 'Unix timestamp when this token was issued',
  };

  return {
    // ── state ──
    dbe: {
      tables:      [],   // [{name, description, count}]
      table:       '',   // currently selected table name
      description: '',   // description of the selected table
      columns:     [],   // column names for the current table
      rows:        [],   // current page rows (array of arrays)
      page:        1,
      limit:       50,
      total:       0,
      pages:       0,
      filter:      '',
      loading:     false,
      loadingTables: false,
      error:       null,
      selectedRow: null,  // row index for detail panel
      copied:      false,
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

    // ── fetch table list ──
    async dbeLoadTables() {
      this.dbe.loadingTables = true;
      this.dbe.error = null;
      try {
        const res  = await this.zapFetch('/zaplab/api/db/tables', {
          headers: { 'Authorization': 'Bearer ' + pb.authStore.token, 'X-API-Token': this.apiToken },
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || res.statusText);
        this.dbe.tables = data.tables || [];
      } catch (err) {
        this.dbe.error = err.message;
      } finally {
        this.dbe.loadingTables = false;
      }
    },

    // ── select and load a table ──
    async dbeSelectTable(t) {
      this.dbe.table       = t.name;
      this.dbe.description = t.description;
      this.dbe.page        = 1;
      this.dbe.filter      = '';
      this.dbe.selectedRow = null;
      await this.dbeLoadTable();
    },

    // ── fetch table data ──
    async dbeLoadTable() {
      if (!this.dbe.table) return;
      this.dbe.loading     = true;
      this.dbe.error       = null;
      this.dbe.selectedRow = null;
      try {
        const params = new URLSearchParams({
          page:   this.dbe.page,
          limit:  this.dbe.limit,
          filter: this.dbe.filter,
        });
        const res  = await this.zapFetch(`/zaplab/api/db/tables/${this.dbe.table}?${params}`, {
          headers: { 'Authorization': 'Bearer ' + pb.authStore.token, 'X-API-Token': this.apiToken },
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || res.statusText);
        this.dbe.columns = data.columns || [];
        this.dbe.rows    = data.rows    || [];
        this.dbe.total   = data.total   || 0;
        this.dbe.pages   = data.pages   || 0;
      } catch (err) {
        this.dbe.error = err.message;
      } finally {
        this.dbe.loading = false;
      }
    },

    // ── pagination ──
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

    // ── row detail ──
    dbeSelectRow(idx) {
      this.dbe.selectedRow = this.dbe.selectedRow === idx ? null : idx;
      this.dbe.copied = false;
    },

    dbeRowObject(idx) {
      const row = this.dbe.rows[idx];
      if (!row) return {};
      return Object.fromEntries(this.dbe.columns.map((col, i) => [col, row[i]]));
    },

    dbeColumnDoc(col) {
      return columnDocs[`${this.dbe.table}.${col}`] || '';
    },

    // is the value likely a cryptographic blob? (long hex string)
    dbeIsBlob(val) {
      return typeof val === 'string' && val.length >= 32 && /^[0-9a-f]+$/i.test(val);
    },

    dbeFormatValue(val) {
      if (val === null || val === undefined) return 'NULL';
      if (typeof val === 'string' && val === '') return '(empty)';
      return String(val);
    },

    async dbeCopyRow(idx) {
      const obj = this.dbeRowObject(idx);
      try {
        await navigator.clipboard.writeText(JSON.stringify(obj, null, 2));
        this.dbe.copied = true;
        setTimeout(() => { this.dbe.copied = false; }, 2000);
      } catch {}
    },

    async dbeCopyTable() {
      if (!this.dbe.rows.length) return;
      const arr = this.dbe.rows.map(row =>
        Object.fromEntries(this.dbe.columns.map((col, i) => [col, row[i]]))
      );
      try {
        await navigator.clipboard.writeText(JSON.stringify(arr, null, 2));
        this.dbe.copied = true;
        setTimeout(() => { this.dbe.copied = false; }, 2000);
      } catch {}
    },
  };
}
