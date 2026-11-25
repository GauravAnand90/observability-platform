# Runbook: CheckoutTargetDown / SLOSignalMissing

**Severity:** page (`CheckoutTargetDown`) or ticket (`SLOSignalMissing`)

## Why these matter

SLO alerts are computed from the service's metrics. If Prometheus can't scrape the service, or the metric disappears, every burn-rate alert goes silent, even if customers are failing. These alerts cover that blind spot.

## CheckoutTargetDown

Prometheus has failed to scrape `checkout-api` for 2 minutes.

1. In Prometheus → **Status → Targets**, read the scrape error (connection refused, timeout, DNS).
2. Check the service is running: `docker compose ps app` locally, or `kubectl get pods -l app=checkout-api` in a cluster.
3. Connection refused or crash-looping: check the service logs (`make logs S=app`). The service itself is down, which is a customer-facing outage.
4. Timeout only: the service may be overloaded, or `/metrics` is slow. Check the RED dashboard and the node's CPU.

## SLOSignalMissing

`http_requests_total` for the checkout route doesn't exist at all.

- **No traffic** (for example, the load generator or an upstream router stopped): confirm with the upstream team.
- **Metric renamed or relabeled** in a release: the SLO rules select on exact names and labels. Fix the instrumentation or update [`scripts/gen_slo_rules.py`](../../scripts/gen_slo_rules.py) in the same PR.
