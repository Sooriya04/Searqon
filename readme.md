# Searqon — Web Intelligence Engine for AI Agents

[![Docker Hub](https://img.shields.io/docker/v/sooriya04/searqon?label=Docker%20Hub&color=2496ed)](https://hub.docker.com/r/sooriya04/searqon)
[![Version](https://img.shields.io/badge/version-v1.2.0-blue)](https://github.com/Sooriya04/Searqon/releases)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

Searqon is an open-source, self-hosted web intelligence engine that **searches, crawls, scrapes, ranks, and synthesizes** information from the internet. Built entirely in Go for maximum performance, minimal resource usage, and total transparency.

> **Searqon is not a chatbot.** It is the foundational search and web retrieval layer designed to power AI agents, RAG (Retrieval-Augmented Generation) systems, and knowledge-driven applications.

Think of it as a transparent alternative to services like Tavily or Firecrawl — where you **control the entire pipeline**, from how sources are selected to how content is extracted and cleaned.

---

## 🤖 MCP (Model Context Protocol) — Use Searqon in Claude Desktop, Cursor & Windsurf

Searqon `v1.2.0` ships with a **native Go MCP server**, making it the simplest way to give any AI agent access to real-time web search and sophisticated anti-bot scraping.

### Claude Desktop / Cursor Configuration
```json
{
  "mcpServers": {
    "searqon": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "SEARXNG_URL=http://host.docker.internal:8080",
        "-v", "searqon-data:/app/data",
        "sooriya04/searqon:latest",
        "mcp"
      ]
    }
  }
}
```

### MCP Tools Available to Your AI Agent

| Tool | Description |
| :--- | :--- |
| `searqon_search` | Real-time meta-search via SearXNG + DuckDuckGo + Arxiv + Wikipedia with reciprocal rank fusion |
| `searqon_scrape` | 4-tier anti-bot scraper (Cloudflare bypass, headless fallback, proxy rotation) → clean Markdown with `max_tokens` guard |
| `searqon_research` | One-shot deep retrieval: search → concurrently scrape top sources → return structured research dossier |
| `searqon_map` | Domain discovery & sitemap crawler: map internal URL trees, paths, and site hierarchies |
| `searqon_extract` | Targeted CSS selector or structured JSON schema extraction for tables, prices, and metadata |

Full MCP integration guide: [`MCP.md`](./MCP.md) | [`docs/MCP.md`](./docs/MCP.md)

---

## 📖 Documentation

Complete documentation is available in the [`docs/`](./docs/index.md) directory:

- [**MCP Integration Guide**](./MCP.md) — Claude Desktop, Cursor, Windsurf, and custom agents.
- [**Docker Hub Guide**](./docs/docker.md) — Docker setup, compose, and environment variables.
- [**Orchestration Workflow**](./docs/workflow/workflow.md) — High-level query pipeline, fallback chains, and lifecycle.
- [**Scraping Specification**](./docs/workflow/scraping.md) — Concurrency queues, robots.txt compliance, and Lightpanda.
- [**Stealth & Anti-Bot Escalation**](./docs/stealth_escalation.md) — 4-tier chained escalation (Fast HTTP → Spoofed Headers → Camoufox/Lightpanda → Residential Proxy).
- [**Architecture & Config**](./docs/architecture.md) — Database schema, variables, and error policies.
- [**API Reference**](./docs/api.md) — Request/Response structures for all endpoints.

---

## ⚡ Key Features

- **Native MCP Server (v1.2.0):** Run as `docker run -i ... mcp` for instant Claude Desktop / Cursor integration. Also exposes HTTP SSE at `/mcp` and `/mcp/sse`.
- **4-Tier Anti-Bot Escalation:** Fast HTTP → Spoofed Chrome/TLS Fingerprint → Camoufox/Lightpanda Headless → Rotating Residential Proxy. Bypasses Cloudflare, DataDome, and PerimeterX.
- **SearXNG Meta-Search:** Aggregates SearXNG, DuckDuckGo, Arxiv, and Wikipedia with reciprocal rank fusion and deduplication.
- **Context Window Protection:** Token budget (`max_tokens`) clamps scraped Markdown to safe sizes for LLMs.
- **Ultra-Lightweight Docker (~8 MB):** Single static Go binary on a `scratch` base image. Starts in <10ms.
- **Dual Storage:** SQLite (default, zero setup) or PostgreSQL + Redis for production scale.
- **Persistent Caching:** SQLite/Redis L1 cache, sub-5ms recurring query speeds.
- **Dual-Provider Search Chain:** SearXNG primary with automatic DuckDuckGo Lite fallback.

---

## 🚀 Quick Start

### Run via Docker (Recommended)
```bash
# Start as a web API server
docker run -p 7493:7493 -v searqon-data:/app/data sooriya04/searqon:latest

# Start as an MCP stdio server (for Claude Desktop / Cursor)
docker run -i --rm -v searqon-data:/app/data sooriya04/searqon:latest mcp
```

### Run via Docker Compose (Full Stack with SearXNG + Redis)
```bash
docker compose up -d
```

### Run via Makefile (Local Dev)
```bash
make run       # compile & run on port 7493
make kill      # kill server
make restart   # restart
```

---

## 🌐 API Endpoints

| Method | Endpoint         | Description                                              |
|--------|------------------|----------------------------------------------------------|
| POST   | `/search`        | Full pipeline: meta-search → scrape → structured JSON   |
| GET    | `/search?q=..`   | Fast query-string search                                 |
| GET    | `/search/stream` | Real-time SSE streaming of search results               |
| POST   | `/scrape`        | Scrape a single URL → clean Markdown (4-tier anti-bot)  |
| POST   | `/scrape/chunked`| RAG-ready chunked scrape with token counts              |
| POST   | `/scrape/batch`  | Scrape multiple URLs concurrently                        |
| POST   | `/crawl`         | Recursive site crawler with SSE streaming               |
| POST   | `/map`           | Discover all internal URLs on a domain                  |
| GET    | `/mcp`           | MCP Server-Sent Events endpoint                         |
| POST   | `/mcp`           | MCP JSON-RPC endpoint (remote agents)                   |
| GET    | `/health`        | Health check and system status                          |
| GET    | `/metrics`       | Prometheus-compatible telemetry                          |

---

## Usage Examples

```bash
# Search and scrape the top 5 results concurrently
curl -X POST http://localhost:7493/search \
  -H "Content-Type: application/json" \
  -d '{"query": "what is ycombinator", "limit": 5, "scrape": true}'

# Anti-bot scrape a single page → clean Markdown
curl -X POST http://localhost:7493/scrape \
  -H "Content-Type: application/json" \
  -d '{"url": "https://en.wikipedia.org/wiki/Y_Combinator", "format": "markdown"}'

# RAG-ready chunked scrape
curl -X POST http://localhost:7493/scrape/chunked \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com", "chunk_size": 500}'
```

---

## Project Philosophy

> Build the system first. Add intelligence later.

Searqon prioritizes **transparency, modularity, and control**. Every decision in the retrieval pipeline is visible and customizable — no black boxes, no hidden APIs. Designed to be the web-access layer for AI agents that need real-world information.

---

## License

MIT License. See [LICENSE](LICENSE) for details.
