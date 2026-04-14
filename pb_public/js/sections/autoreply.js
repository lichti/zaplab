// Auto-Reply Rules — manage automated response rules for incoming messages.
function autoReplySection() {
  return {
    ar: {
      rules:   [],
      total:   0,
      loading: false,
      error:   '',
      toast:   null,
    },
    arForm: {
      // identity
      name:              '',
      enabled:           true,
      priority:          10,
      stop_on_match:     false,
      // conditions
      cond_from:         'others',
      cond_chat_jid:     '',
      cond_sender_jid:   '',
      cond_text_pattern: '',
      cond_text_match_type: 'contains',
      cond_case_sensitive: false,
      cond_hour_from:    -1,
      cond_hour_to:      -1,
      // action
      action_type:        'reply',
      action_reply_text:  '',
      action_webhook_url: '',
      action_script_id:   '',
    },
    arShowForm: false,
    arEditId:   null,

    async arLoad() {
      this.ar.loading = true;
      this.ar.error   = '';
      try {
        const res  = await fetch('/zaplab/api/auto-reply-rules', { headers: apiHeaders() });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.ar.rules = data.rules  || [];
        this.ar.total = data.total  || 0;
      } catch (err) {
        this.ar.error = err.message;
      } finally {
        this.ar.loading = false;
      }
    },

    arOpenCreate() {
      this.arEditId = null;
      this.arForm = {
        name: '', enabled: true, priority: 10, stop_on_match: false,
        cond_from: 'others', cond_chat_jid: '', cond_sender_jid: '',
        cond_text_pattern: '', cond_text_match_type: 'contains', cond_case_sensitive: false,
        cond_hour_from: -1, cond_hour_to: -1,
        action_type: 'reply', action_reply_text: '', action_webhook_url: '', action_script_id: '',
      };
      this.arShowForm = true;
    },

    arOpenEdit(rule) {
      this.arEditId = rule.id;
      this.arForm = {
        name:                rule.name,
        enabled:             rule.enabled,
        priority:            rule.priority,
        stop_on_match:       rule.stop_on_match,
        cond_from:           rule.cond_from           || 'others',
        cond_chat_jid:       rule.cond_chat_jid        || '',
        cond_sender_jid:     rule.cond_sender_jid      || '',
        cond_text_pattern:   rule.cond_text_pattern    || '',
        cond_text_match_type:rule.cond_text_match_type || 'contains',
        cond_case_sensitive: rule.cond_case_sensitive  || false,
        cond_hour_from:      rule.cond_hour_from       ?? -1,
        cond_hour_to:        rule.cond_hour_to         ?? -1,
        action_type:         rule.action_type          || 'reply',
        action_reply_text:   rule.action_reply_text    || '',
        action_webhook_url:  rule.action_webhook_url   || '',
        action_script_id:    rule.action_script_id     || '',
      };
      this.arShowForm = true;
    },

    async arSave() {
      if (!this.arForm.name) return;
      this.ar.loading = true;
      this.ar.toast   = null;
      try {
        const url    = this.arEditId
          ? `/zaplab/api/auto-reply-rules/${this.arEditId}`
          : '/zaplab/api/auto-reply-rules';
        const method = this.arEditId ? 'PATCH' : 'POST';
        const res    = await fetch(url, {
          method,
          headers: { ...apiHeaders(), 'Content-Type': 'application/json' },
          body:    JSON.stringify(this.arForm),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.ar.toast   = { type: 'success', text: this.arEditId ? 'Rule updated.' : 'Rule created.' };
        this.arShowForm = false;
        await this.arLoad();
      } catch (err) {
        this.ar.toast = { type: 'error', text: err.message };
      } finally {
        this.ar.loading = false;
      }
    },

    async arToggle(rule) {
      try {
        const res  = await fetch(`/zaplab/api/auto-reply-rules/${rule.id}/toggle`, {
          method: 'POST', headers: apiHeaders(),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        rule.enabled = data.enabled;
      } catch (err) {
        this.ar.toast = { type: 'error', text: err.message };
      }
    },

    async arDelete(rule) {
      if (!confirm(`Delete rule "${rule.name}"?`)) return;
      try {
        const res  = await fetch(`/zaplab/api/auto-reply-rules/${rule.id}`, {
          method: 'DELETE', headers: apiHeaders(),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        await this.arLoad();
      } catch (err) {
        this.ar.toast = { type: 'error', text: err.message };
      }
    },

    arMatchTypeBadge(t) {
      const map = { prefix: 'blue', contains: 'green', exact: 'yellow', regex: 'purple' };
      return map[t] || 'gray';
    },

    arActionBadge(t) {
      const map = { reply: 'green', webhook: 'blue', script: 'purple' };
      return map[t] || 'gray';
    },
  };
}
