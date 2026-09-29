# Searqon — Autonomous Web Intelligence & RAG Retrieval Engine

Searqon is an open-source, self-hosted web intelligence and search engine written in Go. Designed as a transparent, high-performance alternative to services like Tavily and Firecrawl, Searqon searches, crawls, bypasses anti-bot defenses, extracts clean Markdown, and indexes content for AI agents and RAG (Retrieval-Augmented Generation) pipelines.

---

## ⚡ Highlights

* **Ultra-Lightweight Scratch Container**: Built on a `scratch` base with a 0 MB OS footprint and only **~8.28 MB** total compressed image size.
* **4-Tier Stealth Anti-Bot Escalation**: Automatically ladders from fast HTTP requests to browser TLS fingerprint spoofing (Chrome 128, Safari 17, Edge 128), headless browser execution, and residential proxy fallbacks to clear Cloudflare Turnstile, DataDome, and PerimeterX challenges.
* **Embedded SQLite or Scalable PostgreSQL**:
  * **SQLite Mode (Default)**: Zero-dependency storage using pure-Go SQLite with WAL mode, FTS5 full-text search indexing, and vector embedding tables (`./data/searqon.db`).
  * **PostgreSQL Mode**: Seamless connection pooling via `pgxpool` with automated schema migrations.
* **Clean Markdown & Content Sanitization**: Converts raw HTML into noise-free, token-optimized Markdown stripped of navigation menus, headers, scripts, and cookie banners.
* **Federated Meta-Search**: Aggregates and reranks results across SearXNG, DuckDuckGo, Wikipedia, ArXiv, and academic providers.
* **Integrated SQLite Audit Logging**: Structured JSON request and error telemetry stored in `./data/logs.db` with an inspection endpoint (`GET /logs`).

---

## 🚀 Quick Start (Standalone Container)

Run Searqon standalone using its built-in SQLite engine on port **`7493`**:

```bash
docker run -d \
  --name searqon \
  -p 7493:7493 \
  -v $(pwd)/data:/app/data \
  sooriya04/searqon:latest
```

Verify that the engine is online:
```bash
curl http://localhost:7493/health
```

---

## 🐳 Full Stack Deployment (Docker Compose)

To run Searqon with meta-search aggregation (**SearXNG**), PostgreSQL, and Redis:

```yaml
services:
  searqon:
    image: sooriya04/searqon:latest
    ports:
      - "7493:7493"
    environment:
      - PORT=7493
      - SEARXNG_URL=http://searxng:8080
      - DB_TYPE=sqlite # Set to "postgres" to switch to PostgreSQL
      - DATABASE_URL=postgres://searqon:searqon_pass@searqon-db:5432/searqon_cache?sslmode=disable
      - REDIS_HOST=searqon-redis
      - REDIS_PORT=6379
    volumes:
      - ./data:/app/data
    depends_on:
      - searxng
    restart: unless-stopped

  searxng:
    image: searxng/searxng:latest
    ports:
      - "8080:8080"
    dns:
      - 8.8.8.8
      - 1.1.1.1
    restart: unless-stopped

  searqon-db:
    image: postgres:15-alpine
    ports:
      - "5435:5432"
    environment:
      - POSTGRES_USER=searqon
      - POSTGRES_PASSWORD=searqon_pass
      - POSTGRES_DB=searqon_cache
    volumes:
      - searqon-db-data:/var/lib/postgresql/data
    restart: unless-stopped

  searqon-redis:
    image: redis:alpine
    ports:
      - "6379:6379"
    restart: unless-stopped

volumes:
  searqon-db-data:
```

---

## ⚙️ Configuration Reference

| Variable | Default | Purpose |
| :--- | :--- | :--- |
| `PORT` | `7493` | HTTP API listening port |
| `DB_TYPE` | `sqlite` | Primary storage engine (`sqlite` or `postgres`) |
| `SQLITE_PATH` | `./data/searqon.db` | Local SQLite cache & FTS5 database path |
| `DATABASE_URL` | *(Postgres DSN)* | PostgreSQL connection URI when using Postgres mode |
| `SEARXNG_URL` | `http://searxng:8080` | Endpoint of the SearXNG meta-search provider |
| `REDIS_HOST` | `searqon-redis` | Hostname for Redis L1 cache |
| `OLLAMA_HOST` | `http://host.docker.internal:11434` | Endpoint for local Ollama LLM / embedding models |
| `ROTATING_PROXIES` | `""` | Comma-separated HTTP/HTTPS proxies for Tier 4 escalation |
| `RESIDENTIAL_PROXY_URL` | `""` | Residential proxy gateway URL |

---

## 📡 API Endpoints

* **`POST /search`** — Meta-search aggregation, reranking, and live page scraping into structured JSON.
* **`GET /search?q=query&limit=5`** — Fast query-string search.
* **`POST /scrape`** — 4-tier anti-bot scraping for a single URL returning clean Markdown.
* **`POST /scrape/chunked`** — Scrapes and partitions text into RAG-ready semantic chunks with token counts.
* **`POST /scrape/batch`** — High-concurrency parallel scraping across multiple links.
* **`POST /map`** — Discovers all internal URLs on a target domain.
* **`POST /crawl`** — Recursive site crawler with depth limits and real-time SSE streaming.
* **`GET /health`** — Service status and active pipeline capabilities.
* **`GET /logs`** — Queryable SQLite system and audit logs.

---

## 📄 Source & Documentation

* **GitHub Repository**: [https://github.com/Sooriya04/Searqon](https://github.com/Sooriya04/Searqon)
* **Documentation**: [https://sooriya04.github.io/Searqon/](https://sooriya04.github.io/Searqon/)
* **License**: MIT
