package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// runLoadgen sends steady traffic so the SLIs always have data. Its HTTP
// client injects W3C traceparent headers, so each trace in Tempo starts at
// the load generator and continues into checkout-api.
func runLoadgen(ctx context.Context, target string, rps float64) error {
	if rps <= 0 {
		return fmt.Errorf("rps must be > 0, got %v", rps)
	}

	shutdownTracing, err := initTracing(ctx, "loadgen")
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}
	defer flushTracing(shutdownTracing)

	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	url := target + "/api/checkout"

	var ok, failed atomic.Int64
	send := time.NewTicker(time.Duration(float64(time.Second) / rps))
	report := time.NewTicker(30 * time.Second)
	defer send.Stop()
	defer report.Stop()

	slog.Info("load generator started", "target", url, "rps", rps)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-report.C:
			slog.Info("load generator stats", "ok", ok.Swap(0), "failed", failed.Swap(0))
		case <-send.C:
			go func() {
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
				if err != nil {
					failed.Add(1)
					return
				}
				resp, err := client.Do(req)
				if err != nil {
					failed.Add(1)
					return
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode >= 500 {
					failed.Add(1)
					return
				}
				ok.Add(1)
			}()
		}
	}
}
