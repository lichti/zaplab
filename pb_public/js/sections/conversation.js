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
    cvMessages:      [],
    cvHasMore:       false,
    cvNextBefore:    null,
    cvSelectedMsg:   null,  // raw event drawer
    cvNames:         {},    // {jid: displayName}

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
          const msgs = (d.messages || []).reverse(); // API returns DESC, reverse for display
          if (before) {
            this.cvMessages = [...msgs, ...this.cvMessages];
          } else {
            this.cvMessages = msgs;
          }
          this.cvHasMore    = d.has_more    || false;
          this.cvNextBefore = d.next_before || null;
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
        // Convert SQLite datetime to RFC3339
        const ts = this.cvNextBefore.replace(' ', 'T').replace('Z', '+00:00');
        this.cvLoadMessages(this.cvSelectedChat, ts);
      }
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

    // Returns the best display name for a JID.
    // Priority: cvNames lookup → stripped JID
    cvDisplayName(jid) {
      if (!jid) return '?';
      return this.cvNames[jid] || this.cvShortJID(jid);
    },

    cvMsgIcon(type) {
      const icons = {
        text: '', image: '🖼', video: '▶', audio: '🎵',
        document: '📄', sticker: '🔖', location: '📍',
        reaction: '💬', deleted: '🗑', unknown: '?',
      };
      if (type && type.startsWith('edited_')) return '✏️';
      return icons[type] ?? '?';
    },

    cvMsgLabel(msg) {
      if (msg.type === 'deleted') return '🗑 mensagem apagada';
      if (msg.type === 'reaction') return msg.text || '❤️';
      if (msg.type === 'audio')    return '🎵 áudio';
      if (msg.type === 'sticker')  return '🔖 figurinha';
      if (msg.text)    return msg.text.substring(0, 120);
      if (msg.caption) return (this.cvMsgIcon(msg.type) + ' ' + msg.caption).substring(0, 120);
      if (msg.type === 'image')    return '🖼 imagem';
      if (msg.type === 'video')    return '▶ vídeo';
      if (msg.type === 'document') return '📄 documento';
      if (msg.type === 'location') return '📍 localização';
      return '?';
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
      // Strip :device suffix and @domain
      return jid.replace(/:\d+@/, '@').split('@')[0];
    },
  };
}
