// Conversation View
// GET /zaplab/api/conversation/chats  — chat list
// GET /zaplab/api/conversation/names  — {jid: displayName} map
// GET /zaplab/api/conversation?chat=...&limit=...&before=...  — messages
function conversationSection() {
  return {
    // ── state ──
    cvChatsLoading:  false,
    cvChatsError:    '',
    cvChats:         [],
    cvChatFilter:    '',
    cvSelectedChat:  '',
    cvMsgLoading:    false,
    cvMsgError:      '',
    cvMessages:      [],   // all messages including reaction events (ASC order)
    cvReactions:     {},   // {msgID: [{emoji, sender, fromMe}]}
    cvHasMore:       false,
    cvNextBefore:    null,
    cvSelectedMsg:   null,  // raw event drawer
    cvNames:         {},    // {jid: displayName}
    cvOrderAsc:      true,  // true = old→new (default chat order), false = new→old

    // ── init ──
    initConversation() {
      this.$watch('activeSection', val => {
        if (val === 'conversation' && this.cvChats.length === 0) this.cvLoadChats();
        // if navigated here with a chat already selected (from search), load it
        if (val === 'conversation' && this.cvSelectedChat && this.cvMessages.length === 0) {
          this.cvSelectChat(this.cvSelectedChat);
        }
      });
    },

    // ── load chat list + names ──
    async cvLoadChats() {
      if (this.cvChatsLoading) return;
      this.cvChatsLoading = true;
      this.cvChatsError   = '';
      try {
        const [chatsRes, namesRes] = await Promise.all([
          fetch('/zaplab/api/conversation/chats?limit=200', { headers: this.apiHeaders() }),
          fetch('/zaplab/api/conversation/names',           { headers: this.apiHeaders() }),
        ]);
        if (chatsRes.ok) this.cvChats = (await chatsRes.json()).chats || [];
        if (namesRes.ok) this.cvNames = (await namesRes.json()).names || {};
      } catch (err) {
        this.cvChatsError = err.message || 'Load failed';
      } finally {
        this.cvChatsLoading = false;
      }
    },

    // ── select chat and load messages ──
    async cvSelectChat(chat) {
      this.cvSelectedChat = chat;
      this.cvMessages     = [];
      this.cvReactions    = {};
      this.cvHasMore      = false;
      this.cvNextBefore   = null;
      this.cvSelectedMsg  = null;
      await this.cvLoadMessages(chat, null);
    },

    async cvLoadMessages(chat, before) {
      if (this.cvMsgLoading) return;
      this.cvMsgLoading = true;
      this.cvMsgError   = '';
      try {
        const params = new URLSearchParams({ chat, limit: 60 });
        if (before) params.set('before', before);
        const res = await fetch('/zaplab/api/conversation?' + params, { headers: this.apiHeaders() });
        if (res.ok) {
          const d = await res.json();
          const msgs = (d.messages || []).reverse(); // API returns DESC, reverse for ASC display
          if (before) {
            this.cvMessages = [...msgs, ...this.cvMessages];
          } else {
            this.cvMessages = msgs;
          }
          this.cvHasMore    = d.has_more    || false;
          this.cvNextBefore = d.next_before || null;
          this.cvBuildReactionsMap();
        } else {
          this.cvMsgError = `HTTP ${res.status}`;
        }
      } catch (err) {
        this.cvMsgError = err.message || 'Load failed';
      } finally {
        this.cvMsgLoading = false;
      }
    },

    cvLoadMore() {
      if (this.cvHasMore && this.cvNextBefore) {
        const ts = this.cvNextBefore.replace(' ', 'T').replace('Z', '+00:00');
        this.cvLoadMessages(this.cvSelectedChat, ts);
      }
    },

    // Build reactions map: {targetMsgID: [{emoji, sender, fromMe}]}
    cvBuildReactionsMap() {
      const r = {};
      for (const msg of this.cvMessages) {
        if (msg.type === 'reaction' && msg.react_target) {
          if (!r[msg.react_target]) r[msg.react_target] = [];
          r[msg.react_target].push({ emoji: msg.text || '❤️', sender: msg.sender, fromMe: msg.is_from_me });
        }
      }
      this.cvReactions = r;
    },

    // Messages to display: exclude standalone reaction events, apply order.
    cvDisplayMessages() {
      const msgs = this.cvMessages.filter(m => m.type !== 'reaction');
      return this.cvOrderAsc ? msgs : [...msgs].reverse();
    },

    cvToggleOrder() {
      this.cvOrderAsc = !this.cvOrderAsc;
    },

    // CSS classes for the bubble based on message type and direction.
    cvBubbleClass(msg) {
      if (msg.type === 'deleted') {
        return 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 border border-red-200 dark:border-red-800 rounded-xl px-3 py-2 text-xs shadow-sm';
      }
      if (msg.type && msg.type.startsWith('edited_')) {
        return msg.is_from_me
          ? 'bg-yellow-500 dark:bg-yellow-700 text-white rounded-xl rounded-br-sm px-3 py-2 text-xs shadow-sm'
          : 'bg-yellow-50 dark:bg-yellow-900/30 text-gray-800 dark:text-[#e6c97a] border border-yellow-200 dark:border-yellow-700 rounded-xl rounded-bl-sm px-3 py-2 text-xs shadow-sm';
      }
      return msg.is_from_me
        ? 'bg-green-500 dark:bg-green-700 text-white rounded-xl rounded-br-sm px-3 py-2 text-xs shadow-sm'
        : 'bg-white dark:bg-[#21262d] text-gray-800 dark:text-[#c9d1d9] border border-gray-200 dark:border-[#30363d] rounded-xl rounded-bl-sm px-3 py-2 text-xs shadow-sm';
    },

    // ── helpers ──
    cvFilteredChats() {
      if (!this.cvChatFilter) return this.cvChats;
      const f = this.cvChatFilter.toLowerCase();
      return this.cvChats.filter(c => {
        const name = (this.cvNames[c.chat] || '').toLowerCase();
        return name.includes(f) || c.chat.toLowerCase().includes(f);
      });
    },

    cvDisplayName(jid) {
      if (!jid) return '?';
      return this.cvNames[jid] || this.cvShortJID(jid);
    },

    cvMsgIcon(type) {
      const icons = {
        text: '', image: '🖼', video: '▶', audio: '🎵',
        document: '📄', sticker: '🔖', location: '📍',
        reaction: '💬', deleted: '🗑',
      };
      if (type && type.startsWith('edited_')) return '✏️';
      return icons[type] ?? '';
    },

    cvMsgLabel(msg) {
      const base = msg.type && msg.type.startsWith('edited_') ? msg.type.slice(7) : msg.type;
      if (base === 'deleted')  return '';
      if (base === 'reaction') return msg.text || '❤️';
      if (base === 'audio')    return '';
      if (base === 'sticker')  return '';
      if (msg.text)    return msg.text.substring(0, 300);
      if (msg.caption) return msg.caption.substring(0, 300);
      return '';
    },

    cvFormatTime(created) {
      if (!created) return '';
      return new Date(created.replace(' ', 'T')).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    },
    cvFormatDate(created) {
      if (!created) return '';
      return new Date(created.replace(' ', 'T')).toLocaleDateString();
    },
    cvFormatLastMsg(ts) {
      if (!ts) return '';
      const d = new Date(ts.replace(' ', 'T'));
      const now = new Date();
      if (d.toDateString() === now.toDateString()) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
      return d.toLocaleDateString();
    },
    cvShortJID(jid) {
      if (!jid) return '?';
      return jid.replace(/:\d+@/, '@').split('@')[0];
    },
  };
}
