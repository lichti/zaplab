// Mention Tracker — browse @mention events and stats.
function mentionTrackerSection() {
  return {
    mt: {
      mentions:     [],
      total:        0,
      loading:      false,
      error:        '',
      mentionedJid: '',
      chatJid:      '',
      onlyBot:      false,
    },
    mtStats: {
      topMentioned: [],
      botByChat:    [],
      loading:      false,
      error:        '',
    },
    mtTab: 'log', // 'log' | 'stats'

    async mtLoad() {
      this.mt.loading = true;
      this.mt.error   = '';
      try {
        const p = new URLSearchParams({ limit: 200 });
        if (this.mt.mentionedJid) p.set('mentioned_jid', this.mt.mentionedJid);
        if (this.mt.chatJid)      p.set('chat_jid',      this.mt.chatJid);
        if (this.mt.onlyBot)      p.set('is_bot',        'true');
        const res  = await fetch(`/zaplab/api/mentions?${p}`, { headers: apiHeaders() });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.mt.mentions = data.mentions || [];
        this.mt.total    = data.total    || 0;
      } catch (err) {
        this.mt.error = err.message;
      } finally {
        this.mt.loading = false;
      }
    },

    async mtLoadStats() {
      this.mtStats.loading = true;
      this.mtStats.error   = '';
      try {
        const res  = await fetch('/zaplab/api/mentions/stats', { headers: apiHeaders() });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Failed');
        this.mtStats.topMentioned = data.top_mentioned || [];
        this.mtStats.botByChat    = data.bot_by_chat   || [];
      } catch (err) {
        this.mtStats.error = err.message;
      } finally {
        this.mtStats.loading = false;
      }
    },

    mtBotBadge: isBot => isBot
      ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
      : 'bg-gray-100 text-gray-600 dark:bg-[#21262d] dark:text-[#8b949e]',
  };
}
