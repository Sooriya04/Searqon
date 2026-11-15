# Searqon Codebase Audit Report

**Date:** 2026-11-13  
**Auditor:** Barbara Vane (Barb)  
**Project:** Searqon (`Searqon/src`)  
**Scope:** Full repository audit (HTTP middleware, Scraper pipeline, Crawler, Stream Markdown converter, MCP server, Search engine, Database/SQLite connection pooling)  
**Assumed Load:** Multi-user production deployment serving simultaneous AI agent research tasks, concurrent recursive site crawling (dozens of pages at depth 2–3), and high-throughput anti-bot scraping over HTTP and SSE streams.

---

## Executive Summary

Searqon is a self-hosted web intelligence and search engine built in Go. While core unit tests pass cleanly and the architecture is thoughtfully partitioned into modular packages, an in-depth code audit revealed critical production vulnerabilities:
1. **Security:** Server-Side Request Forgery (SSRF) bypasses via unchecked HTTP redirects and unvalidated robots.txt fetches.
2. **Concurrency:** A race condition between workers and coordinator causing site crawls to abort prematurely after only 1 page, and a data race on search results during scrape timeouts.
3. **Resource Leakage:** An unbounded in-memory map storing multi-megabyte raw HTML strings with zero TTL or eviction.
4. **Data Integrity:** Streaming Markdown converter dropping all hyperlinks inside HTML table cells, and slicing UTF-8 strings at byte boundaries corrupting runes in MCP responses.

---

## Implementation Execution Strategy (Batched Fix & Push Protocol)

To maintain a clean git history, keep diffs minimal and verified, and eliminate risk of regression, **do not attempt to fix all issues in a monolithic commit**. 

Execute the fixes in strictly ordered batches: **2 bugs at a time with tests and pushes, ending with the final 3 bugs**:

* **Batch 1 (Critical Security & Crawler Race):** Fix **Bug 1** & **Bug 2** -> Run `go test ./...` -> Commit & push.
* **Batch 2 (Memory Leak & Search Data Race):** Fix **Bug 3** & **Bug 4** -> Run `go test ./...` -> Commit & push.
* **Batch 3 (Data Loss & Byte Slicing):** Fix **Bug 5** & **Bug 6** -> Run `go test ./...` -> Commit & push.
* **Batch 4 (Rate Limit Bypass & Cookie Contention):** Fix **Bug 7** & **Bug 8** -> Run `go test ./...` -> Commit & push.
* **Batch 5 (SQLite Lock & Relative Path Resolution):** Fix **Bug 9** & **Bug 10** -> Run `go test ./...` -> Commit & push.
* **Batch 6 (Last 3 Bugs: Tag Attributes, Gzip Header, MCP Protocol):** Fix **Bug 11**, **Bug 12** & **Bug 13** -> Run full test suite & race detector (`go test -race ./...`) -> Final commit & push.

---

## Detailed Audit Findings

### Must Fix

#### 1. SSRF Bypass via HTTP Redirects and Unvalidated robots.txt Fetches
- **Location:** `src/scraper/scraper_http.go:L48-53`, `src/scraper/escalation.go:L58-63`, `src/scraper/robots.go:L120-125`, `src/crawler/crawler.go:L43`
- **What this is:** `ValidateScrapeURL` blocks private/loopback/cloud metadata IP ranges (`169.254.169.254`, `127.0.0.1`, `10.0.0.0/8`, etc.) before scraping.
- **Problem:** When an external target returns an HTTP 301/302 redirect to an internal IP or metadata endpoint, `httpClient.CheckRedirect` follows it unconditionally without re-running SSRF validation. Furthermore, `fetchRobotsData` and `/crawl` / `/map` fetch `robots.txt` directly without any SSRF checks.
- **Fix:** In `httpClient.CheckRedirect` and `fetchHTTPRaw`, call `ValidateScrapeURL(req.URL.String())` before following any redirect, and validate URLs inside `fetchRobotsData`.
- **If skipped:** Any external site or agent query can pivot Searqon into an internal network scanner to steal AWS/GCP IAM credentials or attack unauthenticated intranet services.

#### 2. Crawler Terminates Prematurely Due to Worker-Coordinator Race Condition
- **Location:** `src/crawler/crawler.go:L216-224`, `src/crawler/crawler.go:L233-259`
- **What this is:** `CrawlSiteWithContext` spawns 5 parallel workers to crawl a domain up to a requested page limit.
- **Problem:** When a worker finishes task 0, it pushes discovered links to `discoveredLinksChan` and immediately decrements `pendingTasks` to 0. If the coordinator loop checks `for atomic.LoadInt32(&pendingTasks) > 0` before reading `discoveredLinksChan`, the loop exits immediately and cancels all remaining links after just 1 page.
- **Fix:** Keep `pendingTasks` updated before pushing links or use a `sync.WaitGroup` to track inflight tasks rather than racing atomic counters against channel sends.
- **If skipped:** Multi-page recursive site crawls randomly abort after scraping only the single root page.

