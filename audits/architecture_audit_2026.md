# Searqon Comprehensive Architecture Audit & Feature Roadmap

**Date:** November 2026  
**Auditor:** Jessica Drew (Oracle Senior Systems Architect & Code Auditor)  
**Target Repository:** `Searqon` (Commit: `2e3ef7a`, synced with upstream main)  
**Assumed Load:** Production multi-tenant agent workload; concurrent autonomous agent swarms (Hermes, Claude Desktop, Cursor, Windsurf) executing high-throughput retrieval, deep multi-page research, and streaming tool calls.

---

## 1. Current State Engine Architecture

### 1.1 Core Binary & Runtime Lifecycle
- **Runtime:** Go 1.24+ zero-CGO binary (~8–12 MB compiled), deployed via standalone Docker container (`Dockerfile`) or direct binary.
- **Entrypoint (`src/main.go`):** Dual-mode execution:
  - CLI Stdio Mode (`searqon mcp`): Sets log output to `stderr` and executes JSON-RPC 2.0 stdio event loop (`src/mcp/server.go:StartStdioLoop`).
  - Server Mode: Initializes proxy pool (`scraper.InitProxyPool`), database subsystem (`db.InitDB`), structured logger (`utils.InitLogger`), and registers HTTP/SSE routes on `http.NewServeMux` wrapped by inbound rate limiting, gzip compression, and HTTP request logging middleware.

### 1.2 Storage & Caching Subsystem (`src/db/`)
- **Tri-Tier Caching Hierarchy:**
  1. **L1 Distributed Cache:** Redis (`src/db/redis.go`) with configurable TTLs (`search_ttl_hours: 24`, `scrape_ttl_days: 7`).
  2. **In-Memory Cache:** Global `sync.Map` (`searchMemoryCache`, `scrapeMemoryCache`) with background TTL eviction (`StartCacheCleanupWorker`).
  3. **L2 Persistent Storage:** SQLite (pure Go `modernc.org/sqlite` with WAL mode, `busy_timeout=5000`) or PostgreSQL (`github.com/jackc/pgx/v5/pgxpool` with max 15 conns).
- **Indexing Engines:**
  - Full-Text Search (FTS): SQLite FTS5 virtual table (`scrape_cache_fts`) and PostgreSQL `tsvector` with GIN indexing (`src/db/db_index.go`).
  - Semantic Vector Store: SQLite `page_embeddings` table and PostgreSQL `pgvector` with HNSW indexing (`src/db/db_vector.go`).

### 1.3 Search & Retrieval Pipelines (`src/search/`)
- **Multi-Engine Discovery (`src/search/discovery.go`):**
  - Concurrently queries SearXNG (JSON API), DuckDuckGo (HTML scraping fallback), Wikipedia (MediaWiki Action API), and Arxiv (Atom XML query API).
  - Provider fault tolerance: Per-engine circuit breakers (`engineCircuitBreaker`) tripping for 30s after 3 consecutive failures.
  - Multi-query decomposition: Splits queries by boolean operators (`and`, `or`, `|`, `,`, `;`) and extracted quotes (`splitQueryParts`).
  - Aggregation & Fusion: Reciprocal Rank Fusion (RRF, $k=60$) with academic weight boosting for Arxiv ($1.5\times$).
- **Ranking Engine (`src/search/ranker.go`, `src/search/semantic.go`):**
  - Lexical Scoring: TF-IDF term density, exact phrase matching, domain authority boosts (e.g. `arxiv.org`, `github.com`, `go.dev`, `news.ycombinator.com`, `.edu`, `.gov`), content word count quality curve, and publication freshness decay.
  - Semantic Reranking: Cosine similarity via Ollama embeddings (`nomic-embed-text`), blending lexical ($0.7$) and semantic ($0.3$) scores.

### 1.4 Scraper & Anti-Bot Escalation Ladder (`src/scraper/`)
- **4-Tier Escalation Pipeline (`src/scraper/escalation.go`):**
  - **Tier 1 (Fast HTTP):** Direct lightweight HTTP request (~50–150ms) using Go standard transport.
  - **Tier 2 (Anti-Bot Spoofed Headers):** Coherent browser personas (Chromium 128 on Win11/macOS, Safari 17.6, Firefox 129, Edge 128) with matching `Sec-CH-UA`, modern TLS 1.3 curve preferences (`X25519`, `P256`, `P384`), and search referer injection (`src/scraper/antibot.go`).
  - **Tier 3 (Headless Browser):** Dynamic client-side rendering via Lightpanda (custom zero-dependency WebSocket CDP client or CLI binary in `src/scraper/scraper_lightpanda.go`) or Camoufox stealth Firefox anti-detect browser (`src/scraper/scraper_camoufox.go`).
  - **Tier 4 (Residential / Rotating Proxy):** Traffic routing through proxy pools with round-robin failover (`src/scraper/proxy.go`).
