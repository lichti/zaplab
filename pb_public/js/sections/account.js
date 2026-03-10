// Account section — connected account info, profile picture, about text, logout.
function accountSection() {
  return {
    // ── state ──
    account: {
      jid:          '',
      phone:        '',
      push_name:    '',
      business_name:'',
      platform:     '',
      status:       '',
      avatar_url:   '',
      loading:      false,
      error:        '',
    },
    accountAvatarBroken: false,

    // ── section init ──
    initAccount() {
      this.$watch('activeSection', val => {
        if (val === 'account' && !this.account.jid && !this.account.loading) {
          this.fetchAccount();
        }
      });
    },

    // ── methods ──
    async fetchAccount() {
      this.account.loading = true;
      this.account.error   = '';
      this.accountAvatarBroken = false;
      try {
        const res  = await fetch('/zaplab/api/wa/account');
        const data = await res.json();
        if (!res.ok) {
          this.account.error = data.message || 'Failed to load account info';
          return;
        }
        Object.assign(this.account, data);
      } catch (err) {
        this.account.error = err.message;
      } finally {
        this.account.loading = false;
      }
    },

    // Returns initials from push_name for the avatar fallback.
    accountInitials() {
      const name = this.account.push_name || this.account.phone || '?';
      return name.split(' ').slice(0, 2).map(w => w[0]).join('').toUpperCase();
    },

    // Formats the phone number as +XX XX XXXXX-XXXX (best-effort).
    accountPhoneFormatted() {
      const p = this.account.phone;
      if (!p) return '';
      if (p.length === 13) return `+${p.slice(0,2)} ${p.slice(2,4)} ${p.slice(4,9)}-${p.slice(9)}`;
      if (p.length === 12) return `+${p.slice(0,2)} ${p.slice(2,4)} ${p.slice(4,8)}-${p.slice(8)}`;
      return `+${p}`;
    },
  };
}
