// Media section — download and decrypt WhatsApp CDN media files.
function mediaSection() {
  return {
    // ── state ──
    media: {
      url:       '',
      mediaKey:  '',
      mediaType: 'image',
      loading:   false,
      toast:     null,
    },
    mediaPreviewTab:    localStorage.getItem('zaplab-media-preview-tab') || 'curl',
    mediaPreviewCopied: false,
    mediaResult:        null, // { blobURL, mime, ext, size, type }

    // ── section init ──
    initMedia() {
      this.$watch('mediaPreviewTab', val => {
        localStorage.setItem('zaplab-media-preview-tab', val);
      });
      this.$watch('media.mediaType', () => {
        this.media.toast = null;
      });
    },

    // ── helpers ──
    mediaIsDisabled() {
      return this.media.loading || !this.media.url.trim() || !this.media.mediaKey.trim();
    },

    mediaCurlPreview() {
      const token   = this.apiToken || '<your-api-token>';
      const url     = `${window.location.origin}/media/download`;
      const payload = {
        url:        this.media.url       || '<cdn_url>',
        media_key:  this.media.mediaKey  || '<base64_media_key>',
        media_type: this.media.mediaType,
      };
      return [
        `# Authentication enabled — X-API-Token or Dashboard Session required`,
        `curl -X POST \\`,
        `  ${url} \\`,
        `  -H "Content-Type: application/json" \\`,
        `  -H "X-API-Token: ${token}" \\`,
        `  -d '${JSON.stringify(payload)}' \\`,
        `  --output "media_decrypted"`,
      ].join('\n');
    },

    mediaPreviewContent() {
      return this.highlightCurl(this.mediaCurlPreview());
    },

    async copyMediaPreview() {
      try {
        await navigator.clipboard.writeText(this.mediaCurlPreview());
        this.mediaPreviewCopied = true;
        setTimeout(() => { this.mediaPreviewCopied = false; }, 2000);
      } catch {}
    },

    mediaFormatSize(bytes) {
      if (bytes < 1024) return `${bytes} B`;
      if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
      return `${(bytes / 1024 / 1024).toFixed(2)} MB`;
    },

    // ── submit ──
    async submitMedia() {
      this.media.toast = null;
      this.media.loading = true;
      // revoke previous blob URL to free memory
      if (this.mediaResult?.blobURL) {
        URL.revokeObjectURL(this.mediaResult.blobURL);
        this.mediaResult = null;
      }
      try {
        const res = await this.zapFetch('/zaplab/api/media/download', {
          method:  'POST',
          headers: { 'Content-Type': 'application/json', 'X-API-Token': this.apiToken },
          body:    JSON.stringify({
            url:        this.media.url.trim(),
            media_key:  this.media.mediaKey.trim(),
            media_type: this.media.mediaType,
          }),
        });

        if (!res.ok) {
          const data = await res.json().catch(() => ({ message: `HTTP ${res.status}` }));
          this.media.toast = { ok: false, message: data.message || 'Error' };
          return;
        }

        const mime    = res.headers.get('Content-Type') || 'application/octet-stream';
        const disp    = res.headers.get('Content-Disposition') || '';
        const blob    = await res.blob();
        const blobURL = URL.createObjectURL(blob);

        // Derive ext from Content-Disposition or MIME
        const extMatch = disp.match(/filename="[^"]*(\.[^".]+)"/);
        const ext      = extMatch ? extMatch[1] : ('.' + (mime.split('/')[1]?.split(';')[0] || 'bin'));

        let previewType = 'other';
        if (mime.startsWith('image/'))      previewType = 'image';
        else if (mime.startsWith('audio/')) previewType = 'audio';
        else if (mime.startsWith('video/')) previewType = 'video';

        this.mediaResult = { blobURL, mime, ext, size: blob.size, previewType };
        this.mediaPreviewTab = 'result';
        this.media.toast = {
          ok:      true,
          message: `Decrypted — ${mime} · ${this.mediaFormatSize(blob.size)}`,
        };
      } catch (err) {
        this.media.toast = { ok: false, message: err.message };
      } finally {
        this.media.loading = false;
      }
    },

    mediaDownload() {
      if (!this.mediaResult) return;
      const a      = document.createElement('a');
      a.href       = this.mediaResult.blobURL;
      a.download   = `whatsapp_media${this.mediaResult.ext}`;
      a.click();
    },
  };
}
