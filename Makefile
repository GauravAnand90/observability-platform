# Everything runs in containers: Docker is the only local requirement.
PROM_IMAGE    := prom/prometheus:v3.5.0
AM_IMAGE      := prom/alertmanager:v0.28.1
OTELCOL_IMAGE := otel/opentelemetry-collector-contrib:0.163.1
GO_IMAGE      := golang:1.26

.PHONY: help up down ps logs break slow fix status alerts test test-go test-rules check rules tidy

help: ## Show targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-11s %s\n", $$1, $$2}'

# ---------- Run the stack ----------

up: ## Build and start the stack
	docker compose up -d --build
	@echo ""
	@echo "  Grafana       http://localhost:3000   (SLO overview is the home dashboard)"
	@echo "  Prometheus    http://localhost:9090/alerts"
	@echo "  Alertmanager  http://localhost:9093"
	@echo "  checkout-api  http://localhost:8080/metrics"

down: ## Stop the stack and delete its data
	docker compose down -v

ps: ## Show container status
	docker compose ps

logs: ## Follow logs (S=service to filter, e.g. make logs S=alert-sink)
	docker compose logs -f $(S)

# ---------- Game day: break it on purpose ----------

break: ## Fail 30% of checkouts (pages within ~5 minutes)
	@curl -fsS -X POST 'http://localhost:8080/admin/fault?error_rate=0.3' && echo

slow: ## Add 400ms to every checkout (burns the latency SLO)
	@curl -fsS -X POST 'http://localhost:8080/admin/fault?latency_ms=400' && echo

fix: ## Remove all injected faults
	@curl -fsS -X POST 'http://localhost:8080/admin/fault?error_rate=0&latency_ms=0' && echo

status: ## Show current fault settings
	@curl -fsS http://localhost:8080/admin/fault && echo

alerts: ## Show notifications Alertmanager sent (what would have paged)
	@curl -fsS http://localhost:9095/alerts | python3 -m json.tool

# ---------- Checks (same as CI) ----------

test: test-go test-rules check ## Run every check

test-go: ## go vet + unit tests for the demo service
	docker run --rm -v "$(CURDIR)/app:/src" -w /src $(GO_IMAGE) sh -c "go mod tidy && go vet ./... && go test ./..."

test-rules: ## promtool unit tests for the SLO alerts
	docker run --rm -v "$(CURDIR)/prometheus:/p:ro" -w /p --entrypoint promtool $(PROM_IMAGE) test rules tests/slo-alerts.test.yml

check: ## Validate Prometheus, Alertmanager, Collector, dashboards and compose
	docker run --rm -v "$(CURDIR)/prometheus:/p:ro" -w /p --entrypoint promtool $(PROM_IMAGE) check config prometheus.yml
	docker run --rm -v "$(CURDIR)/alertmanager:/a:ro" --entrypoint amtool $(AM_IMAGE) check-config /a/alertmanager.yml
	docker run --rm -v "$(CURDIR)/otel-collector:/c:ro" $(OTELCOL_IMAGE) validate --config=/c/config.yaml
	@for f in grafana/dashboards/*.json; do python3 -m json.tool "$$f" >/dev/null && echo "valid JSON: $$f" || exit 1; done
	docker compose config -q && echo "docker-compose.yml is valid"

rules: ## Regenerate SLO recording rules from scripts/gen_slo_rules.py
	python3 scripts/gen_slo_rules.py

tidy: ## Resolve Go modules and write go.sum (commit the result)
	docker run --rm -v "$(CURDIR)/app:/src" -w /src $(GO_IMAGE) go mod tidy
