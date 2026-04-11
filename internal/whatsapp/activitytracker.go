package whatsapp

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

const (
	atProbeInterval   = 2 * time.Second
	atProbeJitter     = 500 * time.Millisecond
	atProbeTimeout    = 10 * time.Second
	atMovingAvgWindow = 3
	atGlobalSamples   = 2000
	atThresholdRatio  = 0.90
	atMinSamples      = 5 // minimum samples before classifying as Standby
)

// ATState represents the inferred device state.
type ATState string

const (
	ATOnline  ATState = "Online"
	ATStandby ATState = "Standby"
	ATOffline ATState = "Offline"
)

// probeWaiter holds a channel that receives the RTT when the delivery receipt arrives.
type probeWaiter struct {
	ch     chan int64
	sentAt time.Time
}

// atTracker holds per-JID state.
type atTracker struct {
	JID          types.JID
	SessionID    string
	ProbeMethod  string
	cancel       context.CancelFunc
	mu           sync.Mutex
	recentRTTs   []float64 // last atMovingAvgWindow samples (moving average)
	globalRTTs   []float64 // last atGlobalSamples (for median baseline)
	CurrentState ATState
}

var (
	atMu       sync.RWMutex
	atTrackers = map[string]*atTracker{} // key: jid.String()

	atProbeMu      sync.Mutex
	atProbeWaiters = map[string]*probeWaiter{} // key: messageID
)

// NotifyProbeReceipt is called from the receipt event handler to signal RTT to a waiting probe.
func NotifyProbeReceipt(messageID string) {
	atProbeMu.Lock()
	w, ok := atProbeWaiters[messageID]
	if ok {
		delete(atProbeWaiters, messageID)
	}
	atProbeMu.Unlock()
	if !ok {
		return
	}
	rtt := time.Since(w.sentAt).Milliseconds()
	select {
	case w.ch <- rtt:
	default:
	}
}

// StartTracker begins tracking the device activity of a JID.
// probeMethod must be "delete" or "reaction".
func StartTracker(jid types.JID, probeMethod string) (string, error) {
	if probeMethod != "delete" && probeMethod != "reaction" {
		probeMethod = "delete"
	}
	key := jid.String()
	atMu.Lock()
	defer atMu.Unlock()
	if _, exists := atTrackers[key]; exists {
		return "", fmt.Errorf("already tracking %s", key)
	}
	sessionID, err := createActivitySession(jid, probeMethod)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t := &atTracker{
		JID:          jid,
		SessionID:    sessionID,
		ProbeMethod:  probeMethod,
		cancel:       cancel,
		CurrentState: ATOffline,
	}
	atTrackers[key] = t
	go t.run(ctx)
	return sessionID, nil
}

// StopTracker stops tracking a JID.
func StopTracker(jid types.JID) error {
	key := jid.String()
	atMu.Lock()
	t, exists := atTrackers[key]
	if exists {
		delete(atTrackers, key)
	}
	atMu.Unlock()
	if !exists {
		return fmt.Errorf("not tracking %s", key)
	}
	t.cancel()
	go closeActivitySession(t.SessionID)
	return nil
}

// StopAllTrackers stops every active tracker (called when the feature is disabled).
func StopAllTrackers() {
	atMu.Lock()
	trackers := make([]*atTracker, 0, len(atTrackers))
	for _, t := range atTrackers {
		trackers = append(trackers, t)
	}
	atTrackers = map[string]*atTracker{}
	atMu.Unlock()
	for _, t := range trackers {
		t.cancel()
		go closeActivitySession(t.SessionID)
	}
}

// StartTrackersBulk starts trackers for a list of JIDs, skipping already-tracked ones.
// Returns counts of started, skipped, and failed trackers.
func StartTrackersBulk(jids []types.JID, probeMethod string) (started, skipped, failed int) {
	for _, jid := range jids {
		_, err := StartTracker(jid, probeMethod)
		switch {
		case err == nil:
			started++
		case isAlreadyTracking(err):
			skipped++
		default:
			logger.Warnf("ActivityTracker: bulk start failed jid=%s err=%v", jid, err)
			failed++
		}
	}
	return
}

func isAlreadyTracking(err error) bool {
	return err != nil && len(err.Error()) > 16 && err.Error()[:16] == "already tracking"
}

// GetTrackerStatus returns a snapshot of all active trackers.
func GetTrackerStatus() []map[string]any {
	atMu.RLock()
	defer atMu.RUnlock()
	out := make([]map[string]any, 0, len(atTrackers))
	for _, t := range atTrackers {
		t.mu.Lock()
		state := t.CurrentState
		t.mu.Unlock()
		out = append(out, map[string]any{
			"jid":          t.JID.String(),
			"session_id":   t.SessionID,
			"probe_method": t.ProbeMethod,
			"state":        state,
		})
	}
	return out
}

// ─── Tracker loop ─────────────────────────────────────────────────────────────

func (t *atTracker) run(ctx context.Context) {
	for {
		jitter := time.Duration(rand.Int63n(int64(atProbeJitter)))
		select {
		case <-ctx.Done():
			return
		case <-time.After(atProbeInterval + jitter):
		}
		t.probe(ctx)
	}
}