- **Bot & Challenge Detection (`src/scraper/bot_detection.go`):** Deterministic signature scanner detecting Cloudflare Turnstile/Under Attack, DataDome, PerimeterX, Arkose Labs, Kasada, Imperva, CAPTCHAs, WAFs, and client-side SPA empty shells (`<app-root>`, `#root`, `#app`, `window.prerenderready=false`).
- **Content Cleaning & Chunker (`src/extractor/`, `src/chunker/`):**
  - Dual extraction: `go-readability` primary, noise-filtered `goquery` fallback.
  - Table-to-Markdown converter (`ConvertTablesInDocument`), outline/heading slug extractor, and clean paragraph-boundary truncation.
  - Overlapping sentence-boundary chunker (512 token target, 128 overlap) with BM25 scoring and SimHash 64-bit Hamming distance deduplication.

### 1.5 Protocol Interfaces
- **REST Endpoints (`src/handlers/`):** `/search`, `/scrape`, `/pipeline`, `/extract`, `/map`, `/crawl`, `/stream`, `/stats`, `/metrics`, `/health`, `/r/*`.
- **Model Context Protocol (`src/mcp/server.go`):** Exposes 5 agent tools (`searqon_search`, `searqon_scrape`, `searqon_research`, `searqon_map`, `searqon_extract`) over both Stdio JSON-RPC 2.0 and HTTP/SSE (`/mcp`, `/mcp/sse`).

---

## 2. Flaws, Bottlenecks & Technical Debt

### 2.1 Critical Bugs & Protocol Violations

#### Finding 1: Protocol Violation in MCP Tool Error Responses
- **Location:** `src/mcp/server.go:L287-302`
- **Mechanism:** When a tool fails or sets `toolResult.IsError = true`, the handler sets **both** `resp.Result = toolResult` and `resp.Error = &RPCError{Code: -32000, ...}`.
- **Impact:** Violates JSON-RPC 2.0 specification §5 (response MUST contain either `result` or `error`, never both) and the MCP specification. Strict JSON-RPC clients (Claude Desktop, Cursor, Hermes) fail to parse the dual payload or discard the structured diagnostic data.
- **Fix:** If `toolResult.IsError` is true, populate only `resp.Result = toolResult` with `resp.Error = nil`. Reserve `resp.Error` strictly for transport/framing JSON-RPC protocol errors (`-32700`, `-32600`, `-32601`, `-32602`).

#### Finding 2: Cache Layer Drops Cryptographic Citation Provenance
- **Location:** `src/db/db.go:L201-224, L253-276`, `src/db/db_scrape.go:L45-80, L173-222`
- **Mechanism:** Commit `6f61be0` added `ContentHash`, `ContentChanged`, and `ScrapedAt` to `models.ScrapeResult`. However, the SQLite and PostgreSQL table definitions (`scrape_cache`) lack `content_hash` and `scraped_at` columns. `SaveScrapeCache` drops both fields, and `GetScrapeCache` returns them as empty strings `""`.
- **Impact:** Across server restarts or on cache hits, citation verification hashes (`sha256`) and provenance timestamps are lost. `ContentChanged` comparison (`scraped.ContentHash != prevCached.ContentHash`) fails silently on subsequent scrapes.
- **Fix:** Add `content_hash TEXT` and `scraped_at DATETIME` to the `scrape_cache` DDL and map them in `SaveScrapeCache` and `GetScrapeCache`.

#### Finding 3: URL Normalization Erases Vital Query Parameters
- **Location:** `src/search/discovery.go:L185, L280-292`
- **Mechanism:** `normalizeDiscoveryURL(raw string)` unconditionally wipes `parsed.RawQuery = ""` and `entry.result.URL = normalizeDiscoveryURL(...)`.
- **Impact:** Destroys all query-parameter routed URLs. For example, all YouTube URLs become `https://www.youtube.com/watch`, Hacker News comments collapse into `https://news.ycombinator.com/item`, and search/blog paths lose their document IDs.
- **Fix:** Only strip tracking parameters (`utm_*`, `fbclid`, `gclid`, `ref_`), while preserving functional query parameters.

#### Finding 4: Crawler Link Discovery Fails on Cached Pages
- **Location:** `src/crawler/crawler.go:L161, L172, L206`
- **Mechanism:** In `CrawlSiteWithContext`, workers call `scraper.ScrapeSingleURLNative(task.url, format, false)`. On a cache hit, `htmlContent` is returned as `""`. Line 172 gates link parsing on `htmlContent != ""`.
- **Impact:** If the root URL or any child URL is already cached, zero links are discovered from it. Deep crawling terminates prematurely after reading only cached roots. Furthermore, line 206 overrides the incoming `ctx` with `context.WithTimeout(context.Background(), ...)`, ignoring caller cancellations.
- **Fix:** If `htmlContent == ""` on a cache hit, fall back to parsing outbound links from `scraped.OutboundLinks` or `scraped.Content`. Pass caller `ctx` into `context.WithTimeout(ctx, 60*time.Second)`.

