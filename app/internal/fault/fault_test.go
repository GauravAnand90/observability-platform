package fault

import (
	"net/url"
	"testing"
	"time"
)

func TestApplyValidValues(t *testing.T) {
	var s Settings
	got, err := s.Apply(url.Values{"error_rate": {"0.25"}, "latency_ms": {"400"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.ErrorRate != 0.25 || got.ExtraLatencyMs != 400 {
		t.Fatalf("got %+v", got)
	}
	if s.ExtraLatency() != 400*time.Millisecond {
		t.Fatalf("ExtraLatency = %v", s.ExtraLatency())
	}
}

func TestApplyLeavesMissingParamsUnchanged(t *testing.T) {
	var s Settings
	if _, err := s.Apply(url.Values{"error_rate": {"0.5"}, "latency_ms": {"100"}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Apply(url.Values{"error_rate": {"0"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.ErrorRate != 0 || got.ExtraLatencyMs != 100 {
		t.Fatalf("got %+v", got)
	}
}

func TestApplyRejectsOutOfRange(t *testing.T) {
	cases := []url.Values{
		{"error_rate": {"1.5"}},
		{"error_rate": {"-0.1"}},
		{"error_rate": {"abc"}},
		{"latency_ms": {"-1"}},
		{"latency_ms": {"20000"}},
		{"latency_ms": {"1.5"}},
	}
	for _, q := range cases {
		var s Settings
		if _, err := s.Apply(q); err == nil {
			t.Errorf("Apply(%v) succeeded, want error", q)
		}
	}
}

func TestShouldFailBounds(t *testing.T) {
	var s Settings
	for i := 0; i < 1000; i++ {
		if s.ShouldFail() {
			t.Fatal("ShouldFail returned true with error_rate 0")
		}
	}
	if _, err := s.Apply(url.Values{"error_rate": {"1"}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		if !s.ShouldFail() {
			t.Fatal("ShouldFail returned false with error_rate 1")
		}
	}
}
