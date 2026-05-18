# Alebus Development Makefile
# Phase 5B-0: Local Infrastructure Bootstrap

.PHONY: help dev-preflight dev-up dev-down dev-logs dev-status dev-verify db-up db-down db-logs db-shell migrate-up migrate-down migrate-version migrate-create test-infra test-all lint lint-install verify-postgis clean api-client-ts-install api-client-ts-typecheck api-client-ts-build api-client-ts-clean api-openapi-lint

# Default target
help:
	@echo "Alebus Development Commands"
	@echo ""
	@echo "API Clients:"
	@echo "  make api-client-ts-install   Install TS client deps (api/clients/ts)"
	@echo "  make api-client-ts-typecheck Typecheck TS client"
	@echo "  make api-client-ts-build     Build TS client (emits dist/)"
	@echo "  make api-client-ts-clean     Remove dist/ + node_modules/ for TS client"
	@echo ""
	@echo "API Docs:"
	@echo "  make api-openapi-lint        Lint api/openapi.yaml (Redocly CLI via npx)"
	@echo ""
	@echo "Database:"
	@echo "  make db-up           Start Postgres container"
	@echo "  make db-down         Stop and remove Postgres container (keeps data)"
	@echo "  make db-reset        Stop, remove container AND delete data volume"
	@echo "  make db-logs         View Postgres logs"
	@echo "  make db-shell        Open psql shell in container"
	@echo ""
	@echo "Migrations:"
	@echo "  make migrate-up      Run all pending migrations"
	@echo "  make migrate-down    Rollback last migration"
	@echo "  make migrate-version Show current migration version"
	@echo "  make migrate-create NAME=xxx  Create new migration files"
	@echo ""
	@echo "Testing:"
	@echo "  make test-infra      Run infrastructure tests"
	@echo "  make test-all        Run all tests"
	@echo "  make lint           Run golangci-lint"
	@echo ""
	@echo "Dev Stack (Canonical):"
	@echo "  make dev-up          Start canonical dev stack (postgres, redis, emqx, ingestor, worker, simulator, api)"
	@echo "  make dev-down        Stop dev stack"
	@echo "  make dev-status      Show running containers and ports"
	@echo "  make dev-logs        Tail logs from all dev services"
	@echo "  make dev-verify      Verify all services are healthy"
	@echo "  make dev-preflight   Run safety checks before starting stack"
	@echo ""
	@echo "Verification:"
	@echo "  make verify-postgis  Verify PostGIS is installed"
	@echo "  make verify-db       Full database verification"
	@echo ""

# ============================================================================
# Dev Stack Management (Canonical)
# ============================================================================

dev-preflight:
	@echo "🔍 Running pre-flight checks..."
	@bash scripts/docker-safety-check.sh

dev-up: dev-preflight
	@echo "🚀 Starting canonical dev stack..."
	docker compose -f compose/dev.yml --profile journey-worker up -d --build
	@echo ""
	@echo "✅ Dev stack is running."
	@echo "   📊 Verify: make dev-status"
	@echo "   📋 Logs: make dev-logs"
	@echo "   🔌 simulation_preview proxies to http://127.0.0.1:9090"

dev-down:
	@echo "⏹️  Stopping dev stack..."
	docker compose -f compose/dev.yml --profile journey-worker down

dev-logs:
	@echo "📋 Tailing logs..."
	docker compose -f compose/dev.yml --profile journey-worker logs -f

dev-status:
	@echo "=== Active Dev Stack Containers ==="
	@docker ps --filter "name=alebus-" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
	@echo ""
	@echo "=== Expected Ports ==="
	@echo "  5432   postgres"
	@echo "  6382   redis (host) → 6379 (container)"
	@echo "  1883   emqx (MQTT)"
	@echo "  18083  emqx (dashboard)"
	@echo "  9100   ingestor"
	@echo "  9090   simulator (dev API) ← simulation_preview target"
	@echo "  8080   api (prod-like, optional)"

dev-verify:
	@echo "=== Dev Stack Verification ==="
	@echo ""
	@echo "1. Postgres:"
	@docker exec alebus-postgres pg_isready -U alebus -d alebus
	@echo ""
	@echo "2. Redis:"
	@docker exec alebus-redis-emqx redis-cli ping
	@echo ""
	@echo "3. EMQX:"
	@curl -sf http://127.0.0.1:18083 > /dev/null && echo "EMQX dashboard accessible" || echo "EMQX dashboard NOT accessible"
	@echo ""
	@echo "4. Simulator API (9090):"
	@curl -sf http://127.0.0.1:9090/health > /dev/null && echo "Simulator health OK" || echo "Simulator health FAILED"
	@echo ""
	@echo "5. API (8080):"
	@curl -sf http://127.0.0.1:8080/health > /dev/null && echo "API health OK" || echo "API health FAILED"
	@echo ""
	@echo "✅ Verification complete."

