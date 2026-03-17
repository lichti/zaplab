// Network Graph section
// Renders a force-directed graph of the WhatsApp contact/group network derived
// from stored Message events.  Uses a pure-JS Verlet physics simulation and
// draws on an HTML <canvas> element — no external libraries required.
//
// Node types:
//   self      — the device owner (blue, larger)
//   contact   — individual chat partners (orange)
//   group     — group chats (green)
//   broadcast — broadcast lists (purple)
//
// Edges:
//   self ↔ chat   — message count (thickness = weight)
//   sender ↔ group — member appeared in a group conversation
function networkGraphSection() {
  return {
    // ── state ──
    ngLoading:    false,
    ngError:      '',
    ngPeriod:     30,
    ngPeriodOpts: [7, 30, 90, 365, 0],
    ngNodes:      [],
    ngEdges:      [],
    ngNodeMap:    {},
    ngStats:      { total_messages: 0, total_nodes: 0, total_edges: 0 },
    ngSelected:   null,   // hovered/clicked node

    // ── physics constants ──
    ngRepulsion: 9000,
    ngSpring:    0.025,
    ngRestLen:   130,
    ngGravity:   0.012,
    ngDamping:   0.82,

    // ── canvas ──
    ngW:         800,
    ngH:         550,
    ngAnimating: false,
    ngFrame:     null,
    ngDragging:  null,
    ngHovered:   null,

    // ── init ──
    initNetworkGraph() {},

    // ── load ──
    async ngLoad() {
      if (this.ngLoading) return;
      this.ngLoading  = true;
      this.ngError    = '';
      this.ngSelected = null;
      this.ngHovered  = null;
      if (this.ngFrame) cancelAnimationFrame(this.ngFrame);
      try {
        const res = await fetch(
          `/zaplab/api/network/graph?period=${this.ngPeriod}`,
          { headers: this.apiHeaders() }
        );
        if (!res.ok) {
          const d = await res.json();
          this.ngError = d.error || `HTTP ${res.status}`;
          return;
        }
        const data  = await res.json();
        this.ngNodes = data.nodes || [];
        this.ngEdges = data.edges || [];
        this.ngStats = {
          total_messages: data.total_messages || 0,
          total_nodes:    data.total_nodes    || 0,
          total_edges:    data.total_edges    || 0,
        };
        this._ngInitPositions();
        this._ngBuildNodeMap();
        this._ngStartSim();
      } catch (err) {
        this.ngError = err.message || 'Load failed';
      } finally {
        this.ngLoading = false;
      }
    },

    // ── physics ──────────────────────────────────────────────────────────────

    _ngInitPositions() {
      const N = this.ngNodes.length;
      const cx = this.ngW / 2, cy = this.ngH / 2;
      this.ngNodes.forEach((n, i) => {
        if (n.node_type === 'self') {
          n.x = cx; n.y = cy; n.pinned = true;
        } else {
          const angle = (2 * Math.PI * i) / N;
          const r = Math.min(cx, cy) * 0.55;
          n.x = cx + r * Math.cos(angle) + (Math.random() - 0.5) * 30;
          n.y = cy + r * Math.sin(angle) + (Math.random() - 0.5) * 30;
        }
        n.vx = 0; n.vy = 0;
      });
    },

    _ngBuildNodeMap() {
      this.ngNodeMap = {};
      for (const n of this.ngNodes) this.ngNodeMap[n.id] = n;
    },

    _ngStartSim() {
      if (this.ngFrame) cancelAnimationFrame(this.ngFrame);
      this.ngAnimating = true;
      let iter = 0;
      const tick = () => {
        if (!this.ngAnimating) return;
        this._ngTick();
        this._ngRender();
        iter++;
        // Run at full speed for 200 frames, then slow down to ~10fps for updates
        if (iter < 200) {
          this.ngFrame = requestAnimationFrame(tick);
        } else {
          this.ngFrame = setTimeout(() => requestAnimationFrame(tick), 80);
        }
      };
      this.ngFrame = requestAnimationFrame(tick);
    },

    _ngTick() {
      const nodes = this.ngNodes;
      const N = nodes.length;
      for (const n of nodes) { n.fx = 0; n.fy = 0; }

      // Repulsion (all pairs)
      for (let i = 0; i < N; i++) {
        for (let j = i + 1; j < N; j++) {
          const a = nodes[i], b = nodes[j];
          let dx = a.x - b.x, dy = a.y - b.y;
          const d2 = Math.max(dx * dx + dy * dy, 0.25);
          const d  = Math.sqrt(d2);
          const f  = this.ngRepulsion / d2;
          const nx = dx / d, ny = dy / d;
          a.fx += f * nx; a.fy += f * ny;
          b.fx -= f * nx; b.fy -= f * ny;
        }
      }

      // Spring attraction along edges
      for (const ed of this.ngEdges) {
        const a = this.ngNodeMap[ed.source], b = this.ngNodeMap[ed.target];
        if (!a || !b) continue;
        const dx = b.x - a.x, dy = b.y - a.y;
        const d  = Math.sqrt(dx * dx + dy * dy) + 0.001;
        const f  = this.ngSpring * (d - this.ngRestLen);
        a.fx += f * dx / d; a.fy += f * dy / d;
        b.fx -= f * dx / d; b.fy -= f * dy / d;
      }

      // Gravity toward center
      const cx = this.ngW / 2, cy = this.ngH / 2;
      for (const n of nodes) {
        n.fx += this.ngGravity * (cx - n.x);
        n.fy += this.ngGravity * (cy - n.y);
      }

      // Integrate
      for (const n of nodes) {
        if (n.pinned) continue;
        n.vx = (n.vx + n.fx) * this.ngDamping;
        n.vy = (n.vy + n.fy) * this.ngDamping;
        n.x += n.vx;
        n.y += n.vy;
        const r = this._ngRadius(n);
        n.x = Math.max(r + 2, Math.min(this.ngW - r - 2, n.x));
        n.y = Math.max(r + 2, Math.min(this.ngH - r - 2, n.y));
      }
    },

    // ── rendering ────────────────────────────────────────────────────────────

    _ngRender() {
      const canvas = this.$refs && this.$refs.ngCanvas;
      if (!canvas) return;
      const ctx   = canvas.getContext('2d');
      const dark  = document.documentElement.classList.contains('dark');
      const bg    = dark ? '#0d1117' : '#f6f8fa';
      const edgeC = dark ? 'rgba(139,148,158,' : 'rgba(100,116,139,';
      const lblC  = dark ? '#c9d1d9' : '#24292f';

      ctx.clearRect(0, 0, this.ngW, this.ngH);
      ctx.fillStyle = bg;
      ctx.fillRect(0, 0, this.ngW, this.ngH);

      // Edges
      for (const ed of this.ngEdges) {
        const a = this.ngNodeMap[ed.source], b = this.ngNodeMap[ed.target];
        if (!a || !b) continue;
        const alpha = Math.min(0.7, 0.08 + ed.weight / 60);
        ctx.beginPath();
        ctx.moveTo(a.x, a.y);
        ctx.lineTo(b.x, b.y);
        ctx.strokeStyle = edgeC + alpha + ')';
        ctx.lineWidth   = Math.min(4, 0.4 + ed.weight / 15);
        ctx.stroke();
      }

      // Nodes
      for (const n of this.ngNodes) {
        const r = this._ngRadius(n);
        const isHov = this.ngHovered && this.ngHovered.id === n.id;
        const isSel = this.ngSelected && this.ngSelected.id === n.id;

        ctx.beginPath();
        ctx.arc(n.x, n.y, r + (isHov || isSel ? 2 : 0), 0, 2 * Math.PI);
        ctx.fillStyle = this._ngColor(n, dark);
        ctx.fill();

        ctx.strokeStyle = isSel ? (dark ? '#e6edf3' : '#24292f') : (dark ? '#21262d' : '#ffffff');
        ctx.lineWidth   = isSel ? 2.5 : 1.5;
        ctx.stroke();
      }

      // Labels
      ctx.textAlign    = 'center';
      ctx.textBaseline = 'top';
      for (const n of this.ngNodes) {
        const r = this._ngRadius(n);
        const isHov = this.ngHovered && this.ngHovered.id === n.id;
        const isSel = this.ngSelected && this.ngSelected.id === n.id;
        // Always show self label; show others when hovered, selected, or important
        if (n.node_type !== 'self' && !isHov && !isSel && n.msg_count < 5) continue;
        const label = n.label.length > 14 ? n.label.slice(0, 13) + '…' : n.label;
        ctx.font      = (n.node_type === 'self' ? 'bold ' : '') + '10px system-ui,sans-serif';
        ctx.fillStyle = lblC;
        ctx.fillText(label, n.x, n.y + r + 3);
      }
    },

    // ── geometry helpers ─────────────────────────────────────────────────────

    _ngRadius(n) {
      if (n.node_type === 'self')  return 14;
      if (n.node_type === 'group') return Math.min(20, 7 + Math.sqrt(n.msg_count) * 0.9);
      return Math.min(16, 5 + Math.sqrt(n.msg_count) * 0.6);
    },

    _ngColor(n, dark) {
      const d = { self: '#58a6ff', contact: '#ffa657', group: '#3fb950', broadcast: '#bc8cff' };
      const l = { self: '#0550ae', contact: '#953800', group: '#116329', broadcast: '#6639ba'  };
      return (dark ? d : l)[n.node_type] || '#888';
    },

    _ngNodeAt(mx, my) {
      for (const n of this.ngNodes) {
        const dx = mx - n.x, dy = my - n.y;
        const r  = this._ngRadius(n) + 4;
        if (dx * dx + dy * dy <= r * r) return n;
      }
      return null;
    },

    _ngCanvasXY(evt) {
      const canvas = this.$refs && this.$refs.ngCanvas;
      if (!canvas) return { x: 0, y: 0 };
      const rect = canvas.getBoundingClientRect();
      return {
        x: (evt.clientX - rect.left) * (this.ngW / rect.width),
        y: (evt.clientY - rect.top)  * (this.ngH / rect.height),
      };
    },

    // ── mouse events ─────────────────────────────────────────────────────────

    ngOnMouseDown(evt) {
      const { x, y } = this._ngCanvasXY(evt);
      const n = this._ngNodeAt(x, y);
      if (n) {
        this.ngDragging = n;
        n.pinned = true;
        this.ngSelected = n;
        evt.preventDefault();
      } else {
        this.ngSelected = null;
      }
    },

    ngOnMouseMove(evt) {
      const { x, y } = this._ngCanvasXY(evt);
      if (this.ngDragging) {
        this.ngDragging.x = x;
        this.ngDragging.y = y;
        this._ngRender();
      } else {
        const prev = this.ngHovered;
        this.ngHovered = this._ngNodeAt(x, y);
        if (prev !== this.ngHovered) this._ngRender();
      }
    },

    ngOnMouseUp() {
      if (this.ngDragging && this.ngDragging.node_type !== 'self') {
        this.ngDragging.pinned = false;
      }
      this.ngDragging = null;
    },

    ngOnMouseLeave() {
      if (this.ngDragging && this.ngDragging.node_type !== 'self') {
        this.ngDragging.pinned = false;
      }
      this.ngDragging = null;
      this.ngHovered  = null;
      this._ngRender();
    },

    // ── UI helpers ────────────────────────────────────────────────────────────

    ngPeriodLabel(p) {
      if (p === 0) return 'All time';
      return p + 'd';
    },

    ngTypeColor(type) {
      return {
        self:      'bg-blue-500  dark:bg-blue-600',
        contact:   'bg-orange-400 dark:bg-orange-500',
        group:     'bg-green-500 dark:bg-green-600',
        broadcast: 'bg-purple-500 dark:bg-purple-600',
      }[type] || 'bg-gray-400';
    },

    ngNodeEdges(n) {
      if (!n) return [];
      return this.ngEdges.filter(e => e.source === n.id || e.target === n.id);
    },

    ngEdgeLabel(ed, refId) {
      const other = ed.source === refId ? ed.target : ed.source;
      const n = this.ngNodeMap[other];
      return n ? n.label : other;
    },

    ngStopSim() {
      this.ngAnimating = false;
      if (this.ngFrame) { cancelAnimationFrame(this.ngFrame); clearTimeout(this.ngFrame); }
    },

    ngResumeSim() {
      this._ngStartSim();
    },
  };
}
