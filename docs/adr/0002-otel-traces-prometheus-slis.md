# ADR 0002: OpenTelemetry for traces, Prometheus-native metrics for SLIs

**Status:** Accepted

## Context

OpenTelemetry can carry metrics as well as traces, and we could send everything over OTLP. But the SLO rules select on exact metric names and labels. When OTLP metrics are translated to Prometheus, names and labels depend on translation settings and versions (unit suffixes, `_total`, dots versus underscores). A silent rename would make every SLO alert stop firing.

## Decision

- **SLIs come from Prometheus-native metrics** (`http_requests_total`, `http_request_duration_seconds`) that the service exposes itself. Their names are a contract, unit-tested in Go and in `promtool`, and guarded by the `SLOSignalMissing` alert.
- **Traces use the OpenTelemetry SDK** and go over OTLP to an **OpenTelemetry Collector**, never straight to a backend. The app only knows the standard `OTEL_*` environment variables.
- The **Collector** owns telemetry policy:
  - **Tail sampling** keeps 100% of error traces, 100% of traces over 300ms (the latency SLO threshold), and 10% of the rest.
  - The **span_metrics** and **service_graph** connectors derive RED metrics and the service map from all spans *before* sampling, so they stay accurate.
- **Tempo** stores traces. Grafana links metrics to traces (TraceQL panels on the RED dashboard, traces-to-metrics on the Tempo datasource).

## Consequences

- An SLO alert answers "are users hurting?" and the kept error and slow traces answer "why?". That shortens time to mitigate.
- The backend can be swapped (Tempo, Jaeger, a vendor) by changing Collector config only. No app redeploy.
- Two metric paths exist (native and span-derived). Only the native one is used for SLOs; span metrics are for services that are traced but not yet instrumented.
- Tail sampling needs all spans of a trace on one Collector. Scaling out means adding a load-balancing exporter layer keyed by trace ID.
