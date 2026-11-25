// Package fault holds the runtime fault-injection settings used to
// demonstrate SLO burn-rate alerts on demand.
package fault

import (
	"fmt"
	"math/rand/v2"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Settings is safe for concurrent use.
type Settings struct {
	mu           sync.RWMutex
	errorRate    float64
	extraLatency time.Duration
}

// Snapshot is a point-in-time copy of the settings.
type Snapshot struct {
	ErrorRate      float64 `json:"error_rate"`
	ExtraLatencyMs int64   `json:"latency_ms"`
}

// Get returns the current settings.
func (s *Settings) Get() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Snapshot{ErrorRate: s.errorRate, ExtraLatencyMs: s.extraLatency.Milliseconds()}
}

// Apply updates settings from query parameters error_rate (0..1) and latency_ms (0..10000).
// Parameters that are absent are left unchanged.
func (s *Settings) Apply(q url.Values) (Snapshot, error) {
	var (
		rate    *float64
		latency *time.Duration
	)
	if v := q.Get("error_rate"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < 0 || f > 1 {
			return Snapshot{}, fmt.Errorf("error_rate must be a number between 0 and 1, got %q", v)
		}
		rate = &f
	}
	if v := q.Get("latency_ms"); v != "" {
		ms, err := strconv.Atoi(v)
		if err != nil || ms < 0 || ms > 10000 {
			return Snapshot{}, fmt.Errorf("latency_ms must be an integer between 0 and 10000, got %q", v)
		}
		d := time.Duration(ms) * time.Millisecond
		latency = &d
	}

	s.mu.Lock()
	if rate != nil {
		s.errorRate = *rate
	}
	if latency != nil {
		s.extraLatency = *latency
	}
	s.mu.Unlock()
	return s.Get(), nil
}

// ShouldFail decides whether the current request should fail.
func (s *Settings) ShouldFail() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.errorRate > 0 && rand.Float64() < s.errorRate
}

// ExtraLatency returns the injected latency to add to each request.
func (s *Settings) ExtraLatency() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.extraLatency
}
