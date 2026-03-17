package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lichti/zaplab/internal/whatsapp"
	"github.com/pocketbase/pocketbase/core"
)

// getSSEStream streams WhatsApp events over Server-Sent Events.
// Auth is required (same as other protected routes).
// Optional query param: type=EventType (filter to one event type).
func getSSEStream(e *core.RequestEvent) error {
	flusher, ok := e.Response.(http.Flusher)
	if !ok {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": "streaming not supported"})
	}

	typeFilter := e.Request.URL.Query().Get("type")

	e.Response.Header().Set("Content-Type", "text/event-stream")
	e.Response.Header().Set("Cache-Control", "no-cache")
	e.Response.Header().Set("Connection", "keep-alive")
	e.Response.Header().Set("X-Accel-Buffering", "no")
	e.Response.WriteHeader(http.StatusOK)

	// Send initial "connected" heartbeat
	fmt.Fprintf(e.Response, "event: connected\ndata: {\"ts\":%d}\n\n", time.Now().Unix())
	flusher.Flush()

	ch := whatsapp.SSESubscribe()
	defer whatsapp.SSEUnsubscribe(ch)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ctx := e.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			fmt.Fprintf(e.Response, ": heartbeat\n\n")
			flusher.Flush()
		case evt, ok := <-ch:
			if !ok {
				return nil
			}
			if typeFilter != "" && evt.Type != typeFilter {
				continue
			}
			payload, _ := json.Marshal(evt)
			fmt.Fprintf(e.Response, "event: message\ndata: %s\n\n", string(payload))
			flusher.Flush()
		}
	}
}
