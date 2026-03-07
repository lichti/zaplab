// Main factory — merges all section factories + shared state + init.
const pb = new PocketBase(window.location.origin);

function zaplab() {
  return Object.assign(
    {},
    utilsSection(),
    pairingSection(),
    accountSection(),
    eventsSection(),
    sendSection(),
    sendRawSection(),
    ctrlSection(),
    contactsSection(),
    groupsSection(),
    {
      // ── shared persistent state ──
      theme:           localStorage.getItem('zaplab-theme')          || 'dark',
      sidebarExpanded: localStorage.getItem('zaplab-sidebar')        !== 'collapsed',
      activeSection:   localStorage.getItem('zaplab-active-section') || 'events',
      apiToken:        localStorage.getItem('zaplab-api-token')      || '',

      // ── shared navigation ──
      toggleTheme() {
        this.theme = this.theme === 'dark' ? 'light' : 'dark';
      },
      toggleSidebar() {
        this.sidebarExpanded = !this.sidebarExpanded;
      },
      setSection(s) {
        this.activeSection = s;
        if (window.innerWidth < 768) this.sidebarExpanded = false;
      },

      // ── init ──
      async init() {
        this.$watch('theme', val => {
          document.documentElement.classList.toggle('dark', val === 'dark');
          localStorage.setItem('zaplab-theme', val);
        });
        this.$watch('sidebarExpanded', val => {
          localStorage.setItem('zaplab-sidebar', val ? 'expanded' : 'collapsed');
        });
        this.$watch('activeSection', val => {
          localStorage.setItem('zaplab-active-section', val);
        });

        this.initPairing();
        this.initAccount();
        this.initSend();
        this.initSendRaw();
        this.initCtrl();
        this.initContacts();
        this.initGroups();

        this.eventsHeight = Math.max(120, Math.floor(window.innerHeight * 0.45));
        if (window.innerWidth < 768) this.sidebarExpanded = false;
        await this.loadInitialEvents();
        this.subscribeEvents();
        this.fetchAccount();
      },
    }
  );
}
