package metrics

import (
	"strings"
	"testing"
	"time"
)

func render(t *testing.T, r *Registry) string {
	t.Helper()
	var b strings.Builder
	if _, err := r.WriteTo(&b); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return b.String()
}

func TestCountersByRouteMethodCode(t *testing.T) {
	r := NewRegistry("checkout-api", "test")
	r.Observe("/api/checkout", "POST", 200, 10*time.Millisecond)
	r.Observe("/api/checkout", "POST", 200, 10*time.Millisecond)
	r.Observe("/api/checkout", "POST", 500, 10*time.Millisecond)

	out := render(t, r)
	for _, want := range []string{
		`http_requests_total{service="checkout-api",route="/api/checkout",method="POST",code="200"} 2`,
		`http_requests_total{service="checkout-api",route="/api/checkout",method="POST",code="500"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing line %q in:\n%s", want, out)
		}
	}
}

func TestHistogramIsCumulativeAndHas300msBucket(t *testing.T) {
	r := NewRegistry("checkout-api", "test")
	r.Observe("/api/checkout", "POST", 200, 40*time.Millisecond)  // le 0.05
	r.Observe("/api/checkout", "POST", 200, 300*time.Millisecond) // le 0.3 (boundary is inclusive)
	r.Observe("/api/checkout", "POST", 200, 450*time.Millisecond) // le 0.5
	r.Observe("/api/checkout", "POST", 200, 9*time.Second)        // +Inf only

	out := render(t, r)
	for _, want := range []string{
		`http_request_duration_seconds_bucket{service="checkout-api",route="/api/checkout",le="0.025"} 0`,
		`http_request_duration_seconds_bucket{service="checkout-api",route="/api/checkout",le="0.05"} 1`,
		`http_request_duration_seconds_bucket{service="checkout-api",route="/api/checkout",le="0.3"} 2`,
		`http_request_duration_seconds_bucket{service="checkout-api",route="/api/checkout",le="0.5"} 3`,
		`http_request_duration_seconds_bucket{service="checkout-api",route="/api/checkout",le="5"} 3`,
		`http_request_duration_seconds_bucket{service="checkout-api",route="/api/checkout",le="+Inf"} 4`,
		`http_request_duration_seconds_count{service="checkout-api",route="/api/checkout"} 4`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing line %q in:\n%s", want, out)
		}
	}
}

func TestOutputHasTypeLinesAndIsDeterministic(t *testing.T) {
	r := NewRegistry("checkout-api", "test")
	r.Observe("/b", "GET", 200, time.Millisecond)
	r.Observe("/a", "GET", 200, time.Millisecond)

	first, second := render(t, r), render(t, r)
	if first != second {
		t.Fatal("exposition output is not deterministic")
	}
	for _, want := range []string{
		"# TYPE http_requests_total counter",
		"# TYPE http_request_duration_seconds histogram",
		"# TYPE app_build_info gauge",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Index(first, `route="/a"`) > strings.Index(first, `route="/b"`) {
		t.Error("series are not sorted by route")
	}
}

func TestLabelEscaping(t *testing.T) {
	if got := escape(`a"b\c` + "\n"); got != `a\"b\\c\n` {
		t.Errorf("escape = %q", got)
	}
}
