package simulation

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types"

	"github.com/lichti/zaplab/internal/whatsapp"
)

// SimRequest holds the parameters for starting a route simulation.
type SimRequest struct {
	To              string
	GPXBase64       string
	SpeedKmh        float64
	IntervalSeconds float64
	Caption         string
}

// ActiveSim describes a running simulation.
type ActiveSim struct {
	ID               string  `json:"id"`
	To               string  `json:"to"`
	TotalDistanceKm  float64 `json:"total_distance_km"`
	EstimatedMinutes float64 `json:"estimated_minutes"`
	Waypoints        int     `json:"waypoints"`
	SpeedKmh         float64 `json:"speed_kmh"`
	IntervalSeconds  float64 `json:"interval_seconds"`
	cancel           context.CancelFunc
}

var (
	mu         sync.Mutex
	activeSims = map[string]*ActiveSim{}
)

// Start parses the GPX, builds the route, and launches a background goroutine
// that sends live location updates until the route is complete or Stop is called.
func Start(toJID types.JID, req SimRequest) (*ActiveSim, error) {
	if req.SpeedKmh <= 0 {
		req.SpeedKmh = 60
	}
	if req.IntervalSeconds <= 0 {
		req.IntervalSeconds = 5
	}

	points, err := ParseGPXBase64(req.GPXBase64)
	if err != nil {
		return nil, fmt.Errorf("GPX parse error: %w", err)
	}

	route := NewRoute(points)
	if route.Total == 0 {
		return nil, fmt.Errorf("route has zero length")
	}

	ctx, cancel := context.WithCancel(context.Background())

	sim := &ActiveSim{
		ID:               randomID(),
		To:               req.To,
		TotalDistanceKm:  route.Total,
		EstimatedMinutes: (route.Total / req.SpeedKmh) * 60,
		Waypoints:        len(points),
		SpeedKmh:         req.SpeedKmh,
		IntervalSeconds:  req.IntervalSeconds,
		cancel:           cancel,
	}

	mu.Lock()
	activeSims[sim.ID] = sim
	mu.Unlock()

	go func() {
		defer func() {
			mu.Lock()
			delete(activeSims, sim.ID)
			mu.Unlock()
		}()

		interval := time.Duration(req.IntervalSeconds * float64(time.Second))
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		kmPerSec := req.SpeedKmh / 3600.0
		startTime := time.Now()
		var seq int64 = 1

		for {
			elapsed := time.Since(startTime).Seconds()
			distKm := elapsed * kmPerSec
			pt := route.PointAt(distKm, req.SpeedKmh)

			whatsapp.SendLiveLocation(
				toJID,
				pt.Lat,
				pt.Lon,
				10,
				pt.SpeedMps,
				pt.Bearing,
				req.Caption,
				seq,
				uint32(elapsed),
				nil,
			)
			seq++

			if distKm >= route.Total {
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	return sim, nil
}

// Stop cancels a running simulation by ID. Returns false if not found.
func Stop(id string) bool {
	mu.Lock()
	defer mu.Unlock()
	sim, ok := activeSims[id]
	if !ok {
		return false
	}
	sim.cancel()
	return true
}

// List returns a snapshot of all currently active simulations.
func List() []*ActiveSim {
	mu.Lock()
	defer mu.Unlock()
	out := make([]*ActiveSim, 0, len(activeSims))
	for _, s := range activeSims {
		out = append(out, s)
	}
	return out
}

func randomID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
