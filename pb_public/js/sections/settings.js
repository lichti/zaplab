// Settings section — General application configuration.
function settingsSection() {
  return {
    config: {
      recover_edits:   false,
      recover_deletes: false,
      apiToken:        localStorage.getItem('zaplab-api-token') || '',
      loading:         false,
    },

    saveApiToken() {
      localStorage.setItem('zaplab-api-token', this.config.apiToken);
      this.apiToken = this.config.apiToken; // Sync with global state
    },

    async initSettings() {
      await this.fetchConfig();
    },

    async fetchConfig() {
      this.config.loading = true;
      try {
        const res = await fetch('/zaplab/api/config', {
          headers: { 'X-API-Token': this.apiToken }
        });
        const data = await res.json();
        if (res.ok) {
          this.config.recover_edits   = data.recover_edits;
          this.config.recover_deletes = data.recover_deletes;
        }
      } catch (err) {
        console.error('Failed to fetch config:', err);
      } finally {
        this.config.loading = false;
      }
    },

    async saveConfig(payload) {
      this.config.loading = true;
      try {
        const res = await fetch('/zaplab/api/config', {
          method:  'PUT',
          headers: {
            'Content-Type': 'application/json',
            'X-API-Token':  this.apiToken
          },
          body: JSON.stringify(payload)
        });
        const data = await res.json();
        if (res.ok) {
          this.config.recover_edits   = data.recover_edits;
          this.config.recover_deletes = data.recover_deletes;
        }
      } catch (err) {
        console.error('Failed to save config:', err);
      } finally {
        this.config.loading = false;
      }
    },

    toggleRecoverEdits() {
      this.config.recover_edits = !this.config.recover_edits;
      this.saveConfig({ recover_edits: this.config.recover_edits });
    },

    toggleRecoverDeletes() {
      this.config.recover_deletes = !this.config.recover_deletes;
      this.saveConfig({ recover_deletes: this.config.recover_deletes });
    }
  };
}
