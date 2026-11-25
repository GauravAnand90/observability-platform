# SLO: checkout-api

| | |
|---|---|
| **Service** | checkout-api (`POST /api/checkout`) |
| **Owner** | Checkout team (on-call rotation: `checkout-oncall`) |
| **SLO window** | Rolling 30 days |
| **Status** | Agreed with product, reviewed quarterly |

## Why these SLOs

Checkout is the point where a customer pays. Customers notice two things: whether the checkout succeeded, and whether it felt instant. Each SLO measures one of those, at the load balancer's view of the request, which is the closest point to the user we control.

## SLIs and objectives

| SLO | SLI (good events / valid events) | Objective | Error budget (30d) |
|---|---|---|---|
| `checkout-availability` | Requests that did **not** return 5xx / all requests | **99.9%** | 0.1% ≈ 43 minutes of full outage |
| `checkout-latency` | Requests completed in **≤ 300ms** / all requests | **99%** | 1% of requests may be slower |

- 4xx responses count as **good**: they're the client's error, and counting them would let bad clients burn our budget.
- The 300ms threshold lines up with a histogram bucket boundary (`le="0.3"`), so the SLI is exact rather than interpolated.
- Source metrics: `http_requests_total` and `http_request_duration_seconds` from the service's `/metrics`. PromQL lives in [`prometheus/rules/slo-recording.yml`](../../prometheus/rules/slo-recording.yml).

## Alerting

Multi-window, multi-burn-rate ([ADR 0001](../adr/0001-multiwindow-burn-rate-alerts.md)):

| Alert | Fires when | Budget spent before it fires | Response |
|---|---|---|---|
| `SLOErrorBudgetBurnFast` | 14.4x over 1h **and** 5m, or 6x over 6h **and** 30m | 2% / 5% | **Page** on-call |
| `SLOErrorBudgetBurnSlow` | 3x over 1d **and** 2h, or 1x over 3d **and** 6h | 10% | **Ticket**, next business day |

## Error budget policy

The budget is the agreed amount of unreliability. It's spent on purpose (releases, experiments, migrations) and protected when it runs low.

| Budget remaining (30d) | What happens |
|---|---|
| **> 50%** | Normal. Ship as usual. |
| **25–50%** | Releases need a rollback plan written in the PR. Review the top error sources in the weekly ops review. |
| **0–25%** | Feature releases need sign-off from the team lead. At least one engineer works on reliability items from recent incidents. |
| **< 0% (SLO missed)** | Feature freeze for checkout-api, except fixes and security patches, until the 30-day SLI is back above objective. A postmortem is written for the incidents that used the budget. |

Exceptions (e.g. a business-critical launch) are agreed between the engineering manager and product owner, and recorded in this document.

## Review

The SLO is reviewed every quarter, or after any incident that used more than 20% of the budget. Questions asked each time:

1. Did the SLO page us for things customers didn't notice? Then the objective may be too strict.
2. Did customers complain while the SLO looked healthy? Then the SLI measures the wrong thing.
3. Is the budget consistently unused? Then we could ship faster, or the objective is too loose.