#### 3. Unbounded Raw HTML In-Memory Cache Causes Progressive OOM Leak
- **Location:** `src/scraper/scraper.go:L32`, `src/scraper/scraper.go:L416`
- **What this is:** `rawHTMLCache` is a global `sync.Map` intended to cache raw HTML strings for scraped URLs.
- **Problem:** Every scraped page stores up to 5–8MB of raw HTML strings into `rawHTMLCache` without any TTL, size ceiling, eviction, or deletion logic anywhere in the repository.
- **Fix:** Delete `rawHTMLCache` and rely on SQLite/Redis (which already have automated expiration workers), or wrap it in a fixed-size LRU cache.
- **If skipped:** Any sustained crawling or batch scraping workload will leak memory until the process is OOM-killed by the kernel.

#### 4. Concurrent Data Race on Search Results During Scrape Timeout
- **Location:** `src/search/search.go:L414-428`, `src/search/search.go:L467-482`
- **What this is:** `RunSearchPipelineStream` runs concurrent scrapers on discovered search results with a 15-second timeout.
- **Problem:** If the 15-second timeout fires, the `select` block breaks and the main routine immediately mutates and slices `results` via `DeduplicateResults` and `HybridRankResults`, while late background scraper goroutines are still actively writing to `results[idx]` without holding a shared mutex.
- **Fix:** Clone the current results slice under `mu.Lock()` before deduplication/ranking, or cleanly cancel workers and wait on `wg.Wait()` before proceeding.
- **If skipped:** High-latency queries under load trigger Go runtime data races and memory corruption on search result slices.

#### 5. Streaming Markdown Converter Drops Hyperlinks Inside Table Cells
- **Location:** `src/scraper/stream_markdown.go:L479-487`, `src/scraper/stream_markdown.go:L421-429`
- **What this is:** The streaming HTML tokenizer converts table cells and markdown links into clean text.
- **Problem:** When processing `<td><a href="...">text</a></td>`, `handleText` matches `sm.inCell` first and writes text straight into `currentCell`, leaving `sm.linkText` empty. When `</a>` closes, `sm.linkText` is empty, causing the entire link and URL to be discarded.
- **Fix:** In `handleText`, check `if sm.inLink` before `if sm.inCell` so anchor text is captured and rendered as `[text](url)` inside the cell buffer.
- **If skipped:** Scraped HTML tables (pricing sheets, documentation index tables, benchmarks) lose all hyperlinks and URLs.

#### 6. Multi-Byte UTF-8 String Truncation Corrupts Characters in MCP Tool Responses
- **Location:** `src/mcp/server.go:L559`, `src/mcp/server.go:L701`
- **What this is:** `searqon_scrape` and `searqon_research` truncate long output to `maxTokens * 4` characters to protect LLM context windows.
- **Problem:** The code slices Go strings at raw byte offsets (`content[:charLimit]`). If `charLimit` lands in the middle of a multi-byte UTF-8 rune (emojis, curly quotes, dashes, or non-Latin scripts), it produces malformed UTF-8 byte sequences.
- **Fix:** Reuse `truncateUTF8Safe` from `stream_markdown.go` or step backwards to valid rune boundaries before slicing.
- **If skipped:** LLMs receive corrupted replacement characters (`\ufffd`) or fail with JSON-RPC unmarshaling errors.

---

### Should Fix

#### 7. Rate Limiting is Bypassed via Spoofed X-Forwarded-For Headers
- **Location:** `src/utils/middleware.go:L66-74`
- **What this is:** `InboundRateLimitMiddleware` limits client API requests to 2 req/sec with a burst of 30.
- **Problem:** `GetClientIP` blindly trusts the first IP in `X-Forwarded-For` without verifying whether `r.RemoteAddr` is a trusted proxy. Anyone sending a randomized `X-Forwarded-For: <random-ip>` gets an isolated rate-limiter bucket on every request.
- **Fix:** Default to `r.RemoteAddr`, and only read `X-Forwarded-For` if `r.RemoteAddr` matches a configured trusted proxy subnet.
- **If skipped:** Attackers can bypass API rate limits completely with a single header line.

