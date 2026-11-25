# Runbook: SLOErrorBudgetBurnSlow

**Severity:** ticket (business hours) · **Dashboard:** [SLO overview](http://localhost:3000/d/slo-overview)

## What it means

The SLO is consuming budget faster than it can afford over 1–3 days (3x over 1 day, or 1x over 3 days), but not fast enough to wake anyone. Left alone, the budget runs out before the 30-day window ends.

Alertmanager suppresses this ticket while a fast-burn page is firing for the same SLO, so this alert always means a slow, steady problem.

## Investigate

1. On the [SLO overview](http://localhost:3000/d/slo-overview), look at the **1d and 3d burn rates** and when they started rising. Line that up with deploys and config changes.
2. Look for patterns in [error and slow traces](http://localhost:3000/d/checkout-red): a single endpoint, customer, region or dependency.
3. Typical slow-burn causes: a small error rate introduced by a release, gradual latency growth (database table growth, missing index, cache hit rate dropping), or a dependency that is degraded but not down.

## Act

- Create a ticket with the evidence (burn-rate graph, example trace IDs) for the owning team, prioritized for this sprint.
- Check the [error budget policy](../slo/checkout-api.md#error-budget-policy). If remaining budget is below 50%, the release rules there apply.