func (t *atTracker) probe(ctx context.Context) {
	probeID := client.GenerateMessageID()
	ch := make(chan int64, 1)
	w := &probeWaiter{ch: ch, sentAt: time.Now()}

	atProbeMu.Lock()
	atProbeWaiters[probeID] = w
	atProbeMu.Unlock()

	if err := t.sendProbe(probeID); err != nil {
		atProbeMu.Lock()
		delete(atProbeWaiters, probeID)
		atProbeMu.Unlock()
		logger.Warnf("ActivityTracker: probe send failed jid=%s err=%v", t.JID, err)
		return
	}

	var rttMs int64
	var state ATState
	select {
	case rttMs = <-ch:
		state = t.classify(float64(rttMs))
	case <-time.After(atProbeTimeout):
		atProbeMu.Lock()
		delete(atProbeWaiters, probeID)
		atProbeMu.Unlock()
		state = ATOffline
		rttMs = -1
	case <-ctx.Done():
		atProbeMu.Lock()
		delete(atProbeWaiters, probeID)
		atProbeMu.Unlock()
		return
	}

	t.mu.Lock()
	t.CurrentState = state
	median := atMedian(t.globalRTTs)
	threshold := median * atThresholdRatio
	t.mu.Unlock()

	go persistProbe(t.SessionID, t.JID.String(), rttMs, string(state), median, threshold)
}

func (t *atTracker) classify(rttMs float64) ATState {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.globalRTTs) >= atGlobalSamples {
		t.globalRTTs = t.globalRTTs[1:]
	}
	t.globalRTTs = append(t.globalRTTs, rttMs)

	if len(t.recentRTTs) >= atMovingAvgWindow {
		t.recentRTTs = t.recentRTTs[1:]
	}
	t.recentRTTs = append(t.recentRTTs, rttMs)

	if len(t.globalRTTs) < atMinSamples {
		return ATOnline // not enough baseline yet
	}

	median := atMedian(t.globalRTTs)
	threshold := median * atThresholdRatio
	avg := atMovingAvg(t.recentRTTs)

	if avg < threshold {
		return ATOnline
	}
	return ATStandby
}

func atMedian(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sorted := make([]float64, len(samples))
	copy(sorted, samples)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func atMovingAvg(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range samples {
		sum += v
	}
	return sum / float64(len(samples))
}

// ─── Probe senders ────────────────────────────────────────────────────────────

func (t *atTracker) sendProbe(probeID string) error {
	if t.ProbeMethod == "reaction" {
		return sendReactionProbe(t.JID, probeID)
	}
	return sendDeleteProbe(t.JID, probeID)
}

func sendDeleteProbe(jid types.JID, probeID string) error {
	// Send a revoke for a non-existent message. The delivery receipt on
	// the probe envelope (probeID) reveals the device RTT.
	fakeTargetID := client.GenerateMessageID()
	msg := &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Key: &waCommon.MessageKey{
				RemoteJID: proto.String(jid.String()),
				FromMe:    proto.Bool(true),
				ID:        proto.String(fakeTargetID),
			},
			Type: waE2E.ProtocolMessage_REVOKE.Enum(),
		},
	}
	_, err := client.SendMessage(context.Background(), jid, msg,
		whatsmeow.SendRequestExtra{ID: types.MessageID(probeID)})
	return err
}

func sendReactionProbe(jid types.JID, probeID string) error {
	fakeTargetID := client.GenerateMessageID()
	msg := &waE2E.Message{
		ReactionMessage: &waE2E.ReactionMessage{
			Key: &waCommon.MessageKey{
				RemoteJID: proto.String(jid.String()),
				FromMe:    proto.Bool(true),
				ID:        proto.String(fakeTargetID),
			},
			Text:              proto.String("👍"),
			SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
		},
	}
	_, err := client.SendMessage(context.Background(), jid, msg,
		whatsmeow.SendRequestExtra{ID: types.MessageID(probeID)})
	return err
}

// ─── PocketBase persistence ───────────────────────────────────────────────────

func createActivitySession(jid types.JID, probeMethod string) (string, error) {
	col, err := pb.FindCollectionByNameOrId("device_activity_sessions")
	if err != nil {
		return "", err
	}
	record := core.NewRecord(col)
	record.Set("jid", jid.String())
	record.Set("probe_method", probeMethod)
	record.Set("started_at", time.Now().UTC().Format(time.RFC3339))
	if err := pb.Save(record); err != nil {
		return "", err
	}
	return record.Id, nil
}

func closeActivitySession(sessionID string) {
	record, err := pb.FindRecordById("device_activity_sessions", sessionID)
	if err != nil {
		return
	}
	record.Set("stopped_at", time.Now().UTC().Format(time.RFC3339))
	_ = pb.Save(record)
}

func persistProbe(sessionID, jid string, rttMs int64, state string, medianMs, thresholdMs float64) {
	col, err := pb.FindCollectionByNameOrId("device_activity_probes")
	if err != nil {
		return
	}
	record := core.NewRecord(col)
	record.Set("session_id", sessionID)
	record.Set("jid", jid)
	record.Set("rtt_ms", rttMs)
	record.Set("state", state)
	record.Set("median_ms", medianMs)
	record.Set("threshold_ms", thresholdMs)
	_ = pb.Save(record)
}
