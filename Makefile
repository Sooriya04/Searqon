# ─── Searqon Makefile ──────────────────────────────────────────────────────────

APP      := searqon
SRC_DIR  := ./go_scraper
BIN      := ./${APP}
PORT     := 4001

.PHONY: help run build clean test kill restart searxng logs

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "  SearXNG: docker run -d --name searxng -p 8080:8080 searxng/searxng"

run: ## Run server in dev mode (go run .)
	@echo "→ Starting Searqon on :$(PORT)..."
	cd $(SRC_DIR) && go run .

build: ## Compile-check only (output discarded — users run via 'make run')
	@echo "→ Build check..."
	cd $(SRC_DIR) && go build -o /dev/null .
	@echo "✓ Build OK"

clean: ## Remove compiled binary
	rm -f $(BIN)
	@echo "✓ Cleaned"

kill: ## Kill any process on port 3001
	@-kill $$(lsof -t -i:$(PORT)) 2>/dev/null && echo "✓ Killed process on :$(PORT)" || echo "No process on :$(PORT)"

restart: kill ## Kill existing server and rerun
	sleep 0.5
	$(MAKE) run

test: ## Run all go tests
	cd $(SRC_DIR) && go test ./... -v

lint: ## Run go vet
	cd $(SRC_DIR) && go vet ./...

searxng: ## Start SearXNG via Docker (primary search provider)
	@echo "→ Starting SearXNG on :8080..."
	docker run -d --name searxng -p 8080:8080 searxng/searxng || \
		docker start searxng
	@echo "✓ SearXNG running at http://localhost:8080"

searxng-stop: ## Stop SearXNG
	docker stop searxng && echo "✓ SearXNG stopped"

logs: ## Show server logs (for background process)
	@lsof -t -i:$(PORT) | xargs -I{} tail -f /proc/{}/fd/1 2>/dev/null || \
		echo "No running Searqon process found"
