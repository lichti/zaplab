// Reaction Tracker — browse message_reactions collection and view emoji analytics.
function reactionsSection() {
  return {
    rx: {
      reactions: [],
      total:     0,
      loading:   false,
      error:     '',
      messageId: '',
      chatJid:   '',
      senderJid: '',
    },
    rxStats: {
      chatJid:    '',
      topEmojis:  [],
      topSenders: [],
      loading:    false,
      error:      '',
    },
    rxTab: 'log', // 'log' | 'stats'

    async rxLoad() {
      this.rx.loading = true;
      this.rx.error   = '';
      try {
        const p = new URLSearchParams({ limit: 200 });
        if (this.rx.messageId) p.set('message_id', this.rx.messageId);
        if (this.rx.chatJid)   p.set('chat_jid',   this.rx.chatJid);
        if (this.rx.senderJid) p.set('sender_jid', this.rx.senderJid);
        const res  = await fetch(`/zaplab/api/reactions?${p}`, { headers: apiHeaders() });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.rx.reactions = data.reactions || [];
        this.rx.total     = data.total     || 0;
      } catch (err) {
        this.rx.error = err.message;
      } finally {
        this.rx.loading = false;
      }
    },

    async rxLoadStats() {
      if (!this.rxStats.chatJid.trim()) return;
      this.rxStats.loading = true;
      this.rxStats.error   = '';
      try {
        const p   = new URLSearchParams({ chat_jid: this.rxStats.chatJid.trim() });
        const res = await fetch(`/zaplab/api/reactions/stats?${p}`, { headers: apiHeaders() });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.rxStats.topEmojis  = data.top_emojis  || [];
        this.rxStats.topSenders = data.top_senders || [];
      } catch (err) {
        this.rxStats.error = err.message;
      } finally {
        this.rxStats.loading = false;
      }
    },

    rxRemovedBadge: cls => cls
      ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
      : 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
  };
}
