// Command app is the demo workload for the observability platform. One binary,
// four modes, so the whole stack builds from a single image:
//
//	-mode=server     checkout-api: RED metrics on /metrics, OTel traces over OTLP
//	-mode=loadgen    steady synthetic traffic with trace context propagation
//	-mode=alertsink  Alertmanager webhook receiver (stands in for PagerDuty/Slack)
//	-mode=probe      container health check (distroless images have no curl)
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/GauravAnand90/observability-platform/app/internal/alertsink"
	"github.com/GauravAnand90/observability-platform/app/internal/fault"
	"github.com/GauravAnand90/observability-platform/app/internal/metrics"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	mode := flag.String("mode", "server", "server | loadgen | alertsink | probe")
	listen := flag.String("listen", ":8080", "listen address (server, alertsink)")
	target := flag.String("target", "http://localhost:8080", "base URL to call (loadgen, probe)")
	rps := flag.Float64("rps", 20, "requests per second (loadgen)")
	service := flag.String("service", "checkout-api", "service name for metrics and traces")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("mode", *mode, "version", version))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch *mode {
	case "server":
		err = runServer(ctx, *listen, *service)
	case "loadgen":
		err = runLoadgen(ctx, *target, *rps)
	case "alertsink":
		err = runAlertSink(ctx, *listen)
	case "probe":
		err = probe(*target)
	default:
		err = fmt.Errorf("unknown mode %q", *mode)
	}
	if err != nil {
		slog.Error("exiting with error", "err", err)
		os.Exit(1)
	}
}

func runServer(ctx context.Context, listen, service string) error {
	shutdownTracing, err := initTracing(ctx, service)
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}
	defer flushTracing(shutdownTracing)

	reg := metrics.NewRegistry(service, version)
	faults := &fault.Settings{}

	mux := http.NewServeMux()
	mux.Handle("POST /api/checkout",
		instrument(reg, "/api/checkout", otelhttp.NewHandler(checkoutHandler(faults), "POST /api/checkout")))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = reg.WriteTo(w)
	})
	mux.HandleFunc("GET /admin/fault", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, faults.Get())
	})
	mux.HandleFunc("POST /admin/fault", func(w http.ResponseWriter, r *http.Request) {
		snap, err := faults.Apply(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		slog.Warn("fault injection updated", "error_rate", snap.ErrorRate, "latency_ms", snap.ExtraLatencyMs)
		writeJSON(w, http.StatusOK, snap)
	})

	slog.Info("checkout-api listening", "addr", listen)
	return serve(ctx, &http.Server{Addr: listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second})
}

func runAlertSink(ctx context.Context, listen string) error {
	mux := http.NewServeMux()
	mux.Handle("/alerts", &alertsink.Sink{Max: 200, Logger: slog.Default()})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	slog.Info("alert sink listening", "addr", listen)
	return serve(ctx, &http.Server{Addr: listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second})
}

func probe(target string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(target)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("probe %s: status %d", target, resp.StatusCode)
	}
	return nil
}

// serve runs srv until ctx is cancelled, then drains in-flight requests.
func serve(ctx context.Context, srv *http.Server) error {
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()

	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// instrument records RED metrics for every request that reaches h.
func instrument(reg *metrics.Registry, route string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		h.ServeHTTP(rec, r)
		reg.Observe(route, r.Method, rec.status, time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
