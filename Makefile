# ─── Searqon Makefile ──────────────────────────────────────────────────────────

APP      := searqon
SRC_DIR  := ./src
BIN      := ./bin/${APP}
PORT     := 4001

# Load .env file if it exists
ifneq (,$(wildcard .env))
  include .env
  export
endif

.PHONY: help run build clean test kill restart logs

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "  Env: Configure SEARXNG_URL and DATABASE_URL in .env"

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

kill: ## Kill any process on port 4001
	@-kill $$(lsof -t -i:$(PORT)) 2>/dev/null && echo "✓ Killed process on :$(PORT)" || echo "No process on :$(PORT)"

restart: kill ## Kill existing server and rerun
	sleep 0.5
	$(MAKE) run

test: ## Run all go tests
	cd $(SRC_DIR) && go test ./... -v

lint: ## Run go vet
	cd $(SRC_DIR) && go vet ./...

logs: ## Show server logs (for background process)
	@lsof -t -i:$(PORT) | xargs -I{} tail -f /proc/{}/fd/1 2>/dev/null || \
		echo "No running Searqon process found"
