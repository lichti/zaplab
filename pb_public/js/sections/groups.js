// Groups section — state, init, preview helpers, submit, picker, member picker.
function groupsSection() {
  return {
    // ── state ──
    groups: {
      type:              'list',
      jid:               '',
      name:              '',
      participantsList:  '',
      participantAction: 'add',
      newName:           '',
      newTopic:          '',
      setAnnounce:       false,
      announce:          false,
      setLocked:         false,
      locked:            false,
      resetInviteLink:   false,
      inviteLink:        '',
      photoData:         '',
      photoName:         '',
      loading:           false,
      toast:             null,
      result:            null,
    },
    groupsPreviewTab:      localStorage.getItem('zaplab-groups-preview-tab') || 'curl',
    groupsPreviewCopied:   false,

    // ── Group Picker state ──
    groupsList:            [],
    groupsListLoading:     false,
    groupsListFilter:      '',
    groupsPickerOpen:      true,

    // ── Member Picker state ──
    groupsMembersList:     [],
    groupsMembersLoading:  false,
    groupsMembersAdded:    {},

    // ── Leave confirmation ──
    groupsLeaveConfirm:    false,

    // ── Invite QR ──
    groupsInviteQR:        null,
    groupsInviteQRLoading: false,

    // ── Recent JIDs ──
    groupsRecentJIDs:      JSON.parse(localStorage.getItem('zaplab-groups-recent-jids') || '[]'),

    // ── section init ──
    initGroups() {
      this.$watch('groups.type', () => {
        this.groups.toast       = null;
        this.groups.result      = null;
        this.groupsLeaveConfirm = false;
        if (this.groupsPreviewTab === 'response' || this.groupsPreviewTab === 'table') {
          this.groupsPreviewTab = 'curl';
        }
        if (this.groupsNeedsJID() && this.groupsList.length === 0 && !this.groupsListLoading) {
          this.loadGroupsList();
        }
      });
      this.$watch('groupsPreviewTab', val => {
        localStorage.setItem('zaplab-groups-preview-tab', val);
      });
    },

    // ── helpers ──
    groupsNeedsJID() {
      return ['info', 'participants', 'settings', 'photo', 'leave', 'invitelink'].includes(this.groups.type);
    },

    groupsEndpoint() {
      const jid = encodeURIComponent(this.groups.jid || '<jid>');
      switch (this.groups.type) {
        case 'list':         return '/zaplab/api/groups';
        case 'info':         return `/zaplab/api/groups/${jid}`;
        case 'create':       return '/zaplab/api/groups';
        case 'participants': return `/zaplab/api/groups/${jid}/participants`;
        case 'settings':     return `/zaplab/api/groups/${jid}`;
        case 'photo':        return `/zaplab/api/groups/${jid}/photo`;
        case 'leave':        return `/zaplab/api/groups/${jid}/leave`;
        case 'invitelink':   return `/zaplab/api/groups/${jid}/invitelink${this.groups.resetInviteLink ? '?reset=true' : ''}`;
        case 'join':         return '/zaplab/api/groups/join';
        default: return '/zaplab/api/groups';
      }
    },

    groupsMethod() {
      switch (this.groups.type) {
        case 'list':
        case 'info':
        case 'invitelink': return 'GET';
        case 'settings':   return 'PATCH';
        default:           return 'POST';
      }
    },

    groupsLabel() {
      return {
        list:         'List Groups',
        info:         'Get Info',
        create:       'Create Group',
        participants: 'Update Participants',
        settings:     'Update Settings',
        photo:        'Set Photo',
        leave:        'Leave Group',
        invitelink:   'Get Invite Link',
        join:         'Join Group',
      }[this.groups.type] || 'Submit';
    },

    groupsCurlPayload() {
      const parseLines = s => s.split('\n').map(l => l.trim()).filter(Boolean);
      switch (this.groups.type) {
        case 'create':
          return { name: this.groups.name || '<name>', participants: parseLines(this.groups.participantsList) };
        case 'participants':
          return { action: this.groups.participantAction, participants: parseLines(this.groups.participantsList) };
        case 'settings': {
          const body = {};
          if (this.groups.newName)     body.name     = this.groups.newName;
          if (this.groups.newTopic)    body.topic    = this.groups.newTopic;
          if (this.groups.setAnnounce) body.announce = this.groups.announce;
          if (this.groups.setLocked)   body.locked   = this.groups.locked;
          return body;
        }
        case 'photo':
          return { image: this.groups.photoData ? `<base64: ${this.groups.photoName}>` : '<base64 JPEG or PNG>' };
        case 'join':
          return { link: this.groups.inviteLink || '<invite_link>' };
        default:
          return null;
      }
    },

    handleGroupPhoto(event) {
      const file = event.target.files[0];
      if (!file) return;
      this.groups.photoName = file.name;
      const reader = new FileReader();
      reader.onload = e => { this.groups.photoData = e.target.result.split(',')[1]; };
      reader.readAsDataURL(file);
    },

    groupsCurlPreview() {
      const token   = this.apiToken || '<your-api-token>';
      const method  = this.groupsMethod();
      const url     = `${window.location.origin}${this.groupsEndpoint()}`;
      const payload = this.groupsCurlPayload();
      const lines = [
        `# auth disabled — X-API-Token not required`,
        `curl -X ${method} \\`,
        `  ${url} \\`,
        `  -H "Content-Type: application/json" \\`,
        `  -H "X-API-Token: ${token}"`,
      ];
      if (payload !== null) {
        lines[lines.length - 1] += ' \\';
        lines.push(`  -d '${JSON.stringify(payload)}'`);
      }
      return lines.join('\n');
    },

    groupsToastJson() {
      if (!this.groups.result) return '';
      return this.highlight(this.groups.result);
    },

    groupsResultPreview() {
      if (!this.groups.result) {
        return '<span style="color:var(--tw-prose-captions,#8b949e)">No response yet — submit an action first.</span>';
      }
      return this.highlight(this.groups.result);
    },

    groupsPreviewContent() {
      if (this.groupsPreviewTab === 'response') return this.groupsResultPreview();
      return this.highlightCurl(this.groupsCurlPreview());
    },

    async copyGroupsPreview() {
      const text = this.groupsPreviewTab === 'response'
        ? JSON.stringify(this.groups.result, null, 2)
        : this.groupsCurlPreview();
      try {
        await navigator.clipboard.writeText(text);
        this.groupsPreviewCopied = true;
        setTimeout(() => { this.groupsPreviewCopied = false; }, 2000);
      } catch {}
    },

    groupsHasTable() {
      if (!this.groups.result) return false;
      return ['list', 'info', 'invitelink'].includes(this.groups.type);
    },

    // ── Group Picker ──
    groupsListFiltered() {
      const q = this.groupsListFilter.toLowerCase();
      if (!q) return this.groupsList;
      return this.groupsList.filter(g =>
        (g.Name || '').toLowerCase().includes(q) ||
        (g.JID  || '').toLowerCase().includes(q)
      );
    },

    groupAdminCount(g) {
      if (!g.Participants) return 0;
      return g.Participants.filter(p => p.IsAdmin || p.IsSuperAdmin).length;
    },

    async loadGroupsList() {
      this.groupsListLoading = true;
      try {
        const res  = await fetch('/zaplab/api/groups', { headers: { 'X-API-Token': this.apiToken } });
        const data = await res.json();
        if (res.ok) this.groupsList = data.groups || [];
      } catch {}
      this.groupsListLoading = false;
    },

    selectGroupFromPicker(jid) {
      this.groups.jid       = jid;
      this.groupsPickerOpen = false;
    },

    saveRecentJID(jid, name) {
      const existing = JSON.parse(localStorage.getItem('zaplab-groups-recent-jids') || '[]');
      const filtered = existing.filter(r => r.jid !== jid);
      filtered.unshift({ jid, name });
      const trimmed = filtered.slice(0, 5);
      localStorage.setItem('zaplab-groups-recent-jids', JSON.stringify(trimmed));
      this.groupsRecentJIDs = trimmed;
    },

    // ── Member Picker ──
    showMemberPicker() {
      return this.groups.type === 'participants' &&
             ['remove', 'promote', 'demote'].includes(this.groups.participantAction) &&
             this.groups.jid !== '';
    },

    async loadGroupMembers() {
      if (!this.groups.jid) return;
      this.groupsMembersLoading = true;
      this.groupsMembersList    = [];
      this.groupsMembersAdded   = {};
      try {
        const jid  = encodeURIComponent(this.groups.jid);
        const res  = await fetch(`/zaplab/api/groups/${jid}/participants`, { headers: { 'X-API-Token': this.apiToken } });
        const data = await res.json();
        if (res.ok) this.groupsMembersList = data.participants || [];
      } catch {}
      this.groupsMembersLoading = false;
    },

    toggleGroupMember(jid) {
      const lines = this.groups.participantsList.split('\n').map(l => l.trim()).filter(Boolean);
      if (this.groupsMembersAdded[jid]) {
        const idx = lines.indexOf(jid);
        if (idx !== -1) lines.splice(idx, 1);
        delete this.groupsMembersAdded[jid];
      } else {
        if (!lines.includes(jid)) lines.push(jid);
        this.groupsMembersAdded[jid] = true;
      }
      this.groups.participantsList = lines.join('\n');
    },

    // ── Settings loader ──
    async loadCurrentSettings() {
      if (!this.groups.jid) return;
      try {
        const jid  = encodeURIComponent(this.groups.jid);
        const res  = await fetch(`/zaplab/api/groups/${jid}`, { headers: { 'X-API-Token': this.apiToken } });
        const data = await res.json();
        if (res.ok) {
          this.groups.newName     = data.Name  || '';
          this.groups.newTopic    = data.Topic || '';
          this.groups.announce    = !!data.IsAnnounce;
          this.groups.setAnnounce = true;
          this.groups.locked      = !!data.IsLocked;
          this.groups.setLocked   = true;
          this.groups.toast = { ok: true, message: 'Settings loaded' };
        }
      } catch {}
    },

    // ── Leave confirmation ──
    confirmLeave() { this.groupsLeaveConfirm = true; },
    cancelLeave()  { this.groupsLeaveConfirm = false; },

    groupsLeaveGroupName() {
      const found = this.groupsList.find(g => g.JID === this.groups.jid);
      return found ? found.Name : this.groups.jid;
    },

    // ── Invite QR ──
    async fetchInviteLinkQR(link) {
      this.groupsInviteQRLoading = true;
      this.groupsInviteQR        = null;
      try {
        const res  = await fetch('/zaplab/api/wa/qrtext', {
          method:  'POST',
          headers: { 'Content-Type': 'application/json', 'X-API-Token': this.apiToken },
          body:    JSON.stringify({ text: link }),
        });
        const data = await res.json();
        if (res.ok) this.groupsInviteQR = data.image;
      } catch {}
      this.groupsInviteQRLoading = false;
    },

    // ── CSV export ──
    exportGroupsCSV() {
      if (!this.groups.result || !this.groups.result.groups) return;
      const rows = [['Name', 'JID', 'Members', 'Topic', 'Announce', 'Locked']];
      for (const g of this.groups.result.groups) {
        rows.push([
          `"${(g.Name  || '').replace(/"/g, '""')}"`,
          g.JID || '',
          (g.Participants || []).length,
          `"${(g.Topic || '').replace(/"/g, '""')}"`,
          g.IsAnnounce ? 'true' : 'false',
          g.IsLocked   ? 'true' : 'false',
        ]);
      }
      const csv  = rows.map(r => r.join(',')).join('\n');
      const blob = new Blob([csv], { type: 'text/csv' });
      const a    = document.createElement('a');
      a.href     = URL.createObjectURL(blob);
      a.download = `groups-${Date.now()}.csv`;
      a.click();
    },

    // ── Quick actions from table ──
    groupsQuickAction(type, jid) {
      this.groups.type = type;
      this.groups.jid  = jid;
      this.$nextTick(() => {
        document.querySelector('[x-show="activeSection === \'groups\'"]')
          ?.scrollIntoView({ behavior: 'smooth', block: 'start' });
      });
    },

    // ── Submit ──
    async submitGroups() {
      if (this.groups.type === 'leave' && !this.groupsLeaveConfirm) {
        this.confirmLeave();
        return;
      }
      if (this.groups.type === 'photo' && !this.groups.photoData) {
        this.groups.toast = { ok: false, message: 'Please select an image file first' };
        return;
      }
      this.groupsLeaveConfirm = false;
      this.groups.toast       = null;
      this.groups.loading     = true;
      try {
        const method  = this.groupsMethod();
        let payload = this.groupsCurlPayload();
        // For photo, replace preview placeholder with actual base64 data
        if (this.groups.type === 'photo') payload = { image: this.groups.photoData };
        const opts = {
          method,
          headers: { 'Content-Type': 'application/json', 'X-API-Token': this.apiToken },
        };
        if (payload !== null) opts.body = JSON.stringify(payload);
        const res  = await fetch(this.groupsEndpoint(), opts);
        const data = await res.json();
        const successLabel = {
          list:       `${(data.groups || []).length} groups loaded`,
          info:       data.Name ? `Info loaded: ${data.Name}` : 'Info loaded',
          invitelink: 'Invite link loaded',
        }[this.groups.type];
        const toastMsg = data.message || (res.ok ? successLabel : null) || 'Done';
        this.groups.toast = { ok: res.ok, message: toastMsg };
        if (res.ok) {
          this.groups.result    = data;
          this.groupsPreviewTab = this.groupsHasTable() ? 'table' : 'response';
          if (this.groups.jid) {
            const found = this.groupsList.find(g => g.JID === this.groups.jid);
            this.saveRecentJID(this.groups.jid, found?.Name || '');
          }
          if (['create', 'leave', 'join'].includes(this.groups.type)) {
            this.groupsList = [];
            this.loadGroupsList();
          }
          if (this.groups.type === 'invitelink' && data.link) {
            this.fetchInviteLinkQR(data.link);
          }
        }
      } catch (err) {
        this.groups.toast = { ok: false, message: err.message };
      } finally {
        this.groups.loading = false;
      }
    },
  };
}
