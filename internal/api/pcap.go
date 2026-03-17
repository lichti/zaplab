package api

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// ── PCAP Export ───────────────────────────────────────────────────────────────
//
// Exports frame log entries from the PocketBase `frames` collection as a
// libpcap (PCAP) file that can be opened directly in Wireshark or tcpdump.
//
// Each frame log entry is wrapped in a synthetic Ethernet/IPv4/UDP packet:
//
//	EtherType  0x0800 (IPv4)
//	src MAC    52:4c:41:42:00:01  (ASCII "RLAB" + \x00\x01)
//	dst MAC    52:4c:41:42:00:02
//	src IP     127.0.0.1
//	dst IP     127.0.0.2
//	protocol   UDP
//	src port   443  (WhatsApp WebSocket)
//	dst port   12345
//
// UDP payload: JSON object {module, level, seq, msg, created}.
// The resulting file is readable in Wireshark; payload is visible under
// the UDP "Data" field and can be decoded as plain text.

const (
	pcapMagicLE      = 0xa1b2c3d4 // little-endian, microsecond timestamps
	pcapLinkEthernet = 1
)

// getFramesPCAP exports the `frames` collection as a PCAP file download.
//
// Query params:
//
//	module  string  filter by module substring (optional)
//	level   string  filter by exact level: DEBUG, INFO, WARN, ERROR (optional)
//	limit   int     max entries to export (default 1000, max 10000)
func getFramesPCAP(e *core.RequestEvent) error {
	moduleFilter := e.Request.URL.Query().Get("module")
	levelFilter := e.Request.URL.Query().Get("level")
	limit, _ := strconv.Atoi(e.Request.URL.Query().Get("limit"))
	if limit < 1 || limit > 10000 {
		limit = 1000
	}

	where := "1=1"
	if moduleFilter != "" {
		where += fmt.Sprintf(" AND module LIKE '%%%s%%'", sanitizeSQL(moduleFilter))
	}
	if levelFilter != "" {
		where += fmt.Sprintf(" AND level = '%s'", sanitizeSQL(levelFilter))
	}

	sql := fmt.Sprintf(
		`SELECT module, level, seq, msg, created FROM frames WHERE %s ORDER BY created ASC LIMIT %d`,
		where, limit,
	)

	type frameRow struct {
		Module  string `db:"module"`
		Level   string `db:"level"`
		Seq     string `db:"seq"`
		Msg     string `db:"msg"`
		Created string `db:"created"`
	}

	var rows []frameRow
	if err := pb.DB().NewQuery(sql).All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}

	buf := new(bytes.Buffer)
	writePCAPGlobalHeader(buf)

	for _, r := range rows {
		ts := parsePCAPTime(r.Created)
		payload, _ := json.Marshal(map[string]any{
			"module":  r.Module,
			"level":   r.Level,
			"seq":     r.Seq,
			"msg":     r.Msg,
			"created": r.Created,
		})
		writePCAPPacket(buf, ts, payload)
	}

	filename := fmt.Sprintf("zaplab_frames_%s.pcap", time.Now().UTC().Format("20060102_150405"))
	e.Response.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
	e.Response.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	e.Response.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	_, err := e.Response.Write(buf.Bytes())
	return err
}

// parsePCAPTime tries several timestamp formats used by PocketBase.
func parsePCAPTime(s string) time.Time {
	for _, layout := range []string{
		"2006-01-02 15:04:05.999Z07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999Z",
		"2006-01-02 15:04:05Z",
		time.RFC3339Nano,
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Now().UTC()
}

// writePCAPGlobalHeader writes the 24-byte libpcap global file header.
func writePCAPGlobalHeader(w *bytes.Buffer) {
	_ = binary.Write(w, binary.LittleEndian, uint32(pcapMagicLE))      // magic
	_ = binary.Write(w, binary.LittleEndian, uint16(2))                // version major
	_ = binary.Write(w, binary.LittleEndian, uint16(4))                // version minor
	_ = binary.Write(w, binary.LittleEndian, int32(0))                 // timezone offset (UTC)
	_ = binary.Write(w, binary.LittleEndian, uint32(0))                // timestamp accuracy
	_ = binary.Write(w, binary.LittleEndian, uint32(65535))            // snaplen
	_ = binary.Write(w, binary.LittleEndian, uint32(pcapLinkEthernet)) // link type
}

// writePCAPPacket writes one PCAP record: 16-byte record header followed by
// a synthetic Ethernet/IPv4/UDP frame carrying payload as UDP data.
func writePCAPPacket(w *bytes.Buffer, ts time.Time, payload []byte) {
	// ── UDP header ────────────────────────────────────────────────────────────
	udpLen := uint16(8 + len(payload))
	udpHdr := make([]byte, 8)
	binary.BigEndian.PutUint16(udpHdr[0:2], 443)    // src port (WhatsApp WebSocket)
	binary.BigEndian.PutUint16(udpHdr[2:4], 12345)  // dst port
	binary.BigEndian.PutUint16(udpHdr[4:6], udpLen) // length
	// checksum = 0 (disabled; Wireshark accepts this)

	// ── IPv4 header ───────────────────────────────────────────────────────────
	ipTotalLen := uint16(20 + int(udpLen))
	ipHdr := make([]byte, 20)
	ipHdr[0] = 0x45 // version=4, IHL=5 (20 bytes, no options)
	binary.BigEndian.PutUint16(ipHdr[2:4], ipTotalLen)
	ipHdr[8] = 64 // TTL
	ipHdr[9] = 17 // protocol = UDP
	copy(ipHdr[12:16], []byte{127, 0, 0, 1}) // src = 127.0.0.1
	copy(ipHdr[16:20], []byte{127, 0, 0, 2}) // dst = 127.0.0.2

	// ── Ethernet header ───────────────────────────────────────────────────────
	ethHdr := make([]byte, 14)
	copy(ethHdr[0:6], []byte{0x52, 0x4c, 0x41, 0x42, 0x00, 0x02})  // dst MAC
	copy(ethHdr[6:12], []byte{0x52, 0x4c, 0x41, 0x42, 0x00, 0x01}) // src MAC
	binary.BigEndian.PutUint16(ethHdr[12:14], 0x0800)               // EtherType = IPv4

	// ── full packet ───────────────────────────────────────────────────────────
	pktLen := len(ethHdr) + len(ipHdr) + len(udpHdr) + len(payload)
	pkt := make([]byte, 0, pktLen)
	pkt = append(pkt, ethHdr...)
	pkt = append(pkt, ipHdr...)
	pkt = append(pkt, udpHdr...)
	pkt = append(pkt, payload...)

	// ── PCAP record header (16 bytes) ─────────────────────────────────────────
	us := ts.UnixMicro()
	_ = binary.Write(w, binary.LittleEndian, uint32(us/1_000_000)) // ts_sec
	_ = binary.Write(w, binary.LittleEndian, uint32(us%1_000_000)) // ts_usec
	_ = binary.Write(w, binary.LittleEndian, uint32(len(pkt)))     // incl_len
	_ = binary.Write(w, binary.LittleEndian, uint32(len(pkt)))     // orig_len
	w.Write(pkt)
}