#### Finding 5: `/extract` CSS Selectors Return Empty Strings on Cached Content
- **Location:** `src/handlers/extract.go:L65-98`, `src/mcp/server.go:L716-741`
- **Mechanism:** On cache hits, `ScrapeSingleURLWithOptions` returns `rawHTML == ""`. `goquery.NewDocumentFromReader(strings.NewReader(""))` returns a non-nil document with 0 elements and `err == nil`. Line 89 (`if err != nil || doc == nil`) is not taken, so `doc` is never populated from `res.Content`.
- **Impact:** Any `/extract` call or MCP `searqon_extract` targeting an already-cached URL returns empty selector matches `""` for all requested CSS selectors.
- **Fix:** Check `if rawHTML == ""` explicitly; parse `res.Content` or reconstruct the DOM tree from cached markdown/content.

---

### 2.2 Latency, Resource & Architectural Bottlenecks

#### Finding 6: Dead Code in PDF Extractor Causes Anti-Bot Escalation Storms
- **Location:** `src/scraper/pdf_extractor.go:L11-47`, `src/scraper/escalation.go:L120-142`
- **Mechanism:** `ExtractPDFText` is fully implemented using `github.com/ledongthuc/pdf`, but it is never referenced anywhere in the scraping or escalation pipeline. Outbound requests for `.pdf` URLs fetch binary bytes and feed them into HTML readability, extracting 0 words. The engine diagnoses a `js_gate`, escalates through Tiers 1–4, and aborts with HTTP 502 Bad Gateway.
- **Impact:** Complete failure to scrape academic papers (Arxiv), technical manuals, and whitepapers.
- **Fix:** Check `resp.Header.Get("Content-Type") == "application/pdf"` or URL path in `fetchHTTPRaw`; route directly to `ExtractPDFText` and bypass HTML/JS escalation.

#### Finding 7: Ollama Configuration Ignored & Redundant Implementations
- **Location:** `src/search/ollama.go:L24-36`, `src/scraper/ollama.go:L22-34`
- **Mechanism:** `settings.yml` specifies `ollama.enabled`, `ollama.url`, `ollama.model`, and `ollama.embedding_model`. Both `search/ollama.go` and `scraper/ollama.go` ignore `config.Get().Ollama` completely and fall back to hardcoded `http://localhost:11434` and `gemma:2b`. Both files duplicate HTTP client code and types.
- **Impact:** Configured Ollama hosts and models are bypassed. Vector embeddings run 11 serial HTTP calls per search query even if Ollama is disabled in config.
- **Fix:** Consolidate Ollama utilities into a single shared `src/utils/ollama.go` that reads `config.Get().Ollama` and checks `cfg.Ollama.Enabled` before dispatching requests.

#### Finding 8: Monolithic Execution in `/search/stream` SSE Endpoint
- **Location:** `src/handlers/search_stream.go:L33-54`
- **Mechanism:** `SearchStreamHandler` invokes `search.RunSearchPipeline` synchronously, waiting 5–15 seconds for all searches and scrapes to finish before writing any bytes to the client.
- **Impact:** Zero real streaming benefit. Clients experience high initial latency and receive all SSE frames simultaneously.
- **Fix:** Decouple `RunSearchPipeline` using a channel/callback interface: emit `discovery` immediately upon search provider RRF completion, emit `result` as each scraper goroutine finishes, and emit `done` after synthesis.

#### Finding 9: Inbound Rate Limiting Collapses Behind Reverse Proxies
- **Location:** `src/utils/middleware.go:L66-73`
- **Mechanism:** `r.RemoteAddr` is used directly without checking `X-Forwarded-For` or `X-Real-IP`.
- **Impact:** In standard Docker Compose, Kubernetes, or reverse-proxied production environments, all clients share the reverse proxy bridge IP (e.g., `172.18.0.1`). All agents exhaust the global 2 req/sec bucket and trigger false 429 HTTP errors.
- **Fix:** Read the leftmost IP in `X-Forwarded-For` or `X-Real-IP` when present.

#### Finding 10: PostgreSQL Vector Dimension Hardcoded to 768
- **Location:** `src/db/db_vector.go:L58`
- **Mechanism:** Column defined as `VECTOR(768) NOT NULL`.
- **Impact:** Precludes common embedding models such as `text-embedding-3-small` (1536 dim) or `bge-small` (384 dim). Any dimension mismatch causes fatal SQL insert errors.
- **Fix:** Parameterize vector dimension in `settings.yml` or default dynamically on first insert.

