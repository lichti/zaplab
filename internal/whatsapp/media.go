package whatsapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/gabriel-vasile/mimetype"
	"go.mau.fi/whatsmeow/util/cbcutil"
	"go.mau.fi/whatsmeow/util/hkdfutil"
)

// mediaHMACLength is the number of bytes used for the media MAC (last bytes of encrypted file).
const mediaHMACLength = 10

// mediaAppInfo maps user-friendly type names to WhatsApp HKDF info strings.
var mediaAppInfo = map[string]string{
	"image":    "WhatsApp Image Keys",
	"video":    "WhatsApp Video Keys",
	"audio":    "WhatsApp Audio Keys",
	"document": "WhatsApp Document Keys",
	"sticker":  "WhatsApp Image Keys",
}

// MediaDownloadResult holds decrypted media data and detected MIME info.
type MediaDownloadResult struct {
	Data     []byte
	MimeType string
	Ext      string
	Size     int
}

// DownloadAndDecryptMedia fetches an encrypted WhatsApp CDN file and decrypts it.
// mediaTypeStr must be one of: image, video, audio, document, sticker.
// mediaKeyB64 must be the base64-encoded (standard or URL-safe) media key from the message.
func DownloadAndDecryptMedia(ctx context.Context, mediaURL, mediaKeyB64, mediaTypeStr string) (*MediaDownloadResult, error) {
	// Decode media key (try standard, then URL-safe)
	mediaKey, err := base64.StdEncoding.DecodeString(mediaKeyB64)
	if err != nil {
		mediaKey, err = base64.URLEncoding.DecodeString(mediaKeyB64)
		if err != nil {
			mediaKey, err = base64.RawStdEncoding.DecodeString(mediaKeyB64)
			if err != nil {
				return nil, fmt.Errorf("invalid media_key base64: %w", err)
			}
		}
	}

	appInfo, ok := mediaAppInfo[mediaTypeStr]
	if !ok {
		return nil, fmt.Errorf("unknown media_type %q — use: image, video, audio, document, sticker", mediaTypeStr)
	}

	// Download the encrypted file
	encrypted, err := downloadRawMedia(ctx, mediaURL)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	if len(encrypted) <= mediaHMACLength {
		return nil, fmt.Errorf("downloaded file too small (%d bytes)", len(encrypted))
	}

	// Split: last 10 bytes = MAC, rest = ciphertext
	ciphertext := encrypted[:len(encrypted)-mediaHMACLength]
	mac := encrypted[len(encrypted)-mediaHMACLength:]

	// Derive keys: HKDF-SHA256(mediaKey, salt=nil, info=appInfo, 112 bytes)
	// → iv[0:16] | cipherKey[16:48] | macKey[48:80]
	expanded := hkdfutil.SHA256(mediaKey, nil, []byte(appInfo), 112)
	iv := expanded[:16]
	cipherKey := expanded[16:48]
	macKey := expanded[48:80]

	// Validate HMAC-SHA256(macKey, iv || ciphertext)[:10]
	h := hmac.New(sha256.New, macKey)
	h.Write(iv)
	h.Write(ciphertext)
	if !hmac.Equal(h.Sum(nil)[:mediaHMACLength], mac) {
		return nil, fmt.Errorf("HMAC validation failed — check media_key and media_type")
	}

	// Decrypt AES-256-CBC
	data, err := cbcutil.Decrypt(cipherKey, iv, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	// Detect MIME type
	mt := mimetype.Detect(data)
	return &MediaDownloadResult{
		Data:     data,
		MimeType: mt.String(),
		Ext:      mt.Extension(),
		Size:     len(data),
	}, nil
}

func downloadRawMedia(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "WhatsApp/2.24.6.77 A")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CDN returned HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
