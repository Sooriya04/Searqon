# Searqon — Web Intelligence Engine

Searqon is an open-source, self-hosted web intelligence engine that **searches, crawls, extracts, ranks, and synthesizes** information from the internet. Built in Go for maximum performance and minimum overhead.

> **Searqon is not a chatbot.** It is the foundational search and web intelligence layer designed to power AI agents, RAG systems, and knowledge-driven applications.

Think of it as a transparent alternative to services like Tavily — where you control the entire pipeline, from how sources are selected to how content is extracted and cleaned.

---

## Architecture

Searqon processes web searches through five core stages:

```
Query
  │
  ├─ 1. ROUTE      → Classify query intent (tech/medical/news/general...)
  │
  ├─ 2. SEARCH     → SearXNG (primary) → DDG JSON API (fallback)
  │                  Returns: title, URL, snippet
  │
  ├─ 3. CRAWL      → Concurrent goroutines fetch each result page
  │                  Respects robots.txt
  │
  ├─ 4. EXTRACT    → go-readability + goquery strip noise,
  │                  output clean plain text + Markdown
  │
  └─ 5. DELIVER    → Structured JSON to client (LLM, RAG, agent, etc.)
                     Snippet fallback if page is blocked by Cloudflare
```

**Single Go process. Port 3001. ~20MB idle RAM.**

---

## Endpoints

| Method | Endpoint       | Description                                                        |
|--------|----------------|--------------------------------------------------------------------|
| POST   | /search        | Full pipeline: search → scrape → structured JSON                  |
| GET    | /search?q=...  | Same via query string                                              |
| POST   | /scrape        | Scrape a single URL → markdown/text                               |
| POST   | /scrape/batch  | Scrape multiple URLs concurrently                                 |
| POST   | /scrape/html   | Parse raw HTML → clean markdown                                   |
| POST   | /crawl         | Crawl an entire site (depth + page limit)                         |
| POST   | /map           | Discover all URLs on a site                                       |
| GET    | /health        | Health check                                                      |

---

## Search Provider Chain

1. **SearXNG** (primary) — Run locally via Docker. Aggregates Google, Bing, Qwant, DuckDuckGo without getting blocked. Returns clean JSON. Zero latency overhead.
2. **DuckDuckGo JSON API** (fallback) — Uses DDG's internal `d.js` endpoint (not HTML scraping). Automatically used if SearXNG is offline.

---

## Run

### Option 1: Start without SearXNG (DDG fallback mode)
```bash
cd go_scraper
go run .
```

### Option 2: Start with SearXNG (recommended)
```bash
# Terminal 1: Start SearXNG
docker run -d --name searxng -p 8080:8080 searxng/searxng

# Terminal 2: Start Searqon
cd go_scraper
go run .
```

### Build binary
```bash
cd go_scraper
go build -o ../searqon .
./searqon
```

---

## Usage Examples

```bash
# Full search + scrape pipeline (uses SearXNG if running, DDG otherwise)
curl -X POST http://localhost:4001/search \
  -H "Content-Type: application/json" \
  -d '{"query": "what is ycombinator", "limit": 5, "scrape": true}'

# Search only — no page scraping, just titles/URLs/snippets
curl -X POST http://localhost:4001/search \
  -H "Content-Type: application/json" \
  -d '{"query": "anthropic claude 3.5", "limit": 5, "scrape": false}'

# Scrape a specific page
curl -X POST http://localhost:4001/scrape \
  -H "Content-Type: application/json" \
  -d '{"url": "https://anthropic.com/news/claude-3-5-sonnet"}'

# Crawl an entire documentation site
curl -X POST http://localhost:4001/crawl \
  -H "Content-Type: application/json" \
  -d '{"url": "https://docs.anthropic.com", "limit": 30, "depth": 2}'
```

---

## Project Philosophy

> Build the system first. Add intelligence later.

Searqon prioritizes **transparency, modularity, and control**. Every decision in the pipeline is visible and customizable — no black boxes, no magic.

---

## Roadmap

### Phase 1 — Core Engine ✅
- High-performance Go-based parallel scraping
- SearXNG + DDG JSON fallback provider chain
- Robots.txt compliance
- Snippet fallback when pages are blocked

### Phase 2 — Intelligent Orchestration (In Progress)
- Native query intent classifier (< 1ms, no LLM)
- BM25 + semantic hybrid reranking
- Structured knowledge panel extraction
- Agent planning interface

---

## License

MIT License. See LICENSE for details.
