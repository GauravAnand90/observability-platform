# ADR 0003: Route by severity; pages only for fast burns and lost signal

**Status:** Accepted

## Context

On-call fatigue comes from pages that aren't urgent or actionable. Every page should mean: customers are affected now, and a human needs to act now.

## Decision

- Two severities only: **`page`** (interrupt on-call, any hour) and **`ticket`** (team queue, business hours).
- **Page:** fast SLO burn (14.4x or 6x) and `CheckoutTargetDown`, because losing the SLO signal hides real outages.
- **Ticket:** slow SLO burn (3x or 1x) and `SLOSignalMissing`.
- Alertmanager **inhibits** the ticket while a page is firing for the same `{service, slo}`, so one problem creates one notification.
- Grouped by `alertname, service, slo`. Pages use `group_wait: 10s` and repeat hourly while unresolved; tickets repeat every 12h.
- Every alert carries a `runbook_url` and a `dashboard_url`, and CI tests the text, so nobody gets paged with an empty description.
- No cause-based pages (CPU, memory, pod restarts). Those go to dashboards. If they hurt users, the SLO alerts catch it.

## Consequences

- Pages map directly to user impact. Cause-based signals are still available, but they don't wake anyone up.
- Alert definitions are unit-tested with `promtool`, so a threshold or PromQL change that breaks paging fails CI.
