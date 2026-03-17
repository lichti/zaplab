// Conversation View
// GET /zaplab/api/conversation/chats  — chat list
// GET /zaplab/api/conversation/names  — {jid: displayName} map
// GET /zaplab/api/conversation?chat=...&limit=...&before=...  — messages
function conversationSection() {
  return {
    // ── state ──
    cvChatsLoading: false,
    cvChatsError:   '',
    cvChats:        [],
    cvChatFilter:   '',
    cvSelectedChat: '',
    cvMsgLoading:   false,
    cvMsgError:     '',
    cvMessages:     [],  // all events in ASC order (including edits/deletes/reactions)
    cvReactions:    {},  // {msgID: [{emoji, sender}]}            — for chips in the bubble
    cvEdits:        {},  // {originalMsgID: {type, text, created}} — for bubble coloring (last wins)
    cvReactMap:     {},  // {msgID: [fullMsg, ...]}                — for detail panel
    cvEditMap:      {},  // {originalMsgID: [fullMsg, ...]}        — for detail panel
    cvHasMore:      false,
    cvNextBefore:   null,
    cvSelectedMsg:  null,
    cvNames:        {},
    cvOrderAsc:     true, // true = old→new, false = new→old

    // ── init ──
    initConversation() {
      this.$watch('activeSection', val => {
        if (val === 'conversation' && this.cvChats.length === 0) this.cvLoadChats();
        if (val === 'conversation' && this.cvSelectedChat && this.cvMessages.length === 0)
          this.cvSelectChat(this.cvSelectedChat);
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

    // ── select chat ──
    async cvSelectChat(chat) {
      this.cvSelectedChat = chat;
      this.cvMessages     = [];
      this.cvReactions    = {};
      this.cvEdits        = {};
      this.cvReactMap     = {};
      this.cvEditMap      = {};
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
          const d    = await res.json();
          const msgs = (d.messages || []).reverse();
          this.cvMessages    = before ? [...msgs, ...this.cvMessages] : msgs;
          this.cvHasMore     = d.has_more    || false;
          this.cvNextBefore  = d.next_before || null;
          this.cvBuildMaps();
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

    // ── build annotation maps ──
    // cvReactions / cvReactMap  — keyed by target msgID, for chips and detail panel
    // cvEdits / cvEditMap       — keyed by original msgID; last write wins for bubble color
    cvBuildMaps() {
      const reactions = {};
      const reactMap  = {};
      const edits     = {};
      const editMap   = {};
      for (const msg of this.cvMessages) {
        if (msg.type === 'reaction' && msg.react_target) {
          if (!reactions[msg.react_target]) reactions[msg.react_target] = [];
          if (!reactMap[msg.react_target])  reactMap[msg.react_target]  = [];
          reactions[msg.react_target].push({ emoji: msg.text || '❤️', sender: msg.sender });
          reactMap[msg.react_target].push(msg);
        }
        if ((msg.type === 'deleted' || (msg.type && msg.type.startsWith('edited_'))) && msg.edit_target) {
          edits[msg.edit_target] = { type: msg.type, text: msg.text || '', created: msg.created };
          if (!editMap[msg.edit_target]) editMap[msg.edit_target] = [];
          editMap[msg.edit_target].push(msg);
        }
      }
      this.cvReactions = reactions;
      this.cvReactMap  = reactMap;
      this.cvEdits     = edits;
      this.cvEditMap   = editMap;
    },

    // ── display helpers ──

    // Returns messages to show: filters out standalone reactions and edit/delete events
    // (those are rendered as annotations on the original bubble), then applies sort order.
    cvDisplayMessages() {
      const msgs = this.cvMessages.filter(m =>
        m.type !== 'reaction' &&
        m.type !== 'deleted' &&
        !(m.type && m.type.startsWith('edited_'))
      );
      return this.cvOrderAsc ? msgs : [...msgs].reverse();
    },

    cvToggleOrder() { this.cvOrderAsc = !this.cvOrderAsc; },

    // Navigate to Event Browser pre-filtered by msgID.
    cvOpenInEventBrowser(msgId) {
      if (!msgId) return;
      this.eb.filterMsgID = msgId;
      // Reset other filters so the result is clean
      this.eb.filterType = '';
      this.eb.filterText = '';
      this.setSection('eventbrowser');
      this.ebSearch();
    },

    // Effective state of a message considering later edits/deletes.
    // Returns null if no annotation, or {type, text, created} from cvEdits.
    cvAnnotation(msg) { return this.cvEdits[msg.msgID] || null; },

    // CSS classes for the bubble. Checks cvEdits to color original messages.
    cvBubbleClass(msg) {
      const ann = this.cvAnnotation(msg);
      const isDel     = ann && ann.type === 'deleted';
      const isEdited  = ann && ann.type && ann.type.startsWith('edited_');

      if (isDel) {
        return 'rounded-xl px-3 py-2 text-xs shadow-sm ' +
          'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 border border-red-200 dark:border-red-800';
      }
      if (isEdited) {
        return 'rounded-xl px-3 py-2 text-xs shadow-sm ' + (msg.is_from_me
          ? 'bg-yellow-400 dark:bg-yellow-700 text-white rounded-br-sm'
          : 'bg-yellow-50 dark:bg-yellow-900/30 text-gray-800 dark:text-[#e6c97a] border border-yellow-300 dark:border-yellow-700 rounded-bl-sm');
      }
      return 'rounded-xl px-3 py-2 text-xs shadow-sm ' + (msg.is_from_me
        ? 'bg-green-500 dark:bg-green-700 text-white rounded-br-sm'
        : 'bg-white dark:bg-[#21262d] text-gray-800 dark:text-[#c9d1d9] border border-gray-200 dark:border-[#30363d] rounded-bl-sm');
    },

    // CSS classes for the annotation sub-bubble inside a modified message.
    cvAnnotationClass(ann, fromMe) {
      if (ann.type === 'deleted') {
        return 'mt-1.5 pt-1.5 border-t border-red-300 dark:border-red-700 text-[10px] italic opacity-80';
      }
      return 'mt-1.5 pt-1.5 border-t text-[10px] ' + (fromMe
        ? 'border-yellow-300 dark:border-yellow-600'
        : 'border-yellow-300 dark:border-yellow-700');
    },

    cvMsgLabel(msg) {
      if (msg.text)    return msg.text.substring(0, 400);
      if (msg.caption) return msg.caption.substring(0, 400);
      return '';
    },

    cvMsgIcon(type) {
      if (!type) return '';
      const base = type.startsWith('edited_') ? type.slice(7) : type;
      const icons = { image: '🖼', video: '▶', audio: '🎵', document: '📄', sticker: '🔖', location: '📍', contact: '👤', poll: '📊', poll_vote: '🗳', enc_reaction: '🔒' };
      return icons[base] || '';
    },

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
      if (d.toDateString() === new Date().toDateString())
        return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
      return d.toLocaleDateString();
    },
    cvShortJID(jid) {
      if (!jid) return '?';
      return jid.replace(/:\d+@/, '@').split('@')[0];
    },
  };
}
