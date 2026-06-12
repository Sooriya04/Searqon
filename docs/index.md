# Searqon Documentation

Welcome to the Searqon documentation. Searqon is an open-source, self-hosted **web intelligence engine** that searches, crawls, extracts, and synthesizes information from the internet — built entirely in Go.

---

## Table of Contents

| Document | Description |
|---|---|
| [Installation](./installation.md) | Prerequisites, setup, and running locally |
| [Architecture](./architecture.md) | System design, data flow, and component breakdown |
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

## Five-Stage Pipeline

```
Query → Route → Search → Crawl → Extract → Deliver
```

1. **Route** — Classify query intent
2. **Search** — SearXNG (primary) or DuckDuckGo (fallback) returns URLs + snippets
3. **Crawl** — Go goroutines fetch the top pages concurrently
4. **Extract** — go-readability + goquery strip noise, return clean Markdown
5. **Deliver** — Structured JSON to your client, LLM, or agent
