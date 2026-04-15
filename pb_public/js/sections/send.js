// Send Message section — state, init, preview helpers, submit.
function sendSection() {
  return {
    // ── state ──
    send: {
      to:           '',
      type:         'text',
      message:      '',
      fileData:     '',
      fileName:     '',
      mimeType:     '',
      ptt:          false,
      loading:      false,
      toast:        null,
      result:       null,
      // location fields
      latitude:     '',
      longitude:    '',
      locName:      '',
      locAddress:   '',
      locAccuracy:  10,
      locSpeed:     0,
      locBearing:   0,
      // reply_to fields
      replyEnabled: false,
      replyId:      '',
      replySender:  '',
      replyText:    '',
      // mentions
      mentionsEnabled: false,
      mentionsList:    '',
      // view-once (image, video only)
      viewOnce:        false,
    },
    sendPreviewTab:    localStorage.getItem('zaplab-send-preview-tab') || 'json',
    sendPreviewCopied: false,

    // ── section init ──
    initSend() {
      this.$watch('send.type', () => {
        this.send.fileData     = '';
        this.send.fileName     = '';
        this.send.mimeType     = '';
        this.send.toast        = null;
        this.send.result       = null;
        this.send.latitude     = '';
        this.send.longitude    = '';
        this.send.locName      = '';
        this.send.locAddress   = '';
        this.send.locAccuracy  = 10;
        this.send.locSpeed     = 0;
        this.send.locBearing   = 0;
        this.send.replyEnabled    = false;
        this.send.mentionsEnabled = false;
        this.send.mentionsList    = '';
        this.send.viewOnce        = false;
        if (this.sendPreviewTab === 'response') this.sendPreviewTab = 'json';
      });
      this.$watch('sendPreviewTab', val => {
        localStorage.setItem('zaplab-send-preview-tab', val);
      });
    },

    // ── methods ──
    sendPayload() {
      const up   = '<computed after upload>';
      const mime = this.send.mimeType || '<detected from file>';

      switch (this.send.type) {
        case 'text':
          return { conversation: this.send.message };

        case 'image':
          return {
            imageMessage: {
              caption:       this.send.message,
              url:           up,
              directPath:    up,
              mediaKey:      up,
              mimetype:      mime,
              fileEncSha256: up,
              fileSha256:    up,
              fileLength:    up,
            },
          };

        case 'video':
          return {
            videoMessage: {
              caption:       this.send.message,
              url:           up,
              directPath:    up,
              mediaKey:      up,
              mimetype:      mime,
              fileEncSha256: up,
              fileSha256:    up,
              fileLength:    up,
            },
          };

        case 'audio':
          return {
            audioMessage: {
              url:               up,
              directPath:        up,
              mediaKey:          up,
              mimetype:          (mime !== '<detected from file>' ? mime : 'audio/ogg') + '; codecs=opus',
              fileEncSha256:     up,
              fileSha256:        up,
              fileLength:        up,
              ptt:               this.send.ptt,
              mediaKeyTimestamp: '<unix timestamp>',
            },
          };

        case 'document':
          return {
            documentMessage: {
              caption:       this.send.message,
              url:           up,
              directPath:    up,
              mediaKey:      up,
              mimetype:      mime,
              fileEncSha256: up,
              fileSha256:    up,
              fileLength:    up,
            },
          };

        case 'location':
          return {
            locationMessage: {
              degreesLatitude:  parseFloat(this.send.latitude)  || 0,
              degreesLongitude: parseFloat(this.send.longitude) || 0,
              name:    this.send.locName    || '',
              address: this.send.locAddress || '',
            },
          };

        case 'live_location':
          return {
            liveLocationMessage: {
              degreesLatitude:                   parseFloat(this.send.latitude)  || 0,
              degreesLongitude:                  parseFloat(this.send.longitude) || 0,
              accuracyInMeters:                  this.send.locAccuracy  || 0,
              speedInMps:                        this.send.locSpeed     || 0,
              degreesClockwiseFromMagneticNorth: this.send.locBearing   || 0,
              caption: this.send.message || '',
            },
          };

        default:
          return {};
      }
    },

    sendReplyPayload() {
      if (!this.send.replyEnabled || !this.send.replyId) return null;
      return {
        message_id:  this.send.replyId,
        sender_jid:  this.send.replySender || '',
        quoted_text: this.send.replyText   || '',
      };
    },

    _sendMentions() {
      if (!this.send.mentionsEnabled || !this.send.mentionsList.trim()) return null;
      return this.send.mentionsList.split('\n').map(l => l.trim()).filter(Boolean);
    },

    sendCurlPayload() {
      const fileVal   = this.send.fileData ? `<base64: ${this.send.fileName}>` : '<no file selected>';
      const reply     = this.sendReplyPayload();
      const mentions  = this._sendMentions();
      const withExtra = obj => {
        if (reply)    obj.reply_to = reply;
        if (mentions) obj.mentions = mentions;
        return obj;
      };
      const withMedia = obj => {
        if (reply)               obj.reply_to  = reply;
        if (mentions)            obj.mentions  = mentions;
        if (this.send.viewOnce && ['image','video'].includes(this.send.type)) obj.view_once = true;
        return obj;
      };
      const map = {
        text:          withExtra({ to: this.send.to, message: this.send.message }),
        image:         withMedia({ to: this.send.to, message: this.send.message, image:    fileVal }),
        video:         withMedia({ to: this.send.to, message: this.send.message, video:    fileVal }),
        audio:         withExtra({ to: this.send.to, ptt: this.send.ptt,         audio:    fileVal }),
        document:      withExtra({ to: this.send.to, message: this.send.message, document: fileVal }),
        location:      withExtra({ to: this.send.to, latitude: parseFloat(this.send.latitude) || 0, longitude: parseFloat(this.send.longitude) || 0, name: this.send.locName || '', address: this.send.locAddress || '' }),
        live_location: withExtra({ to: this.send.to, latitude: parseFloat(this.send.latitude) || 0, longitude: parseFloat(this.send.longitude) || 0, accuracy_in_meters: this.send.locAccuracy || 0, speed_in_mps: this.send.locSpeed || 0, degrees_clockwise_from_magnetic_north: this.send.locBearing || 0, caption: this.send.message || '' }),
      };
      return map[this.send.type] || {};
    },

    sendEndpoint() {
      const map = {
        text: '/zaplab/api/sendmessage', image: '/zaplab/api/sendimage', video: '/zaplab/api/sendvideo',
        audio: '/zaplab/api/sendaudio',  document: '/zaplab/api/senddocument',
        location: '/zaplab/api/sendlocation', live_location: '/zaplab/api/sendelivelocation',
      };
      return map[this.send.type] || '/zaplab/api/sendmessage';
    },

    sendJsonPreview() {
      const payload  = this.sendPayload();
      const reply    = this.sendReplyPayload();
      const mentions = this._sendMentions();
      const full = { to: this.send.to || '<jid>', ...payload };
      if (reply)    full.reply_to = reply;
      if (mentions) full.mentions = mentions;
      return this.highlight(full);
    },

    sendCurlPreview() {
      const token   = this.apiToken || '<your-api-token>';
      const url     = `${window.location.origin}${this.sendEndpoint()}`;
      const payload = JSON.stringify(this.sendCurlPayload());
      return [
        `# Authentication enabled — X-API-Token or Dashboard Session required`,
        `curl -X POST \\`,
        `  ${url} \\`,
        `  -H "Content-Type: application/json" \\`,
        `  -H "X-API-Token: ${token}" \\`,
        `  -d '${payload}'`,
      ].join('\n');
    },

    sendResultPreview() {
      if (!this.send.result) {
        return '<span style="color:var(--tw-prose-captions,#8b949e)">No response yet — send a message first.</span>';
      }
      return this.highlight(this.send.result);
    },

    sendPreviewContent() {
      if (this.sendPreviewTab === 'json')     return this.sendJsonPreview();
      if (this.sendPreviewTab === 'response') return this.sendResultPreview();
      return this.highlightCurl(this.sendCurlPreview());
    },

    async copyPreview() {
      let text;
      if (this.sendPreviewTab === 'json')          text = JSON.stringify(this.sendPayload(), null, 2);
      else if (this.sendPreviewTab === 'response') text = JSON.stringify(this.send.result, null, 2);
      else                                         text = this.sendCurlPreview();
      try {
        await navigator.clipboard.writeText(text);
        this.sendPreviewCopied = true;
        setTimeout(() => { this.sendPreviewCopied = false; }, 2000);
      } catch {}
    },

    fileAccept() {
      const map = { image: 'image/*', video: 'video/*', audio: 'audio/*', document: '*/*' };
      return map[this.send.type] || '*/*';
    },

    handleFile(event) {
      const file = event.target.files[0];
      if (!file) return;
      this.send.fileName = `${file.name} (${(file.size / 1024).toFixed(1)} KB)`;
      this.send.mimeType = file.type || 'application/octet-stream';
      const reader = new FileReader();
      reader.onload = e => {
        // Strip "data:<mime>;base64," prefix — API expects raw base64
        this.send.fileData = e.target.result.split(',')[1];
      };
      reader.readAsDataURL(file);
    },

    async submitSend() {
      this.send.toast   = null;
      this.send.loading = true;

      const headers = {
        'Content-Type': 'application/json',
        'X-API-Token':  this.apiToken,
      };

      const reply    = this.sendReplyPayload();
      const mentions = this._sendMentions();
      const withExtra = obj => {
        if (reply)    obj.reply_to = reply;
        if (mentions) obj.mentions = mentions;
        return obj;
      };
      const withMedia = obj => {
        if (reply)               obj.reply_to  = reply;
        if (mentions)            obj.mentions  = mentions;
        if (this.send.viewOnce && ['image','video'].includes(this.send.type)) obj.view_once = true;
        return obj;
      };
      const endpointMap = {
        text:          ['/zaplab/api/sendmessage',      withExtra({ to: this.send.to, message: this.send.message })],
        image:         ['/zaplab/api/sendimage',         withMedia({ to: this.send.to, message: this.send.message, image:    this.send.fileData })],
        video:         ['/zaplab/api/sendvideo',         withMedia({ to: this.send.to, message: this.send.message, video:    this.send.fileData })],
        audio:         ['/zaplab/api/sendaudio',         withExtra({ to: this.send.to, ptt: this.send.ptt,         audio:    this.send.fileData })],
        document:      ['/zaplab/api/senddocument',      withExtra({ to: this.send.to, message: this.send.message, document: this.send.fileData })],
        location:      ['/zaplab/api/sendlocation',      withExtra({ to: this.send.to, latitude: parseFloat(this.send.latitude) || 0, longitude: parseFloat(this.send.longitude) || 0, name: this.send.locName || '', address: this.send.locAddress || '' })],
        live_location: ['/zaplab/api/sendelivelocation', withExtra({ to: this.send.to, latitude: parseFloat(this.send.latitude) || 0, longitude: parseFloat(this.send.longitude) || 0, accuracy_in_meters: this.send.locAccuracy || 0, speed_in_mps: this.send.locSpeed || 0, degrees_clockwise_from_magnetic_north: this.send.locBearing || 0, caption: this.send.message || '' })],
      };

      const [endpoint, body] = endpointMap[this.send.type];

      try {
        const res  = await this.zapFetch(endpoint, { method: 'POST', headers, body: JSON.stringify(body) });
        const data = await res.json();
        this.send.toast = { ok: res.ok, message: data.message || JSON.stringify(data) };
        if (res.ok) {
          this.send.result   = data;
          this.sendPreviewTab = 'response';
          this.send.message  = '';
          this.send.fileData = '';
          this.send.fileName = '';
        }
      } catch (err) {
        this.send.toast = { ok: false, message: err.message };
      } finally {
        this.send.loading = false;
      }
    },
  };
}
