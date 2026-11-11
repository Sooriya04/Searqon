# ─── Searqon Makefile ──────────────────────────────────────────────────────────

APP      := searqon
SRC_DIR  := ./src
BIN      := ./bin/${APP}
PORT     := 7493

# Docker Container Settings
IMAGE_NAME     := searqon
RELEASE_TAG    := release
LOCAL_IMAGE    := $(IMAGE_NAME):latest
RELEASE_IMAGE  := $(IMAGE_NAME):$(RELEASE_TAG)
CONTAINER_NAME := searqon-server

# Load .env file if it exists
ifneq (,$(wildcard .env))
  include .env
  export
endif

.PHONY: help run build clean test lint kill restart logs install-lightpanda \
        docker-build docker-release docker-run docker-stop docker-logs docker-test test-docker \
        compose-up compose-down compose-logs

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "  Default Port: $(PORT)"

run: ## Run server in dev mode (go run .), loads .env automatically
	@-kill $$(lsof -t -i:$(PORT)) 2>/dev/null || true
	@echo "→ Starting Searqon on :$(PORT)..."
	@echo "→ SearXNG → $(SEARXNG_URL)"
	cd $(SRC_DIR) && go run .

build: ## Compile binary → bin/searqon
	@echo "→ Building $(APP)..."
	@mkdir -p bin
	cd $(SRC_DIR) && go build -o ../bin/$(APP) .
	@echo "✓ Binary ready: bin/$(APP)"

clean: ## Remove compiled binary
	rm -f $(BIN)
	@echo "✓ Cleaned"

kill: ## Kill any process on port $(PORT)
	@-kill $$(lsof -t -i:$(PORT)) 2>/dev/null && echo "✓ Killed process on :$(PORT)" || echo "No process on :$(PORT)"

restart: kill ## Kill existing server and rerun
	sleep 0.5
	$(MAKE) run

test: ## Run all go tests
	cd $(SRC_DIR) && go test ./... -v

newman-test: ## Run Newman E2E test suite against running Searqon server
	npx --yes newman run tests/searqon.postman_collection.json -e tests/searqon.postman_environment.json

lint: ## Run go vet
	cd $(SRC_DIR) && go vet ./...

install-lightpanda: ## Download and install the Lightpanda headless browser binary locally
	@echo "→ Installing Lightpanda nightly binary..."
	@mkdir -p lightpanda
	curl -L -o ./lightpanda/lightpanda https://github.com/lightpanda-io/browser/releases/download/nightly/lightpanda-x86_64-linux
	chmod a+x ./lightpanda/lightpanda
	@echo "✓ Lightpanda binary installed successfully at ./lightpanda/lightpanda"

logs: ## Show server logs (for background process)
	@lsof -t -i:$(PORT) | xargs -I{} tail -f /proc/{}/fd/1 2>/dev/null || \
		echo "No running Searqon process found"

# ─── Docker Release & Local Workflows (Ultra-light 0 MB OS scratch) ───────────

docker-build: ## Build ultra-lightweight Docker image (scratch-based ~8MB)
	@echo "→ Building Docker image $(LOCAL_IMAGE)..."
	docker build -t $(LOCAL_IMAGE) -f Dockerfile .
	@echo "✓ Docker image ready: $(LOCAL_IMAGE)"

docker-release: ## Build standalone production Docker release image (searqon:release)
	@echo "→ Packaging Searqon production Docker release..."
	docker build -t $(RELEASE_IMAGE) -t $(LOCAL_IMAGE) -f Dockerfile .
	@echo "✓ Production Docker release ready: $(RELEASE_IMAGE) (~8MB)"

docker-run: docker-stop ## Run Searqon in Docker container on odd port $(PORT)
	@echo "→ Starting Searqon container on port $(PORT)..."
	docker run -d \
		--name $(CONTAINER_NAME) \
		-p $(PORT):$(PORT) \
		-v $(PWD)/config/settings.yml:/app/config/settings.yml:ro \
		-v $(PWD)/data:/app/data \
		$(LOCAL_IMAGE)
	@sleep 2
	@echo "✓ Container running at http://localhost:$(PORT)"

