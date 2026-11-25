// Package metrics is a small, dependency-free Prometheus registry for the
// RED signals (rate, errors, duration) the SLOs are built on.
//
// Metric names here are a contract: the SLO recording rules in
// prometheus/rules/ select on them. Change them only together with the rules.
package metrics

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultBuckets include 0.3s exactly, because the latency SLO threshold is 300ms
// and an SLI can only be computed precisely at a bucket boundary.
var DefaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.3, 0.5, 1, 2.5, 5}

type requestKey struct {
	route  string
	method string
	code   string
}

type histogram struct {
	counts []uint64 // non-cumulative per bucket; +Inf is the last slot
	sum    float64
	count  uint64
}

// Registry holds request counters and latency histograms for one service.
type Registry struct {
	mu       sync.Mutex
	service  string
	version  string
	buckets  []float64
	requests map[requestKey]uint64
	latency  map[string]*histogram // keyed by route
	started  time.Time
}

// NewRegistry returns a registry for the given service and build version.
func NewRegistry(service, version string) *Registry {
	return &Registry{
		service:  service,
		version:  version,
		buckets:  DefaultBuckets,
		requests: map[requestKey]uint64{},
		latency:  map[string]*histogram{},
		started:  time.Now(),
	}
}

// Observe records one finished request.
func (r *Registry) Observe(route, method string, code int, d time.Duration) {
	seconds := d.Seconds()

	r.mu.Lock()
	defer r.mu.Unlock()

	r.requests[requestKey{route: route, method: method, code: strconv.Itoa(code)}]++

	h, ok := r.latency[route]
	if !ok {
		h = &histogram{counts: make([]uint64, len(r.buckets)+1)}
		r.latency[route] = h
	}
	idx := len(r.buckets) // +Inf
	for i, upper := range r.buckets {
		if seconds <= upper {
			idx = i
			break
		}
	}
	h.counts[idx]++
	h.sum += seconds
	h.count++
}

// WriteTo renders the registry in the Prometheus text exposition format (0.0.4).
// Output is sorted so it is deterministic and diff-friendly.
func (r *Registry) WriteTo(w io.Writer) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var b strings.Builder
	svc := escape(r.service)

	b.WriteString("# HELP app_build_info Build information.\n# TYPE app_build_info gauge\n")
	fmt.Fprintf(&b, "app_build_info{service=\"%s\",version=\"%s\"} 1\n", svc, escape(r.version))

	b.WriteString("# HELP process_start_time_seconds Start time of the process since unix epoch in seconds.\n# TYPE process_start_time_seconds gauge\n")
	fmt.Fprintf(&b, "process_start_time_seconds %d\n", r.started.Unix())

	b.WriteString("# HELP http_requests_total Total HTTP requests by route, method and status code.\n# TYPE http_requests_total counter\n")
	keys := make([]requestKey, 0, len(r.requests))
	for k := range r.requests {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].route != keys[j].route {
			return keys[i].route < keys[j].route
		}
		if keys[i].method != keys[j].method {
			return keys[i].method < keys[j].method
		}
		return keys[i].code < keys[j].code
	})
	for _, k := range keys {
		fmt.Fprintf(&b, "http_requests_total{service=\"%s\",route=\"%s\",method=\"%s\",code=\"%s\"} %d\n",
			svc, escape(k.route), escape(k.method), k.code, r.requests[k])
	}

	b.WriteString("# HELP http_request_duration_seconds HTTP request latency by route.\n# TYPE http_request_duration_seconds histogram\n")
	routes := make([]string, 0, len(r.latency))
	for route := range r.latency {
		routes = append(routes, route)
	}
	sort.Strings(routes)
	for _, route := range routes {
		h := r.latency[route]
		labels := fmt.Sprintf("service=\"%s\",route=\"%s\"", svc, escape(route))
		var cumulative uint64
		for i, upper := range r.buckets {
			cumulative += h.counts[i]
			fmt.Fprintf(&b, "http_request_duration_seconds_bucket{%s,le=\"%s\"} %d\n", labels, formatFloat(upper), cumulative)
		}
		cumulative += h.counts[len(r.buckets)]
		fmt.Fprintf(&b, "http_request_duration_seconds_bucket{%s,le=\"+Inf\"} %d\n", labels, cumulative)
		fmt.Fprintf(&b, "http_request_duration_seconds_sum{%s} %s\n", labels, formatFloat(h.sum))
		fmt.Fprintf(&b, "http_request_duration_seconds_count{%s} %d\n", labels, h.count)
	}

	n, err := io.WriteString(w, b.String())
	return int64(n), err
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// escape applies the label-value escaping rules of the text format.
func escape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return strings.ReplaceAll(s, `"`, `\"`)
}
