# Searqon Documentation

Welcome to the Searqon documentation. Searqon is an open-source, self-hosted **web intelligence engine** that searches, crawls, extracts, and synthesizes information from the internet — built entirely in Go.

---

## Table of Contents

| Document | Description |
|---|---|
| [Installation](./installation.md) | Prerequisites, database setup, and running locally |
| [Architecture](./architecture.md) | System design, database schemas, and configuration reference |
| [Orchestration Workflow](./workflow/workflow.md) | High-level request lifecycle, discovery chains, and fallbacks |
| [Scraping Pipeline](./workflow/scraping.md) | Parallel crawling, robots.txt, and Lightpanda integration |
| [API Reference](./api.md) | All endpoints with request/response examples |
| [Search Providers](./providers.md) | SearXNG vs DuckDuckGo — when to use what |
| [Configuration](./configuration.md) | Ports, timeouts, limits, and tuning |
| [Deployment](./deployment.md) | Docker, Docker Compose, and production setup |

---

## Quick Start

```bash
# Clone and run
git clone https://github.com/Sooriya04/Searqon
cd searqon/src
go run .

# Test it
curl -X POST http://localhost:4001/search \
  -H "Content-Type: application/json" \
  -d '{"query": "what is ycombinator", "limit": 5, "scrape": true}'
```

---

## What Searqon Is

Searqon is **not a chatbot**. It is the foundational search and web intelligence layer designed to power:

- AI agents and RAG (Retrieval-Augmented Generation) systems
- Knowledge-driven applications
- Self-hosted Perplexity-style search pipelines

Think of it as a transparent alternative to services like **Tavily** — where you control every stage of the pipeline.

---

## Technical Pipeline Overview

```
Query → Cache Check → Search Providers → Concurrency Queue → Crawl (Go or Lightpanda) → Extract → Cache Save → Deliver
```

1. **Cache Check** — PostgreSQL query cache (24h TTL) returns hits in <5ms.
2. **Search Providers** — SearXNG (primary) or DuckDuckGo Lite (fallback) returns search matches.
3. **Crawl Queue** — Enforces 3 concurrent worker limits and global timeouts.
4. **Scraping** — HTML parsed natively or rendered dynamically using the **Lightpanda** browser.
5. **Purifier** — Mozilla Readability strips noise and converts to clean Markdown.
6. **Persistence & Return** — Saves results in the cache database and returns structured JSON to the client.