docker-stop: ## Stop and remove running Searqon Docker container
	@-docker rm -f $(CONTAINER_NAME) 2>/dev/null || true
	@echo "✓ Container $(CONTAINER_NAME) cleaned"

docker-logs: ## View live logs from Searqon Docker container
	@docker logs -f $(CONTAINER_NAME)

docker-test: ## Verify Docker container health and logs endpoints on port $(PORT)
	@echo "→ Testing container endpoints on :$(PORT)..."
	@curl -s http://localhost:$(PORT)/health | grep -q '"status":"ok"' && echo "✓ /health: OK" || echo "✗ /health: FAIL"
	@curl -s http://localhost:$(PORT)/logs | grep -q '"success":true' && echo "✓ /logs: OK" || echo "✗ /logs: FAIL"

test-docker: ## Build Docker image and run full CI smoke test (health, SQLite volume, MCP stdio)
	@echo "→ Building local Docker image searqon:ci..."
	@docker build -t searqon:ci .
	@echo "→ Booting container searqon-test..."
	@-docker rm -f searqon-test 2>/dev/null || true
	@rm -rf smoke-data && mkdir -p smoke-data
	@docker run -d --name searqon-test -p $(PORT):$(PORT) -v $$(pwd)/smoke-data:/app/data searqon:ci
	@echo "→ Polling health endpoint on :$(PORT)..."
	@sh -c ' \
		READY=0; \
		for i in $$(seq 1 20); do \
			if curl -sf http://localhost:$(PORT)/health | grep -q "\"status\":\"ok\""; then \
				READY=1; break; \
			fi; \
			sleep 0.5; \
		done; \
		if [ $$READY -ne 1 ]; then \
			echo "✗ Health check failed or timed out"; \
			docker logs searqon-test; \
			docker rm -f searqon-test; \
			rm -rf smoke-data; \
			exit 1; \
		fi'
	@echo "✓ Health check passed (HTTP 200 {\"status\":\"ok\"})"
	@echo "→ Verifying SQLite volume initialization..."
	@test -f smoke-data/searqon.db || { echo "✗ smoke-data/searqon.db missing"; docker rm -f searqon-test; rm -rf smoke-data; exit 1; }
	@echo "✓ SQLite volume initialized (smoke-data/searqon.db exists)"
	@echo "→ Testing MCP stdio pipeline..."
	@echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | docker run -i --rm searqon:ci mcp | grep -q '"tools"' || \
		{ echo "✗ MCP stdio tools/list test failed"; docker rm -f searqon-test; rm -rf smoke-data; exit 1; }
	@echo "✓ MCP stdio tools/list responded with tools"
	@echo "→ Tearing down smoke test container..."
	@docker rm -f searqon-test >/dev/null
	@rm -rf smoke-data
	@echo "✓ Docker smoke test passed cleanly!"

# ─── Full Stack Orchestration (Searqon + SearXNG + PG/SQLite + Redis) ────────

compose-up: ## Start complete stack (Searqon, SearXNG, Lightpanda, PG/SQLite, Redis)
	@echo "→ Starting Searqon full stack..."
	docker compose up -d
	@echo "✓ Searqon:    http://localhost:$(PORT)"
	@echo "✓ SearXNG:    http://localhost:8080"
	@echo "✓ Lightpanda: localhost:9222 (CDP)"
	@echo "✓ Postgres:   localhost:5435 (searqon-db)"
	@echo "✓ Redis:      localhost:6379"

compose-down: ## Stop complete stack
	@echo "→ Stopping full stack..."
	docker compose down
	@echo "✓ Stack stopped"

compose-logs: ## Tail logs for all services in docker-compose
	docker compose logs -f