#### 8. Synchronous Cookie Jar Database Writes Block All Concurrent Scrapers
- **Location:** `src/utils/cookiejar.go:L40-58`
- **What this is:** `PersistentCookieJar` persists session cookies to SQLite/PostgreSQL to maintain authenticated sessions across scrapes.
- **Problem:** `SetCookies` acquires a global mutex `p.mu` and executes a synchronous database write (`db.SaveSessionCookies`) on every HTTP response with cookies, blocking any other goroutine calling `Cookies(u)`.
- **Fix:** Dispatch `db.SaveSessionCookies` into a buffered background channel or worker pool, releasing `p.mu` immediately.
- **If skipped:** High-concurrency scraping batches bottleneck behind database disk I/O.

#### 9. SQLite Connection Pool Lacks MaxOpenConns Limit, Risky Lock Contention
- **Location:** `src/db/db.go:L106-110`
- **What this is:** Searqon initializes a local SQLite database for search, scrape, and session caching.
- **Problem:** `sql.Open("sqlite", dbPath)` does not call `db.SetMaxOpenConns(1)`. Go's `database/sql` defaults to unlimited connections, causing concurrent write transactions to fail with `database is locked (5)` under load.
- **Fix:** Add `sqliteDB.SetMaxOpenConns(1)` right after opening the SQLite handle.
- **If skipped:** Parallel scraping workers will intermittently fail to cache pages when write collisions occur.

#### 10. Relative Link Resolution in Crawler Resolves Against Root Instead of Parent Page
- **Location:** `src/crawler/crawler.go:L177`
- **What this is:** The recursive crawler parses `a[href]` links on crawled subpages to discover nested pages.
- **Problem:** Relative URLs on deep pages are resolved against `parsedBase` (the original seed URL) rather than `task.url` (the actual page containing the link).
- **Fix:** Parse and resolve relative links against `task.url`.
- **If skipped:** Crawling subdirectories fails to discover child pages or generates broken paths (e.g., resolving `sub/page.html` against root instead of the current directory).

#### 11. Single-Pass HTML Tokenizer Attribute Extraction Drops `alt` Attributes
- **Location:** `src/scraper/stream_markdown.go:L326-332`, `src/scraper/stream_markdown.go:L549-563`
- **What this is:** The streaming markdown generator reads `src` and `alt` attributes on `<img>` tags.
- **Problem:** `extractAttr` consumes the streaming tokenizer's attributes in a single forward pass. If `alt` appears before `src` in the HTML (e.g. `<img alt="..." src="...">`), finding `src` advances past `alt`, causing the second lookup for `alt` to return empty.
- **Fix:** Read all attributes into a map or slice in one pass during tag processing before querying keys.
- **If skipped:** Any image where `alt` is written before `src` loses its alt text and renders as `![image](...)`.

---

### Nice to Have

#### 12. Gzip Middleware Sets Content-Encoding Before Knowing If Response Has a Body
- **Location:** `src/utils/middleware.go:L148-150`
- **What this is:** `GzipCompressionMiddleware` compresses HTTP response bodies when `Accept-Encoding: gzip` is present.
- **Problem:** It writes `Content-Encoding: gzip` into response headers immediately before downstream handlers run. If a handler returns status 204 or 304, the gzip footer is still written, creating an invalid HTTP response.
- **Fix:** Set `Content-Encoding: gzip` lazily inside `gzw.Write` only when bytes are actually written.
- **If skipped:** Empty or cached responses (304 Not Modified) send invalid gzip headers and corrupted empty bodies.

#### 13. MCP JSON-RPC Returns Error Replies for Client Notification Messages
- **Location:** `src/mcp/server.go:L352-354`
- **What this is:** The MCP server handles JSON-RPC requests and notifications from client IDEs and LLM harnesses.
- **Problem:** Unhandled notifications (e.g. `notifications/cancelled`) hit the `default` switch case and return a JSON-RPC error response, violating JSON-RPC 2.0 specifications.
- **Fix:** If `req.ID == nil` (notification), return `nil, nil` without producing any JSON-RPC response.
- **If skipped:** Strict MCP clients like Claude Desktop may log protocol desynchronization warnings.

---

## Verdict & Recommendation

- **Verdict:** Follow the batched execution plan starting with **Batch 1 (SSRF redirect bypass + crawler premature exit race)** and **Batch 2 (rawHTMLCache OOM leak + search data race)**.
- **Lean Potential:** -40 lines possible by dropping `rawHTMLCache` and pruning redundant middleware checks.
- **Not checked:** External live browser daemons (Camoufox/Lightpanda headless runtime processes).
