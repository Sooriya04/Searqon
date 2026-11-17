# Searqon — Web Intelligence Engine for AI Agents

[![Docker Hub](https://img.shields.io/docker/v/sooriya04/searqon?label=Docker%20Hub&color=2496ed)](https://hub.docker.com/r/sooriya04/searqon)
[![GHCR](https://img.shields.io/badge/GHCR-ghcr.io%2Fsooriya04%2Fsearqon-24292e?logo=github)](https://github.com/Sooriya04/Searqon/pkgs/container/searqon)
[![Version](https://img.shields.io/badge/version-v1.2.1-blue)](https://github.com/Sooriya04/Searqon/releases)
[![UTCP Spec](https://img.shields.io/badge/UTCP-v1.0.1%2B-blueviolet)](https://github.com/universal-tool-calling-protocol)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

Searqon is an open-source, self-hosted web intelligence engine that **searches, crawls, scrapes, ranks, and synthesizes** information from the internet. Built entirely in Go for maximum performance, minimal resource usage, and total transparency.

> **Searqon is not a chatbot.** It is the foundational search and web retrieval layer designed to power AI agents, RAG (Retrieval-Augmented Generation) systems, and knowledge-driven applications.

Think of it as a transparent alternative to services like Tavily or Firecrawl — where you **control the entire pipeline**, from how sources are selected to how content is extracted and cleaned.

---

## ⚡ 1-Line Quick Install

Install the standalone static binary (Linux & macOS, amd64 & arm64) in 2 seconds:

```bash
curl -fsSL https://raw.githubusercontent.com/Sooriya04/Searqon/main/install.sh | sh
```

Or spin up the ultra-lightweight **`~8.4 MB` scratch container**:
```bash
docker run -d -p 7493:7493 -v $(pwd)/data:/app/data sooriya04/searqon:latest
```

---

## 📊 Performance & Architecture Benchmarks

Searqon is engineered in pure Go (0 CGO) to eliminate the multi-gigabyte memory and CPU bloat of traditional headless scraper runtimes.

| Metric | **Searqon v1.2.1** 🚀 | **Firecrawl** (Self-hosted) | **Raw Playwright / Puppeteer** | **Hosted APIs** *(Tavily/Perplexity)* |
| :--- | :--- | :--- | :--- | :--- |
| **Container Image** | **`8.42 MB`** *(scratch)* | `~1,450 MB` | `~1,800 MB` | N/A *(Hosted SaaS)* |
| **Idle Memory (RAM)** | **`~14 MB`** | `~380 MB` *(Node + Redis worker)* | `~300 MB` *(Chromium daemon)* | N/A |
| **Peak RAM (50 Concurr.)**| **`~48 MB`** | `~2,400 MB+` | `~3,500 MB+` | N/A |
| **HTML → Markdown Speed** | **`0.013 ms`** *(13,041 ns)* | `~12.4 ms` *(Cheerio/turndown)* | `~18.2 ms` *(DOM serial.)* | `~150–400 ms` API trip |
| **Throughput (1 Core)** | **`76,000+ pages/sec`** | `~80 pages/sec` | `~55 pages/sec` | Subject to API Rate Limits |
| **Token Budgeting** | **Deterministic early exit** | Post-processing truncation | Post-processing truncation | Pay-per-credit |
| **Self-Hosted Cost** | **`$0 / month`** | Requires large VPS | Requires large VPS | **`$20 – $100+ / mo`** |

👉 Detailed micro-benchmarks and reproduction steps: [`benchmarks/README.md`](./benchmarks/README.md)

---

## 🤖 MCP (Model Context Protocol) — Use Searqon in Claude Desktop, Cursor & Windsurf

Searqon `v1.2.1` ships with a **native Go MCP server**, making it the simplest way to give any AI agent access to real-time web search and sophisticated anti-bot scraping.

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

## 🔌 Dual Native Agent Protocols: MCP & UTCP (Universal Tool Calling Protocol)

Searqon provides **first-class dual native protocol support** for modern AI agent architectures:

| Dimension | MCP (Model Context Protocol) | UTCP (Universal Tool Calling Protocol) |
| :--- | :--- | :--- |
| **Primary Target** | Desktop chat harnesses (Claude Desktop, Cursor, Windsurf) | Autonomous agent frameworks, microservices, and Code-Mode sandboxes |
| **Discovery Route** | `tools/list` JSON-RPC method | `GET /utcp` or `GET /utcp/manual` returning standard UTCP JSON manual |
| **Transport** | Stdio pipe or stateful HTTP Server-Sent Events (`/mcp`) | Stateless HTTP REST (`/search`, `/scrape`) & SSE streams (`/search/stream`) |
| **Execution Paradigm** | Sequential multi-turn round-trips: `Model → Tool Call → Model → Tool Call` | Single-turn batching via `@utcp/code-mode` scripts (`Promise.all`) |
| **Intermediary Requirements** | Requires long-lived daemon process or stdio bridge | Zero daemons: direct HTTP calls to Searqon origin with full CORS |

### Code-Mode Single-Turn Batching vs. Sequential MCP Round-Trips

In typical multi-step research tasks (Search → Scrape URL 1 → Scrape URL 2 → Extract Tables), sequential MCP tool calling forces **4 to 8 iterative LLM inference round-trips**. Each turn incurs 2–5 seconds of inference latency and repeatedly burns context window tokens with serialized conversation turns.

With **Searqon UTCP + Code-Mode (`@utcp/code-mode`)**, the agent writes a concise JavaScript/TypeScript snippet that batches operations:
```typescript
const search = await searqon.searqon_search({ query: "generative AI in robotics", limit: 3 });
const pages = await Promise.all(search.results.map(r => searqon.searqon_scrape({ url: r.url })));
return { query: search.query, dossier: pages.map(p => ({ title: p.title, markdown: p.markdown })) };
```

**Measurable Benefits:**
- **Latency**: Cut by **60–80%** (1 LLM inference turn instead of 4–8 sequential turns).
- **Token Efficiency**: Prevents repeated prompt/schema inflation across intermediate chat messages.
- **Concurrency**: Parallelizes scraping directly over native HTTP pipelines.

Complete UTCP specification, tool schemas, and recipes: [**`docs/utcp.md`**](./docs/utcp.md)

---

## 📖 Documentation

Complete documentation is available in the [`docs/`](./docs/index.md) directory:

- [**MCP Integration Guide**](./MCP.md) — Claude Desktop, Cursor, Windsurf, and custom agents.
- [**UTCP & Code-Mode Guide**](./docs/utcp.md) — Dynamic discovery manual, tool schemas, and Code-Mode batching.
- [**Docker Hub Guide**](./docs/docker.md) — Docker setup, compose, and environment variables.
- [**Orchestration Workflow**](./docs/workflow/workflow.md) — High-level query pipeline, fallback chains, and lifecycle.
- [**Scraping Specification**](./docs/workflow/scraping.md) — Concurrency queues, robots.txt compliance, and Lightpanda.
- [**Stealth & Anti-Bot Escalation**](./docs/stealth_escalation.md) — 4-tier chained escalation (Fast HTTP → Spoofed Headers → Camoufox/Lightpanda → Residential Proxy).
- [**Architecture & Config**](./docs/architecture.md) — Database schema, variables, and error policies.
- [**API Reference**](./docs/api.md) — Request/Response structures for all endpoints.

---

## ⚡ Key Features

- **Native MCP Server (v1.2.1):** Run as `docker run -i ... mcp` for instant Claude Desktop / Cursor integration. Also exposes HTTP SSE at `/mcp` and `/mcp/sse`.
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
| GET    | `/utcp`          | UTCP discovery manual (dynamic tool catalog)            |
| GET    | `/utcp/manual`   | UTCP discovery manual alias                             |
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
