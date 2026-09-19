# Searqon Web Intelligence Engine: Scraping Pipeline

This document details the mechanics of Searqon's high-concurrency web scraping system. The scraper parses and cleans web page contents in parallel to provide structured markdown datasets for upstream AI models.

---

## Technical Scraping Architecture

The scraping process is governed by a strict concurrency limit and global timeout to prevent slow web servers from hanging the request execution.

![Web Scraping Pipeline Flow](../images/web_scraping_flow.png)

---

## Pipeline Execution Stages

The scraping pipeline operates sequentially across these distinct stages:

### Stage 1: URL Deduplication
Before initiating any processing or network requests, the engine normalizes the discovered search URLs (e.g., stripping trailing slashes) and deduplicates them. This ensures that duplicate links returned across different search engines are discarded **before** any database lookups or robots.txt checks occur.

### Stage 2: Cache Check (PostgreSQL)
For each unique URL, the database cache is queried:
* **Bypass Cache**: If the request specifies `bypass_cache: true`, the cache is completely ignored.
* **TTL Age Check**: If a cached entry is found in `scrape_cache`, its age is validated against the TTL policy (`SCRAPE_CACHE_TTL_DAYS`).
* **Cache Hit / Miss**: Valid entries return immediately in under 5ms. Misses continue to the concurrency queue.

### Stage 3: Concurrency & Deadline Controls
To prevent resource exhaustion and request hangs:
* **Goroutine Cap**: A maximum of 3 concurrent page fetches (`scrapeLimit`) are dispatched.
* **8-Second Timeout**: The group is governed by a global `context.WithTimeout` set to 8000ms.
* **Mutex Locks**: Writes to the shared results array are synchronized using `sync.Mutex` to prevent race conditions as pages finish loading asynchronously.

### Stage 4: robots.txt Parser & Compliance
For each URL dispatched, the scraper first checks compliance with the host's crawling policy:
1. **Fetch & Cache**: Fetches the host's `robots.txt` and caches the rule payload (`robotsCache`) to avoid repetitive overhead.
2. **Stealth Agent Matching & Rotation**: Searches for matching directives. If general crawlers (`*`) are allowed, the scraper passes to the escalation pipeline.
3. **Crawl Delay Compliance**: Respects optional `Crawl-delay` timers via `time.Sleep`. If disallowed, execution stops immediately and records "disallowed by robots.txt" status.

### Stage 5: Smart Stealth & Anti-Bot Escalation Ladder
When scraping content, Searqon executes a progressive 4-tier escalation ladder designed for maximum speed and stealth:

* **Tier 1 — Fast HTTP (~50-150ms):** Direct, unproxied native Go HTTP request using lightweight standard headers. If the page returns 200 OK without bot challenge signatures, content is extracted immediately with zero overhead.
* **Tier 2 — Anti-Bot Spoofed Headers:** If blocked (HTTP 403, 429, 503, or anti-bot challenge detected), the engine escalates to high-stealth browser personas matching Chrome 128 (Win11/macOS), Safari 17.6, or Firefox 129. Sets matching Client Hints (`Sec-CH-UA`, `Sec-CH-UA-Mobile`, `Sec-CH-UA-Platform`), modern TLS 1.3 ALPN ciphers, and search engine referer spoofing.
* **Tier 3 — Headless Browser (Camoufox / Lightpanda):** If JS execution is required or Tier 2 is blocked by Cloudflare Turnstile / DataDome:
  * **Camoufox**: Open-source anti-detect browser based on Firefox with C++ anti-fingerprinting.
  * **Lightpanda**: Ultra-fast headless browser subprocess executing dynamic JS.
* **Tier 4 — Residential / Rotating Proxy Fallback:** If IP bans, rate limits, or geo-blocks persist, the request routes through the configured proxy pool (`RESIDENTIAL_PROXY_URL`, `RESIDENTIAL_PROXIES`, `ROTATING_PROXIES`, or `proxies.txt`) with spoofed headers or headless browser.

### Stage 6: DOM Purging, Readability & Markdown
Once HTML is retrieved from the winning tier:
1. **Element Stripping**: Drops scripts, styles, iframes, SVGs, and noise templates.
2. **Readability Extraction**: Extracts the main content body using Mozilla's Readability algorithm.
3. **Markdown Conversion**: Converts the clean content block to structured markdown.

### Stage 7: Snippet Fallback on Failure
If all scraping escalation tiers fail (e.g., connection reset, DNS failure, or hard CAPTCHA block), the engine falls back to the search engine snippet returned during discovery.

### Stage 8: Persistent Write
Successfully scraped content, escalation tier, render method, content hash, and error states are written back to the PostgreSQL database (and in-memory cache) for subsequent requests.