---

## 3. High-Impact Roadmap: Top Capabilities for AI Agent Workflows

### Capability 1: True Incremental Token-Streaming Search & Research
- **Objective:** Eliminate agent tool timeouts (15–30s) by converting `/search/stream` and MCP `searqon_research` into real-time incremental feeds.
- **Architecture:**
  - Introduce `SearchStreamObserver` channel interface in `src/search/search.go`.
  - **Phase 1 (~200ms):** Flush `discovery` event containing all resolved search URLs and snippets. Agents begin reasoning or filtering targets immediately.
  - **Phase 2 (~500–1200ms):** Concurrently scrape top results and flush individual `source_chunk` events with cryptographic hashes as each goroutine completes.
  - **Phase 3:** Stream synthesis tokens incrementally into the SSE connection.
  - **MCP Tool Integration:** Emit MCP `notifications/progress` in `searqon_research` to keep client agent turn timers active.

### Capability 2: Native PDF & Multi-Document Format Ingestion Pipeline
- **Objective:** Seamless, first-class scraping of whitepapers, Arxiv PDFs, SEC filings, and scientific literature.
- **Architecture:**
  - Intercept `application/pdf` in `src/scraper/escalation.go` and invoke `ExtractPDFText`.
  - Structure extracted PDF text with standardized page delimiters (`<!-- page: N -->`), heading hierarchies, and table preservation.
  - Generate full citation hashes (`sha256`), word count metrics, and token-bounded chunking for PDFs.
  - Add auto-resolution of academic links: convert `arxiv.org/abs/XXXX.XXXXX` directly into PDF stream extraction when deep text is requested.

### Capability 3: Autonomous Sub-Query Planning & Multi-Perspective Research Dossier
- **Objective:** Transform `searqon_research` from a simple link accumulator into an autonomous deep-research synthesis engine.
- **Architecture:**
  - Implement recursive query planning: when an agent submits a multifaceted prompt (e.g. *"Compare Raft vs Paxos leader election under network partitions"*), Searqon generates 3–4 orthogonal sub-queries targeting distinct facets.
  - Fan out sub-queries across providers with global deduplication.
  - Cross-source citation grounding: compile results into a structured dossier with inline citations (`[^1]`), source confidence ratings, and extraction telemetry.

### Capability 4: Persistent Session Vault & Authenticated Anti-Bot Tunneling
- **Objective:** Enable agents to access paywalled, authenticated, or gated research sources (Substack, IEEE, GitHub, Jira) across long-running sessions.
- **Architecture:**
  - Connect `PersistentCookieJar` (`src/utils/cookiejar.go`) to `fetchHTTPRaw` in `src/scraper/escalation.go`.
  - Expose authenticated session management endpoints (`/session/cookies`) and MCP tool parameters allowing agents to supply domain auth cookies.
  - Automatically persist session cookies across escalation tiers: when Lightpanda or Camoufox executes JS and receives session cookies, capture and store them in the `sessions` DB table for subsequent lightweight Tier 1 requests.

---

## 4. Summary & Recommended Action Plan

| Priority | Item | Component | Impact | Estimated Diff |
| :--- | :--- | :--- | :--- | :--- |
| **P0 (Must Fix)** | Fix dual Result/Error JSON-RPC violation | `src/mcp/server.go` | Fixes MCP client parsing failures | -6 lines, +4 lines |
| **P0 (Must Fix)** | Persist citation hashes & timestamps in DB | `src/db/` | Fixes broken citation verification | +12 lines DDL & Scan |
| **P0 (Must Fix)** | Fix URL query stripping in discovery | `src/search/discovery.go` | Prevents URL corruption | +15 lines |
| **P0 (Must Fix)** | Fix `/extract` and crawler cache hits | `src/handlers/`, `src/crawler/` | Restores cache extraction | +10 lines |
| **P1 (Should Fix)**| Connect PDF extractor to scraping pipeline | `src/scraper/` | Enables PDF & Arxiv paper ingestion | +20 lines |
| **P1 (Should Fix)**| Consolidate Ollama config & clients | `src/search/`, `src/scraper/` | Eliminates dead code & respects config | -60 lines, +25 lines |
| **P1 (Should Fix)**| True SSE streaming in `/search/stream` | `src/handlers/search_stream.go` | Drastically reduces agent time-to-first-token | +35 lines |
| **P2 (Roadmap)**   | Autonomous Sub-Query Planning Engine | `src/search/`, `src/mcp/` | Multi-perspective research dossiers | Roadmap Feature |
| **P2 (Roadmap)**   | Persistent Session Vault & Cookie Tunnel | `src/scraper/`, `src/db/` | Authenticated scraping across agent runs | Roadmap Feature |