# ============================================================================
# API Client Commands
# ============================================================================

api-client-ts-install:
	@echo "Installing TypeScript API client dependencies..."
	@cd api/clients/ts && npm install

api-client-ts-typecheck:
	@echo "Typechecking TypeScript API client..."
	@cd api/clients/ts && npm run typecheck

api-client-ts-build:
	@echo "Building TypeScript API client..."
	@cd api/clients/ts && npm run build

api-client-ts-clean:
	@echo "Cleaning TypeScript API client build artifacts..."
	@cd api/clients/ts && rm -rf dist node_modules package-lock.json

api-openapi-lint:
	@echo "Linting OpenAPI spec (api/openapi.yaml)..."
	@npx -y @redocly/cli@latest lint --config .redocly.yaml api/openapi.yaml

# ============================================================================
# Database Commands
# ============================================================================

db-up:
	@echo "Starting Postgres with PostGIS..."
	docker compose -f compose/dev.yml up -d postgres
	@echo "Waiting for database to be ready..."
	@sleep 3
	@docker compose -f compose/dev.yml ps

db-down:
	@echo "Stopping Postgres..."
	docker compose -f compose/dev.yml down

db-reset:
	@echo "Stopping Postgres and removing data volume..."
	docker compose -f compose/dev.yml down -v

db-logs:
	docker compose -f compose/dev.yml logs -f postgres

db-shell:
	docker compose -f compose/dev.yml exec postgres psql -U alebus -d alebus

# ============================================================================
# Migration Commands
# ============================================================================

migrate-up:
	@echo "Running migrations..."
	migrate -path infrastructure/migrations -database "$(DATABASE_URL)" up

migrate-down:
	@echo "Rolling back last migration..."
	migrate -path infrastructure/migrations -database "$(DATABASE_URL)" down 1

migrate-version:
	@echo "Current migration version:"
	migrate -path infrastructure/migrations -database "$(DATABASE_URL)" version

migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "Error: NAME is required. Usage: make migrate-create NAME=create_users_table"; \
		exit 1; \
	fi
	@echo "Creating migration: $(NAME)"
	migrate create -ext sql -dir infrastructure/migrations -seq $(NAME)

# ============================================================================
# Testing Commands
# ============================================================================

test-infra:
	@echo "Running infrastructure tests..."
	go test -v ./infrastructure/...

test-all:
	@echo "Running all tests..."
	go test -v ./...

lint:
	@echo "Running golangci-lint..."
	@golangci-lint version >/dev/null 2>&1 || (echo "golangci-lint not found. Run: make lint-install" && exit 1)
	golangci-lint run ./...

lint-install:
	@echo "Installing golangci-lint (go install)..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
	@echo "Installed. Ensure GOPATH/bin is on your PATH."

# ============================================================================
# Verification Commands
# ============================================================================

verify-postgis:
	@echo "Verifying PostGIS installation..."
	@docker compose -f compose/dev.yml exec postgres psql -U alebus -d alebus -c "SELECT PostGIS_Version();" || \
		(echo "PostGIS verification failed!" && exit 1)
	@echo "PostGIS is working!"

verify-db:
	@echo "=== Database Verification ==="
	@echo ""
	@echo "1. Checking container status..."
	@docker compose -f compose/dev.yml ps
	@echo ""
	@echo "2. Checking database connection..."
	@docker compose -f compose/dev.yml exec postgres pg_isready -U alebus -d alebus
	@echo ""
	@echo "3. Checking PostGIS..."
	@docker compose -f compose/dev.yml exec postgres psql -U alebus -d alebus -c "SELECT PostGIS_Version();"
	@echo ""
	@echo "=== Verification Complete ==="

# ============================================================================
# Cleanup
# ============================================================================

clean:
	@echo "Cleaning up..."
	docker compose -f compose/dev.yml down -v
	@echo "Done."


#migrate -path infrastructure/migrations -database "postgres://alebus:alebus@localhost:5432/alebus?sslmode=disable" up