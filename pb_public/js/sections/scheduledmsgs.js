// Scheduled Messages — create and manage future message sends.
function scheduledMsgsSection() {
  return {
    sm: {
      messages:  [],
      total:     0,
      loading:   false,
      error:     '',
      toast:     null,
      filter:    'pending', // pending | sent | failed | cancelled | all
    },
    smForm: {
      chatJid:     '',
      messageText: '',
      scheduledAt: '',
      replyToId:   '',
    },
    smShowForm: false,

    async smLoad() {
      this.sm.loading = true;
      this.sm.error   = '';
      try {
        const p = new URLSearchParams();
        if (this.sm.filter !== 'all') p.set('status', this.sm.filter);
        const res  = await fetch(`/zaplab/api/scheduled-messages?${p}`, { headers: apiHeaders() });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.sm.messages = data.scheduled_messages || [];
        this.sm.total    = data.total              || 0;
      } catch (err) {
        this.sm.error = err.message;
      } finally {
        this.sm.loading = false;
      }
    },

    async smCreate() {
      if (!this.smForm.chatJid || !this.smForm.messageText || !this.smForm.scheduledAt) return;
      this.sm.loading = true;
      this.sm.toast   = null;
      try {
        const res  = await fetch('/zaplab/api/scheduled-messages', {
          method:  'POST',
          headers: { ...apiHeaders(), 'Content-Type': 'application/json' },
          body:    JSON.stringify({
            chat_jid:        this.smForm.chatJid,
            message_text:    this.smForm.messageText,
            scheduled_at:    new Date(this.smForm.scheduledAt).toISOString(),
            reply_to_msg_id: this.smForm.replyToId,
          }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.sm.toast = { ok: true, message: 'Scheduled!' };
        this.smForm = { chatJid: '', messageText: '', scheduledAt: '', replyToId: '' };
        this.smShowForm = false;
        await this.smLoad();
      } catch (err) {
        this.sm.toast = { ok: false, message: err.message };
      } finally {
        this.sm.loading = false;
      }
    },

    async smCancel(id) {
      this.sm.loading = true;
      try {
        const res = await fetch(`/zaplab/api/scheduled-messages/${id}`, {
          method:  'PATCH',
          headers: { ...apiHeaders(), 'Content-Type': 'application/json' },
          body:    JSON.stringify({ status: 'cancelled' }),
        });
        if (!res.ok) { const d = await res.json(); throw new Error(d.error || 'Failed'); }
        await this.smLoad();
      } catch (err) {
        this.sm.toast = { ok: false, message: err.message };
      } finally {
        this.sm.loading = false;
      }
    },

    async smDelete(id) {
      this.sm.loading = true;
      try {
        const res = await fetch(`/zaplab/api/scheduled-messages/${id}`, {
          method: 'DELETE', headers: apiHeaders(),
        });
        if (!res.ok) { const d = await res.json(); throw new Error(d.error || 'Failed'); }
        await this.smLoad();
      } catch (err) {
        this.sm.toast = { ok: false, message: err.message };
      } finally {
        this.sm.loading = false;
      }
    },

    smStatusBadge(status) {
      const map = {
        pending:   'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
        sent:      'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
        failed:    'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
        cancelled: 'bg-gray-100 text-gray-600 dark:bg-[#21262d] dark:text-[#8b949e]',
      };
      return map[status] || map.cancelled;
    },
  };
}
