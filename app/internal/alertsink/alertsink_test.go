package alertsink

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const samplePayload = `{
  "version": "4",
  "receiver": "pager",
  "status": "firing",
  "alerts": [{
    "status": "firing",
    "labels": {"alertname": "SLOErrorBudgetBurnFast", "severity": "page", "slo": "checkout-availability", "service": "checkout-api"},
    "annotations": {"summary": "checkout-availability is burning error budget fast"},
    "startsAt": "2026-11-24T10:00:00Z"
  }]
}`

func TestReceiveAndList(t *testing.T) {
	s := &Sink{Max: 10}

	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(samplePayload)))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/alerts", nil))
	var got []Notification
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d notifications, want 1", len(got))
	}
	n := got[0]
	if n.Receiver != "pager" || n.AlertName != "SLOErrorBudgetBurnFast" || n.Severity != "page" || n.SLO != "checkout-availability" {
		t.Fatalf("unexpected notification %+v", n)
	}
}

func TestRejectsBadPayload(t *testing.T) {
	s := &Sink{}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader("{not json")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestKeepsOnlyMostRecent(t *testing.T) {
	s := &Sink{Max: 3}
	for i := 0; i < 5; i++ {
		s.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(samplePayload)))
	}
	if n := len(s.Recent()); n != 3 {
		t.Fatalf("kept %d, want 3", n)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	s := &Sink{}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/alerts", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
