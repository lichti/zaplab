package whatsapp

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pocketbase/pocketbase/core"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// ── Ring buffer ───────────────────────────────────────────────────────────────

const ringBufSize = 2000

// LogEntry is a single captured log line.
type LogEntry struct {
	Seq     uint64    `json:"seq"`
	Time    time.Time `json:"time"`
	Module  string    `json:"module"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type ringBuf struct {
	mu      sync.Mutex
	entries [ringBufSize]LogEntry
	head    int    // next write position
	count   int    // number of valid entries (0..ringBufSize)
	total   uint64 // monotonic counter
}

var ring = &ringBuf{}

// push adds an entry to the ring buffer, overwriting the oldest entry when full.
func (r *ringBuf) push(e LogEntry) {
	r.mu.Lock()
	r.entries[r.head] = e
	r.head = (r.head + 1) % ringBufSize
	if r.count < ringBufSize {
		r.count++
	}
	r.mu.Unlock()
}

// snapshot returns all entries in chronological order (oldest first).
// module and level filters are applied if non-empty.
func (r *ringBuf) snapshot(module, level string, limit int) []LogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Calculate start position
	start := (r.head - r.count + ringBufSize) % ringBufSize
	out := make([]LogEntry, 0, r.count)
	for i := range r.count {
		idx := (start + i) % ringBufSize
		e := r.entries[idx]
		if module != "" && e.Module != module {
			continue
		}
		if level != "" && e.Level != level {
			continue
		}
		out = append(out, e)
	}

	// Return last N entries
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// GetLogEntries returns a snapshot of the in-memory ring buffer.
// module and level may be empty to return all entries.
func GetLogEntries(module, level string, limit int) []LogEntry {
	return ring.snapshot(module, level, limit)
}

// ── Capturing logger ─────────────────────────────────────────────────────────

var captureSeq atomic.Uint64

// captureSink is a non-blocking channel for persisting INFO+ entries to PocketBase.
var captureSink = make(chan LogEntry, 4096)

type capturingLogger struct {
	inner  waLog.Logger
	module string
}

func (l *capturingLogger) Debugf(msg string, args ...interface{}) {
	l.inner.Debugf(msg, args...)
	l.emit("DEBUG", msg, args...)
}
func (l *capturingLogger) Infof(msg string, args ...interface{}) {
	l.inner.Infof(msg, args...)
	l.emit("INFO", msg, args...)
}
func (l *capturingLogger) Warnf(msg string, args ...interface{}) {
	l.inner.Warnf(msg, args...)
	l.emit("WARN", msg, args...)
}
func (l *capturingLogger) Errorf(msg string, args ...interface{}) {
	l.inner.Errorf(msg, args...)
	l.emit("ERROR", msg, args...)
}
func (l *capturingLogger) Sub(module string) waLog.Logger {
	sub := fmt.Sprintf("%s/%s", l.module, module)
	return &capturingLogger{
		inner:  l.inner.Sub(module),
		module: sub,
	}
}

func (l *capturingLogger) emit(level, msg string, args ...interface{}) {
	entry := LogEntry{
		Seq:     captureSeq.Add(1),
		Time:    time.Now(),
		Module:  l.module,
		Level:   level,
		Message: fmt.Sprintf(msg, args...),
	}
	// Always push to in-memory ring (non-blocking, no alloc pressure)
	ring.push(entry)

	// Persist INFO+ to PocketBase (non-blocking — drop if full)
	if level == "INFO" || level == "WARN" || level == "ERROR" {
		select {
		case captureSink <- entry:
		default: // drop: never block the WhatsApp connection
		}
	}
}

// NewCapturingLogger wraps inner so that all log calls are mirrored into the
// ring buffer and (for INFO+) persisted to PocketBase via captureSink.
func NewCapturingLogger(inner waLog.Logger, module string) waLog.Logger {
	return &capturingLogger{inner: inner, module: module}
}

// StartLogConsumer drains captureSink and writes records to the PocketBase
// `frames` collection. Must be called after pb is set (i.e. inside Bootstrap
// or later). Runs in a background goroutine for the lifetime of the process.
func StartLogConsumer() {
	go func() {
		for entry := range captureSink {
			if pb == nil {
				continue
			}
			persistLogEntry(entry)
		}
	}()
}

func persistLogEntry(entry LogEntry) {
	col, err := pb.FindCollectionByNameOrId("frames")
	if err != nil {
		return
	}
	record := core.NewRecord(col)
	record.Set("module", entry.Module)
	record.Set("level", entry.Level)
	record.Set("seq", fmt.Sprintf("%d", entry.Seq))
	record.Set("msg", entry.Message)
	// Ignore save errors — frames are best-effort
	_ = pb.Save(record)
}
