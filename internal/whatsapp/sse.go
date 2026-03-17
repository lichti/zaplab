package whatsapp

import (
	"encoding/json"
	"sync"
)

// SSEEvent is a single server-sent event payload.
type SSEEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// sseBroker multiplexes WhatsApp events to all active SSE subscribers.
type sseBroker struct {
	mu          sync.RWMutex
	subscribers map[chan SSEEvent]struct{}
}

var broker = &sseBroker{
	subscribers: make(map[chan SSEEvent]struct{}),
}

// SSESubscribe registers a new subscriber and returns a buffered channel.
func SSESubscribe() chan SSEEvent {
	ch := make(chan SSEEvent, 64)
	broker.mu.Lock()
	broker.subscribers[ch] = struct{}{}
	broker.mu.Unlock()
	return ch
}

// SSEUnsubscribe removes a subscriber and closes its channel.
func SSEUnsubscribe(ch chan SSEEvent) {
	broker.mu.Lock()
	delete(broker.subscribers, ch)
	broker.mu.Unlock()
	close(ch)
}

// ssePublish sends an event to all current subscribers (non-blocking).
func ssePublish(evtType string, raw interface{}) {
	data, err := json.Marshal(raw)
	if err != nil {
		return
	}
	evt := SSEEvent{Type: evtType, Data: json.RawMessage(data)}
	broker.mu.RLock()
	defer broker.mu.RUnlock()
	for ch := range broker.subscribers {
		select {
		case ch <- evt:
		default: // skip slow consumers
		}
	}
}
