# ADR 0001: Alert on error-budget burn rate, using multiple windows

**Status:** Accepted

## Context

Typical alerts like "error rate > 1% for 5 minutes" have two problems:

- **They page for things that don't matter.** A 5-minute blip at 2% errors uses a tiny fraction of a 30-day budget, yet it wakes someone up.
- **They miss slow problems.** A steady 0.5% error rate never crosses the threshold, but it would exhaust a 99.9% SLO's budget in about a week.

## Decision

Alert on **burn rate**: how many times faster than "exactly on budget" the SLO is consuming its error budget. Use the multi-window, multi-burn-rate approach from the Google SRE Workbook:

| Burn rate | Long window | Short window | Severity |
|---|---|---|---|
| 14.4x | 1h | 5m | page |
| 6x | 6h | 30m | page |
| 3x | 1d | 2h | ticket |
| 1x | 3d | 6h | ticket |

- The **long window** makes sure enough budget has been spent to justify the alert (14.4x over 1h = 2% of a 30-day budget).
- The **short window**, at 1/12 of the long one, has to be burning too. Without it, an alert would keep firing for up to an hour after the problem is fixed.
- Rules are generic over `{service, slo}` labels, so a new SLO needs only its SLI definition; alerts, routing and dashboards apply automatically.

## Consequences

- Fewer, more meaningful pages. Each alert is tied to user impact and to how fast the budget is going.
- Slow burns become tickets instead of being missed.
- Eight recording rules per SLO (one per window plus 30d). This is cheap at this scale, and recording them keeps the alert expressions readable and fast.
- Low-traffic services can make ratios noisy. For those, add a minimum request-rate condition to the alerts.
