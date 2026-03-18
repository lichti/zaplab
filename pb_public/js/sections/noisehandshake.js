// Noise Handshake Inspector — visualises the WhatsApp Noise_XX_25519_AESGCM_SHA256
// handshake protocol and shows the device's public key material.
function noiseHandshakeSection() {
  return {
    // ── state ──
    nhLoading:       false,
    nhKeys:          null,    // device public key info
    nhError:         '',
    nhConnEvents:    [],      // recent connection-lifecycle frames from ring buffer
    nhCopied:        {},      // per-field copy state

    // Protocol steps — static documentation of the WhatsApp Noise_XX handshake
    nhSteps: [
      {
        id: 'init',
        phase: 'Setup',
        actor: 'client',
        label: 'Initialise Noise state',
        detail: 'Pattern: Noise_XX_25519_AESGCM_SHA256\nPrologue: "WA" + version byte + dict version byte\nState: hash ← SHA256(protocol_name), salt ← hash, key ← null',
        proto: null,
        color: 'gray',
      },
      {
        id: 'client_hello',
        phase: 'Handshake → ClientHello',
        actor: 'client',
        label: 'ClientHello — send ephemeral public key',
        detail: 'Client generates a fresh ephemeral Curve25519 key pair (e).\nAuthenticate(e.pub) → hash ← SHA256(hash ∥ e.pub)\nSend HandshakeMessage { ClientHello { ephemeral: e.pub } } over raw WebSocket.',
        proto: 'waWa6.HandshakeMessage.ClientHello { ephemeral bytes }',
        color: 'blue',
      },
      {
        id: 'server_hello',
        phase: 'Handshake ← ServerHello',
        actor: 'server',
        label: 'ServerHello — receive ephemeral + static (encrypted)',
        detail: 'Server replies with its own ephemeral (se), its static public key (encrypted as ss_cipher), and a certificate chain (cert_cipher).\nAuthenticate(se) → hash ← SHA256(hash ∥ se)\nMixKey(DH(e.priv, se)) → HKDF(DH, salt) → (salt, key)\nss_plain ← Decrypt(ss_cipher) // server static key\nMixKey(DH(e.priv, ss_plain)) → new (salt, key)',
        proto: 'waWa6.HandshakeMessage.ServerHello { ephemeral, static, payload bytes }',
        color: 'purple',
      },
      {
        id: 'cert_verify',
        phase: 'Certificate verification',
        actor: 'client',
        label: 'Verify WhatsApp certificate chain',
        detail: 'cert_plain ← Decrypt(cert_cipher)\nParse waCert.CertChain: intermediate + leaf certificates.\nVerify intermediate signature with hardcoded WACertPubKey (Ed25519).\nVerify leaf signature with intermediate key.\nCheck that leaf.key == ss_plain (server static key).\nThis pins the server identity to WhatsApp\'s root certificate.',
        proto: 'waCert.CertChain { Intermediate, Leaf }',
        color: 'orange',
      },
      {
        id: 'client_finish',
        phase: 'Handshake → ClientFinish',
        actor: 'client',
        label: 'ClientFinish — send noise static key + encrypted payload',
        detail: 'Encrypt(noiseKey.pub) → enc_static\nMixKey(DH(noiseKey.priv, se)) → new (salt, key)\nBuild ClientPayload (device identity, version, platform) and Encrypt it.\nSend HandshakeMessage { ClientFinish { static: enc_static, payload: enc_payload } }.',
        proto: 'waWa6.HandshakeMessage.ClientFinish { static, payload bytes }',
        color: 'green',
      },
      {
        id: 'split',
        phase: 'Key derivation',
        actor: 'both',
        label: 'Split — derive session write/read keys',
        detail: 'HKDF(salt, "") → (writeKey, readKey)\nAll subsequent frames are AES-256-GCM encrypted with a counter-based IV.\nThe handshake is complete; the FrameSocket is promoted to a NoiseSocket.',
        proto: null,
        color: 'teal',
      },
    ],

    // ── init ──
    initNoiseHandshake() {},

    async nhLoad() {
      if (this.nhLoading) return;
      this.nhLoading = true;
      this.nhError = '';
      try {
        const [keysRes, ringRes] = await Promise.all([
          fetch('/zaplab/api/wa/keys', { headers: apiHeaders() }),
          fetch('/zaplab/api/frames/ring?limit=200', { headers: apiHeaders() }),
        ]);
        if (keysRes.ok) {
          this.nhKeys = await keysRes.json();
        } else {
          this.nhError = `Keys unavailable (${keysRes.status})`;
        }
        if (ringRes.ok) {
          const data = await ringRes.json();
          // Filter to connection-lifecycle entries only
          this.nhConnEvents = (data.entries || [])
            .filter(e => {
              const m = (e.Module || '').toLowerCase();
              const msg = (e.Message || '').toLowerCase();
              return m === 'client' && (
                msg.includes('connect') || msg.includes('handshake') ||
                msg.includes('disconnect') || msg.includes('socket') ||
                msg.includes('dial') || msg.includes('auth') ||
                msg.includes('pair') || msg.includes('qr')
              );
            })
            .reverse() // newest first
            .slice(0, 50);
        }
      } catch (err) {
        this.nhError = err.message || 'Load failed';
      } finally {
        this.nhLoading = false;
      }
    },

    async nhRefresh() {
      this.nhKeys = null;
      this.nhConnEvents = [];
      await this.nhLoad();
    },

    async nhCopyKey(field, value) {
      try {
        await navigator.clipboard.writeText(value);
        this.nhCopied[field] = true;
        setTimeout(() => { this.nhCopied[field] = false; }, 2000);
      } catch {}
    },

    nhStepColor(color) {
      const map = {
        gray:   'border-gray-300 dark:border-gray-600',
        blue:   'border-blue-400 dark:border-blue-600',
        purple: 'border-purple-400 dark:border-purple-600',
        orange: 'border-orange-400 dark:border-orange-600',
        green:  'border-green-400 dark:border-green-600',
        teal:   'border-teal-400 dark:border-teal-600',
      };
      return map[color] || map.gray;
    },

    nhActorBadge(actor) {
      if (actor === 'client') return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400';
      if (actor === 'server') return 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400';
      return 'bg-gray-100 text-gray-600 dark:bg-gray-700/30 dark:text-gray-400';
    },

    nhPhaseBullet(color) {
      const map = {
        gray:   'bg-gray-400',
        blue:   'bg-blue-500',
        purple: 'bg-purple-500',
        orange: 'bg-orange-500',
        green:  'bg-green-500',
        teal:   'bg-teal-500',
      };
      return map[color] || map.gray;
    },

    nhConnEventLevel(e) {
      const lvl = (e.Level || 'DEBUG');
      const map = {
        DEBUG: 'text-gray-400 dark:text-gray-600',
        INFO:  'text-blue-600 dark:text-blue-400',
        WARN:  'text-yellow-600 dark:text-yellow-400',
        ERROR: 'text-red-600 dark:text-red-400',
      };
      return map[lvl] || map.DEBUG;
    },

    nhFormatTime(e) {
      const ts = e.time || e.Time;
      if (!ts) return '';
      return new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', fractionalSecondDigits: 3 });
    },

    // apiHeaders — reuse from protoSchemaSection (both mixed into zaplab())
    apiHeaders() {
      const headers = {};
      if (pb.authStore.token) {
        headers['Authorization'] = pb.authStore.token;
      } else if (this.apiToken) {
        headers['X-API-Token'] = this.apiToken;
      }
      return headers;
    },
  };
}
