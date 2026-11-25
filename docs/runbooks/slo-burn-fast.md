# Runbook: SLOErrorBudgetBurnFast

**Severity:** page · **SLOs:** any (`slo` label says which) · **Dashboard:** [SLO overview](http://localhost:3000/d/slo-overview)

## What it means

The SLO is burning error budget at 14.4x (1h) or 6x (6h) the sustainable rate. Customers are affected now. At 14.4x the entire 30-day budget is gone in about two days; at 6x, in about five.

## First 5 minutes

1. **Acknowledge** the page and post in the incident channel: "Investigating `<slo>` fast burn."
2. **Check scope** on the [SLO overview](http://localhost:3000/d/slo-overview): which windows are above threshold? A 5m spike with a 1h tail means it's still happening; if the 5m burn has dropped, it may already be recovering.
3. **What changed?** Deploys, config changes, feature flags, dependency incidents in the last hour. A recent deploy is the most likely cause. **Roll back first, debug second.**

## Find the cause with traces

1. Open [Checkout API: RED and traces](http://localhost:3000/d/checkout-red).
2. **Availability SLO:** in *Recent error traces*, open a few traces. The span marked with an error (for example `payment.authorize`) names the failing dependency, and its error event has the message.
3. **Latency SLO:** in *Slow traces (> 300ms)*, compare span durations. One span taking most of the time points to that dependency; evenly slow spans point to CPU saturation or GC on the service itself.
4. In Grafana Explore → Tempo → **Service graph**, red edges show which caller-to-callee link is failing.

## Mitigate

| Cause | Action |
|---|---|
| Bad deploy | Roll back. |
| Dependency failing (e.g. payment gateway) | Fail over to the secondary provider or enable the degraded-mode feature flag. Open an incident with the provider. |
| Overload | Scale out; check whether a client is retrying aggressively. |
| Unknown | Escalate to the secondary on-call after 15 minutes without progress. |

**In the local demo:** `make status` shows injected faults, and `make fix` removes them.

## After

- The alert resolves on its own once the short window (5m or 30m) drops below threshold.
- If more than 20% of the budget was spent, write a postmortem and apply the [error budget policy](../slo/checkout-api.md#error-budget-policy).
