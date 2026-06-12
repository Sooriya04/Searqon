# Searqon — Web Intelligence Engine

Searqon is an open-source, self-hosted web intelligence engine that **searches, crawls, extracts, ranks, and synthesizes** information from the internet. Built entirely in Go for maximum performance, minimal resource usage, and total transparency.

> **Searqon is not a chatbot.** It is the foundational search and web retrieval layer designed to power AI agents, RAG (Retrieval-Augmented Generation) systems, and knowledge-driven applications.

Think of it as a transparent alternative to services like Tavily — where you control the entire pipeline, from how sources are selected to how content is extracted and cleaned.

---

## 📖 Documentation

Complete, in-depth documentation is available in the [`docs/`](./docs/index.md) directory:

- [**Installation Guide**](./docs/installation.md) — How to set up and run Searqon locally.
- [**Architecture**](./docs/architecture.md) — Five-stage pipeline, component breakdown, and concurrency flow.
- [**API Reference**](./docs/api.md) — Request/Response structures for all endpoints.
- [**Search Providers**](./docs/providers.md) — Details on SearXNG and DuckDuckGo Lite.
- [**Configuration**](./docs/configuration.md) — Tunable port, timeouts, limits, and user-agents.
- [**Deployment**](./docs/deployment.md) — Docker, systemd, and reverse proxy setup.

---

## Key Features

- **Decoupled Architecture:** Single compiled Go binary. Port `4001`. ~20MB idle RAM.
- **Dual-Provider Search Chain:** Queries a local SearXNG instance (primary) and automatically falls back to DuckDuckGo Lite HTML (no-JS, parsed via `goquery`) if SearXNG is unavailable.
- **Concurrent Scraper:** Goroutines fetch result pages in parallel under a global 2.5-second context timeout, falling back gracefully to search snippets if a page blocks or hangs.
- **Robots.txt Auditing:** Respects website crawling rules and handles domain politeness.
- **Site Crawling & Mapping:** Includes endpoints to index entire sites (with SSE streaming support) and map link structures.

---

## API Endpoints

| Method | Endpoint       | Description                                                        |
|--------|----------------|--------------------------------------------------------------------|
| POST   | `/search`      | Full pipeline: search → scrape → structured JSON                  |
| GET    | `/search?q=..` | Fast query-string search (supports `scrape=false`)                |
| POST   | `/scrape`      | Scrape a single URL → markdown/text                               |
| POST   | `/scrape/batch`| Scrape multiple URLs concurrently (parallel goroutines)           |
| POST   | `/scrape/html` | Parse raw HTML directly into clean markdown                       |
| POST   | `/crawl`       | Crawl an entire site (supports depth, page limit, and SSE streaming)|
| POST   | `/map`         | Discover all internal URLs on a domain                            |
| GET    | `/health`      | Health check                                                      |

---

## Quick Start

### 1. Run via Makefile
The simplest way to run and manage Searqon:

```bash
# Run server in dev mode (compiles & runs via go run)
make run

# Kill any process running on port 4001
make kill

# Restart the server
make restart
```

### 2. Search Provider Options

Searqon is completely independent of external services. By default, it runs in **DuckDuckGo fallback mode**. For production deployments and higher rate limits, spin up SearXNG:

```bash
# Start SearXNG locally via Docker
make searxng
```

---

## Usage Examples

```bash
# Search and scrape the top 3 results concurrently (returns markdown/text)
curl -X POST http://localhost:4001/search \
  -H "Content-Type: application/json" \
  -d '{"query": "what is ycombinator", "limit": 5, "scrape": true}'

# Search only — fast metadata, no page scraping
curl -X POST http://localhost:4001/search \
  -H "Content-Type: application/json" \
  -d '{"query": "anthropic claude 3.5", "limit": 5, "scrape": false}'

# Scrape a single web page directly
curl -X POST http://localhost:4001/scrape \
  -H "Content-Type: application/json" \
  -d '{"url": "https://en.wikipedia.org/wiki/Y_Combinator"}'
```

---

## Project Philosophy

> Build the system first. Add intelligence later.

Searqon prioritizes **transparency, modularity, and control**. Every decision in the retrieval pipeline is visible and customizable — no black boxes, no hidden APIs.

---

## License

MIT License. See [LICENSE](LICENSE) for details.
