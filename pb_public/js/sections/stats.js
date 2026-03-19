// Advanced Stats & Heatmap section
// Renders activity heatmap (dow × hour), daily sparkline, type distribution,
// and summary cards — all without external charting libraries.
function statsSection() {
  return {
    // ── state ──
    stLoading:   false,
    stError:     '',
    stPeriod:    30,       // days; 0 = all time
    stPeriodOpts: [
      { label: '7 days',  value: 7  },
      { label: '30 days', value: 30 },
      { label: '90 days', value: 90 },
      { label: '1 year',  value: 365},
      { label: 'All time',value: 0  },
    ],

    // data
    stSummary:   null,
    stHeatCells: [],       // [{dow, hour, count}]
    stDaily:     [],       // [{day, count}]
    stTypes:     [],       // [{type, count}]

    // heatmap config
    stDayLabels: ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'],
    stHourLabels: ['00','01','02','03','04','05','06','07','08','09','10','11',
                   '12','13','14','15','16','17','18','19','20','21','22','23'],

    // ── init ──
    initStats() {},

    // ── load ──
    async stLoad() {
      if (this.stLoading) return;
      this.stLoading = true;
      this.stError   = '';
      try {
        const p   = this.stPeriod;
        const [sumRes, heatRes, dailyRes, typesRes] = await Promise.all([
          fetch('/zaplab/api/stats/summary',                   { headers: apiHeaders() }),
          fetch(`/zaplab/api/stats/heatmap?period=${p}`,       { headers: apiHeaders() }),
          fetch(`/zaplab/api/stats/daily?days=${p || 365}`,    { headers: apiHeaders() }),
          fetch(`/zaplab/api/stats/types?period=${p}&limit=15`,{ headers: apiHeaders() }),
        ]);
        if (sumRes.ok)   this.stSummary   = await sumRes.json();
        if (heatRes.ok)  this.stHeatCells = (await heatRes.json()).cells || [];
        if (dailyRes.ok) this.stDaily     = (await dailyRes.json()).days  || [];
        if (typesRes.ok) this.stTypes     = (await typesRes.json()).types || [];
      } catch (err) {
        this.stError = err.message || 'Load failed';
      } finally {
        this.stLoading = false;
      }
    },

    async stRefresh() {
      this.stSummary = null;
      this.stHeatCells = [];
      this.stDaily     = [];
      this.stTypes     = [];
      await this.stLoad();
    },

    async stSetPeriod(val) {
      this.stPeriod = val;
      await this.stRefresh();
    },

    // ── heatmap ──
    // Returns the count for a given (dow, hour) pair.
    stHeatCount(dow, hour) {
      const d = String(dow);
      const h = String(hour).padStart(2, '0');
      const cell = this.stHeatCells.find(c => c.dow === d && c.hour === h);
      return cell ? cell.count : 0;
    },

    // Returns the max count across all cells (used to normalise colours).
    stHeatMax() {
      return Math.max(1, ...this.stHeatCells.map(c => c.count));
    },

    // Returns a Tailwind-ish inline background colour for a cell.
    // Scale: 5 levels using HSL (green, same as GitHub).
    stHeatColor(count) {
      if (count === 0) return 'background:#ebedf0';  // light gray (matches dark mode below)
      const ratio = Math.min(count / this.stHeatMax(), 1);
      // 4 levels: faint → deep green
      const level = Math.ceil(ratio * 4);
      const colors = ['','#9be9a8','#40c463','#30a14e','#216e39'];
      return `background:${colors[level]}`;
    },

    stHeatColorDark(count) {
      if (count === 0) return '#161b22';
      const ratio = Math.min(count / this.stHeatMax(), 1);
      const level = Math.ceil(ratio * 4);
      const colors = ['','#0e4429','#006d32','#26a641','#39d353'];
      return colors[level];
    },

    stCellStyle(count) {
      const lightColors = ['#ebedf0','#9be9a8','#40c463','#30a14e','#216e39'];
      const darkColors  = ['#161b22','#0e4429','#006d32','#26a641','#39d353'];
      if (count === 0) return `--heat-light:${lightColors[0]};--heat-dark:${darkColors[0]}`;
      const ratio = Math.min(count / this.stHeatMax(), 1);
      const level = Math.ceil(ratio * 4);
      return `--heat-light:${lightColors[level]};--heat-dark:${darkColors[level]}`;
    },

    // Returns pre-computed row data for the heatmap grid.
    stHeatRows() {
      return this.stDayLabels.map((label, d) => ({
        d,
        label,
        cells: Array.from({length: 24}, (_, h) => ({
          h,
          count: this.stHeatCount(d, h),
        })),
      }));
    },

    stCellTitle(dow, hour, count) {
      if (count === 0) return `${this.stDayLabels[dow]} ${String(hour).padStart(2,'0')}:00 — no messages`;
      return `${this.stDayLabels[dow]} ${String(hour).padStart(2,'0')}:00 — ${count} messages`;
    },

    // Find the peak dow+hour
    stPeakCell() {
      if (!this.stHeatCells.length) return null;
      return this.stHeatCells.reduce((a, b) => b.count > a.count ? b : a);
    },

    stPeakLabel() {
      const p = this.stPeakCell();
      if (!p) return '—';
      const d = parseInt(p.dow, 10);
      return `${this.stDayLabels[d]} ${p.hour}:00`;
    },

    // ── daily sparkline (SVG bar chart) ──
    stSparkSVG() {
      if (!this.stDaily.length) return '';

      const W = 800, H = 80, pad = 2;
      const n = this.stDaily.length;
      const barW = Math.max(1, Math.floor((W - pad * (n - 1)) / n));
      const maxCount = Math.max(1, ...this.stDaily.map(d => d.count));

      const bars = this.stDaily.map((d, i) => {
        const bh = Math.max(2, Math.round((d.count / maxCount) * (H - 4)));
        const x  = i * (barW + pad);
        const y  = H - bh;
        const fill = d.count === 0 ? '#30363d' : '#238636';
        return `<rect x="${x}" y="${y}" width="${barW}" height="${bh}" fill="${fill}" rx="1"><title>${d.day}: ${d.count}</title></rect>`;
      }).join('');

      return `<svg viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" width="100%" height="${H}">${bars}</svg>`;
    },

    stDailyMax() {
      return this.stDaily.length ? Math.max(...this.stDaily.map(d => d.count)) : 0;
    },

    stDailyTotal() {
      return this.stDaily.reduce((s, d) => s + d.count, 0);
    },

    stDailyAvg() {
      if (!this.stDaily.length) return 0;
      return (this.stDailyTotal() / this.stDaily.length).toFixed(1);
    },

    // Labels for first/last day in sparkline
    stSparkFirstDay() { return this.stDaily.length ? this.stDaily[0].day : ''; },
    stSparkLastDay()  { return this.stDaily.length ? this.stDaily[this.stDaily.length - 1].day : ''; },

    // ── type distribution ──
    stTypesMax() {
      return this.stTypes.length ? Math.max(...this.stTypes.map(t => t.count)) : 1;
    },

    stTypeBarWidth(count) {
      return Math.max(2, Math.round((count / this.stTypesMax()) * 100));
    },

    stTypeColor(type) {
      const map = {
        Message:        'bg-blue-500',
        SentMessage:    'bg-green-500',
        Receipt:        'bg-teal-500',
        Connected:      'bg-emerald-500',
        Disconnected:   'bg-red-500',
        HistorySync:    'bg-purple-500',
        AppStateSync:   'bg-indigo-500',
        Presence:       'bg-cyan-500',
        PairSuccess:    'bg-lime-500',
        QRScannedWithoutMultidevice: 'bg-yellow-500',
      };
      return map[type] || 'bg-gray-400';
    },

    // ── summary helpers ──
    stFmt(n) {
      if (n == null) return '—';
      return Number(n).toLocaleString();
    },

    stRate(part, total) {
      if (!total || !part) return '0%';
      return Math.round((part / total) * 100) + '%';
    },
  };
}
