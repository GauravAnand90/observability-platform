// Package alertsink is a tiny Alertmanager webhook receiver. It stands in for
// PagerDuty or Slack in the local stack: it logs every notification and keeps
// the most recent ones so `make alerts` can show what would have paged.
package alertsink

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Alert is the subset of Alertmanager's webhook alert payload we use.
type Alert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
}

// Payload is Alertmanager's webhook body (version 4).
type Payload struct {
	Receiver string  `json:"receiver"`
	Status   string  `json:"status"`
	Alerts   []Alert `json:"alerts"`
}

// Notification is one received alert, as stored and returned by GET /alerts.
type Notification struct {
	ReceivedAt time.Time `json:"received_at"`
	Receiver   string    `json:"receiver"`
	Status     string    `json:"status"`
	AlertName  string    `json:"alertname"`
	Severity   string    `json:"severity"`
	SLO        string    `json:"slo,omitempty"`
	Summary    string    `json:"summary"`
}

// Sink receives webhooks and remembers the last Max notifications.
type Sink struct {
	Max    int
	Logger *slog.Logger

	mu   sync.Mutex
	last []Notification
}

// ServeHTTP handles POST (receive) and GET (list) on /alerts.
func (s *Sink) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var p Payload
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&p); err != nil {
			http.Error(w, fmt.Sprintf("invalid webhook payload: %v", err), http.StatusBadRequest)
			return
		}
		s.record(p)
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.Recent())
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Sink) record(p Payload) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range p.Alerts {
		n := Notification{
			ReceivedAt: now,
			Receiver:   p.Receiver,
			Status:     a.Status,
			AlertName:  a.Labels["alertname"],
			Severity:   a.Labels["severity"],
			SLO:        a.Labels["slo"],
			Summary:    a.Annotations["summary"],
		}
		if s.Logger != nil {
			s.Logger.Info("alert notification",
				"receiver", n.Receiver, "status", n.Status, "alertname", n.AlertName,
				"severity", n.Severity, "slo", n.SLO, "summary", n.Summary)
		}
		s.last = append(s.last, n)
	}
	max := s.Max
	if max <= 0 {
		max = 100
	}
	if len(s.last) > max {
		s.last = append([]Notification(nil), s.last[len(s.last)-max:]...)
	}
}

// Recent returns stored notifications, newest last.
func (s *Sink) Recent() []Notification {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Notification{}, s.last...)
}
