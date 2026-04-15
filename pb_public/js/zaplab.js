// Main factory — merges all section factories + shared state + init.
const pb = new PocketBase(window.location.origin);
// PocketBase by default uses LocalAuthStore in the browser, but we'll be explicit
// to ensure persistence across tabs and refreshes.

function zaplab() {
  return Object.assign(
    {},
    utilsSection(),
    authSection(),
    dashboardSection(),
    pairingSection(),
    accountSection(),
    eventsSection(),
    eventBrowserSection(),
    errorBrowserSection(),
    msgHistorySection(),
    sendSection(),
    sendRawSection(),
    ctrlSection(),
    spoofSection(),
    contactsSection(),
    contactsMgmtSection(),
    groupsSection(),
    mediaSection(),
    simulationSection(),
    webhookSection(),
    settingsSection(),
    dbExplorerSection(),
    protocolTimelineSection(),
    protoSchemaSection(),
    frameCaptureSection(),
    noiseHandshakeSection(),
    signalSessionsSection(),
    annotationsSection(),
    statsSection(),
    appStateSection(),
    sessionComparatorSection(),
    networkGraphSection(),
    scriptingSection(),
    triggersSection(),
    cronSchedulerSection(),
    searchSection(),
    conversationSection(),
    gallerySection(),
    presenceTimelineSection(),
    receiptsSection(),
    exportSection(),
    dbSandboxSection(),
    connStabilitySection(),
    scriptsIOSection(),
    waHealthSection(),
    framesIQSection(),
    groupMembershipSection(),
    auditLogSection(),
    contactOverviewSection(),
    groupOverviewSection(),
    activityTrackerSection(),
    reactionsSection(),
    mentionTrackerSection(),
    scheduledMsgsSection(),
    contactCacheSection(),
    webhookDeliveriesSection(),
    autoReplySection(),
    notificationsSection(),
    {
      // ── shared persistent state ──
      theme:           localStorage.getItem('zaplab-theme')          || 'dark',
      sidebarExpanded: localStorage.getItem('zaplab-sidebar')        !== 'collapsed',
      activeSection:   window.location.hash.replace('#/', '')        || localStorage.getItem('zaplab-active-section') || 'events',
      apiToken:        localStorage.getItem('zaplab-api-token')      || '',
      isLoggedIn:      pb.authStore.isValid,

      // ── shared navigation ──
      toggleTheme() {
        this.theme = this.theme === 'dark' ? 'light' : 'dark';
      },
      toggleSidebar() {
        this.sidebarExpanded = !this.sidebarExpanded;
      },
      setSection(s) {
        window.location.hash = `#/${s}`;
      },

      // ── init ──
      async init() {
        // Global error interceptor for 401/403
        // If the server rejects the token, we should log out.
        pb.authStore.onChange((token, model) => {
          this.isLoggedIn = !!token;
        });

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

        // Sync hash to activeSection
        window.addEventListener('hashchange', () => {
          const s = window.location.hash.replace('#/', '');
          if (s && s !== this.activeSection) {
            this.activeSection = s;
            if (window.innerWidth < 768) this.sidebarExpanded = false;
          }
        });

        // Ensure current hash is applied if present
        if (window.location.hash) {
          this.activeSection = window.location.hash.replace('#/', '');
        } else {
          // If no hash, set it from current activeSection
          window.location.hash = `#/${this.activeSection}`;
        }

        await this.initAuth();
        this.initDashboard();
        this.initPairing();
        this.initAccount();
        this.initEventBrowser();
        this.initErrorBrowser();
        this.initMsgHistory();
        this.initSend();
        this.initSendRaw();
        this.initCtrl();
        this.initSpoof();
        this.initContacts();
        this.initContactsMgmt();
        this.initGroups();
        this.initMedia();
        this.initSimulation();
        this.initWebhook();
        this.initSettings();
        this.initDBExplorer();
        this.initProtocolTimeline();
        this.initProtoSchema();
        this.initFrameCapture();
        this.initNoiseHandshake();
        this.initSignalSessions();
        this.initAnnotations();
        this.initStats();
        this.initAppState();
        this.initSessionComparator();
        this.initNetworkGraph();
        this.initScripting();
        this.initTriggers();
        this.initCronScheduler();
        this.initSearch();
        this.initConversation();
        this.initGallery();
        this.initPresenceTimeline();
        this.initReceipts();
        this.initExport();
        this.initDBSandbox();
        this.initConnStability();
        this.initWAHealth();
        this.initFramesIQ();
        this.initGroupMembership();
        this.initAuditLog();
        this.initContactOverview();
        this.initGroupOverview();
        this.initActivityTracker();

        this.initNotifications();

        this.eventsHeight = Math.max(120, Math.floor(window.innerHeight * 0.45));
        if (window.innerWidth < 768) this.sidebarExpanded = false;
        await this.loadInitialEvents();
        this.subscribeEvents();
        this.fetchAccount();
      },
    }
  );
}

// Alpine store for cross-section navigation.
// Sections use Alpine.store('nav').X to pass state to each other
// without needing access to the parent zaplab() scope.
document.addEventListener('alpine:init', () => {
  Alpine.store('nav', {
    cvSelectedChat: '',
    srChat:         '',
    srQuery:        '',
    coJID:          '',
    gmtJID:         '',
  });
});

