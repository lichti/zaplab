// Simulation section — GPX route live location simulation.
// WARNING: Work in progress — not yet fully functional.
// WhatsApp live location updates may not behave correctly across all clients.
function simulationSection() {
  return {
    // ── state ──
    sim: {
      to:             '',
      gpxBase64:      '',
      gpxFileName:    '',
      gpxFirstLat:    null,
      gpxFirstLon:    null,
      gpxPointCount:  0,
      speedKmh:       60,
      intervalSecs:   5,
      caption:        '',
      loading:        false,
      toast:          null,
    },
    simActive:           null,   // active simulation object from backend
    simPollingInterval:  null,
    simPreviewTab:       localStorage.getItem('zaplab-sim-preview-tab') || 'curl-init',
    simPreviewCopied:    false,

    // ── init ──
    initSimulation() {
      this.$watch('simPreviewTab', val => {
        localStorage.setItem('zaplab-sim-preview-tab', val);
      });
    },

    // ── GPX file loading ──
    simLoadGPX(event) {
      const file = event.target.files[0];
      if (!file) return;
      this.sim.gpxFileName   = file.name;
      this.sim.gpxBase64     = '';
      this.sim.gpxFirstLat   = null;
      this.sim.gpxFirstLon   = null;
      this.sim.gpxPointCount = 0;
      this.sim.toast         = null;

      const reader = new FileReader();
      reader.onload = (e) => {
        const text   = e.target.result;
        const points = this.simParseGPXPoints(text);
        this.sim.gpxPointCount = points.length;
        if (points.length > 0) {
          this.sim.gpxFirstLat = points[0].lat;
          this.sim.gpxFirstLon = points[0].lon;
        }
        // btoa requires latin1 — use TextEncoder for UTF-8 safe base64
        const bytes  = new TextEncoder().encode(text);
        const binary = Array.from(bytes).map(b => String.fromCharCode(b)).join('');
        this.sim.gpxBase64 = btoa(binary);
      };
      reader.readAsText(file);
    },

    simParseGPXPoints(xml) {
      try {
        const doc    = new DOMParser().parseFromString(xml, 'application/xml');
        const prefer = ['trkpt', 'rtept', 'wpt'];
        for (const tag of prefer) {
          const nodes = doc.querySelectorAll(tag);
          if (nodes.length > 0) {
            return Array.from(nodes).map(p => ({
              lat: parseFloat(p.getAttribute('lat')),
              lon: parseFloat(p.getAttribute('lon')),
            }));
          }
        }
        return [];
      } catch { return []; }
    },

    // ── curl previews ──
    simCurlInit() {
      const token   = this.apiToken || '<your-api-token>';
      const payload = {
        to:                  this.sim.to        || '<jid>',
        latitude:            this.sim.gpxFirstLat ?? '<lat>',
        longitude:           this.sim.gpxFirstLon ?? '<lon>',
        accuracy_in_meters:  10,
        sequence_number:     1,
        caption:             this.sim.caption  || '',
      };
      return [
        `# Step 1 — establish the live location share`,
        `curl -X POST \\`,
        `  ${window.location.origin}/sendelivelocation \\`,
        `  -H "Content-Type: application/json" \\`,
        `  -H "X-API-Token: ${token}" \\`,
        `  -d '${JSON.stringify(payload)}'`,
      ].join('\n');
    },

    simCurlStart() {
      const token   = this.apiToken || '<your-api-token>';
      const payload = {
        to:               this.sim.to      || '<jid>',
        gpx_base64:       this.sim.gpxBase64 ? '<gpx_base64>' : '<base64 encoded GPX>',
        speed_kmh:        this.sim.speedKmh,
        interval_seconds: this.sim.intervalSecs,
        caption:          this.sim.caption || '',
        message_id:       '<send_response.ID from step 1>',
      };
      return [
        `# Step 2 — start the route simulation`,
        `curl -X POST \\`,
        `  ${window.location.origin}/simulate/route \\`,
        `  -H "Content-Type: application/json" \\`,
        `  -H "X-API-Token: ${token}" \\`,
        `  -d '${JSON.stringify(payload)}'`,
      ].join('\n');
    },

    simPreviewContent() {
      if (this.simPreviewTab === 'curl-init')  return this.highlightCurl(this.simCurlInit());
      if (this.simPreviewTab === 'curl-start') return this.highlightCurl(this.simCurlStart());
      if (this.simPreviewTab === 'active' && this.simActive) return this.highlight(this.simActive);
      return '<span style="color:var(--tw-prose-captions,#8b949e)">No active simulation yet.</span>';
    },

    async copySimPreview() {
      const text = this.simPreviewTab === 'curl-start' ? this.simCurlStart()
                 : this.simPreviewTab === 'active'     ? JSON.stringify(this.simActive, null, 2)
                 : this.simCurlInit();
      try {
        await navigator.clipboard.writeText(text);
        this.simPreviewCopied = true;
        setTimeout(() => { this.simPreviewCopied = false; }, 2000);
      } catch {}
    },

    // ── start ──
    async simStart() {
      if (!this.sim.to)         { this.sim.toast = { ok: false, message: 'Destination (to) is required' }; return; }
      if (!this.sim.gpxBase64)  { this.sim.toast = { ok: false, message: 'GPX file is required' }; return; }
      if (this.sim.gpxFirstLat === null) { this.sim.toast = { ok: false, message: 'GPX has no valid points' }; return; }

      this.sim.loading = true;
      this.sim.toast   = null;

      try {
        // Step 1 — send first live location to establish the share in WhatsApp
        //           and capture the message ID needed for subsequent updates.
        const initRes = await fetch('/sendelivelocation', {
          method:  'POST',
          headers: { 'Content-Type': 'application/json', 'X-API-Token': this.apiToken },
          body: JSON.stringify({
            to:                  this.sim.to,
            latitude:            this.sim.gpxFirstLat,
            longitude:           this.sim.gpxFirstLon,
            accuracy_in_meters:  10,
            sequence_number:     1,
            caption:             this.sim.caption,
          }),
        });
        const initData = await initRes.json();
        if (!initRes.ok) {
          this.sim.toast = { ok: false, message: initData.message || 'Failed to establish live location' };
          return;
        }
        const messageID = initData.send_response?.ID || '';
        if (!messageID) {
          this.sim.toast = { ok: false, message: 'Could not retrieve message ID from live location response' };
          return;
        }

        // Step 2 — start the backend simulation loop, passing the original message ID
        //           so each update reuses it and WhatsApp moves the pin instead of
        //           creating a new message.
        const simRes  = await fetch('/simulate/route', {
          method:  'POST',
          headers: { 'Content-Type': 'application/json', 'X-API-Token': this.apiToken },
          body: JSON.stringify({
            to:               this.sim.to,
            gpx_base64:       this.sim.gpxBase64,
            speed_kmh:        this.sim.speedKmh,
            interval_seconds: this.sim.intervalSecs,
            caption:          this.sim.caption,
            message_id:       messageID,
          }),
        });
        const simData = await simRes.json();
        if (!simRes.ok) {
          this.sim.toast = { ok: false, message: simData.message || 'Failed to start simulation' };
          return;
        }

        this.simActive       = simData.simulation;
        this.simPreviewTab   = 'active';
        const km  = simData.simulation.total_distance_km.toFixed(2);
        const min = Math.ceil(simData.simulation.estimated_minutes);
        this.sim.toast = { ok: true, message: `Simulation started — ${km} km · ~${min} min` };
        this.simStartPolling();

      } catch (err) {
        this.sim.toast = { ok: false, message: err.message };
      } finally {
        this.sim.loading = false;
      }
    },

    // ── stop ──
    async simStop() {
      if (!this.simActive) return;
      try {
        await fetch(`/simulate/route/${this.simActive.id}`, {
          method:  'DELETE',
          headers: { 'X-API-Token': this.apiToken },
        });
      } catch {}
      this.simStopPolling();
      this.simActive     = null;
      this.simPreviewTab = 'curl-init';
      this.sim.toast     = { ok: true, message: 'Simulation stopped' };
    },

    // ── polling ──
    simStartPolling() {
      this.simStopPolling();
      this.simPollingInterval = setInterval(async () => {
        try {
          const res  = await fetch('/simulate/route', { headers: { 'X-API-Token': this.apiToken } });
          const data = await res.json();
          const found = (data.simulations || []).find(s => s.id === this.simActive?.id);
          if (!found) {
            this.simStopPolling();
            this.simActive     = null;
            this.simPreviewTab = 'curl-init';
            this.sim.toast     = { ok: true, message: 'Route completed — simulation finished' };
          }
        } catch {}
      }, 5000);
    },

    simStopPolling() {
      if (this.simPollingInterval) {
        clearInterval(this.simPollingInterval);
        this.simPollingInterval = null;
      }
    },
  };
}
