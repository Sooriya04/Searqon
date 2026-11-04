# Searqon Codebase Audit Report

**Date:** 2026-11-01  
**Project:** Searqon (`Searqon/src`)  
**Scope:** Architecture, Anti-Bot Scraper, Crawler, Database & Caching, Concurrency, and MCP Endpoints  
**Assumed Load:** Continuous production multi-user service handling concurrent agent requests, background crawl tasks, and high-volume SSE streaming.

---

## Executive Summary

Searqon is a web search aggregation and scraping engine built with Go, offering REST, SSE, and Model Context Protocol (MCP) APIs. The core architecture is modular and performs well in unit tests. However, an in-depth audit identified critical resource leaks (unbounded in-memory maps), a functional bug in cached content extraction, and hardcoded TLS verification bypass.

---

## Findings

### Must Fix

#### 1. Unbounded Memory Leak in In-Memory Search & Scrape Caches
- **Location:** `src/db/db.go` (L41–42), `src/db/db_scrape.go` (L139–142), `src/db/db_search.go` (L90–93)
- **What this is:** `searchMemoryCache` and `scrapeMemoryCache` are global `sync.Map` instances storing scrape bodies and search results in memory alongside SQLite/PostgreSQL.
- **Problem:** `StartCacheCleanupWorker` purges expired rows from SQLite/PostgreSQL, but it never deletes keys from `searchMemoryCache` or `scrapeMemoryCache`. Under continuous load or large web crawls, memory consumption grows monotonically until an Out-Of-Memory (OOM) crash occurs.
- **Fix:** In `runCleanup()`, iterate over `searchMemoryCache` and `scrapeMemoryCache` and call `Delete()` for expired items, or replace with an LRU / TTL-bounded cache.
- **If skipped:** Long-running server instances will run out of RAM and crash after heavy crawling sessions.

#### 2. Unbounded Memory Leak in IP & Domain Rate Limiters
- **Location:** `src/utils/middleware.go` (L17–27), `src/scraper/antibot.go` (L15–25)
- **What this is:** `clientLimiters` and `domainLimiters` track token-bucket rate limiters per remote client IP and per scraped domain using `sync.Map`.
- **Problem:** Limiter entries are never evicted. Over days of indexing thousands of unique domains and receiving traffic from diverse client IPs, these maps retain every entry permanently.
- **Fix:** Store a last-seen timestamp on each limiter entry and periodically purge limiters idle for more than 10–15 minutes.
- **If skipped:** The process accumulates hundreds of thousands of dormant limiter objects over time, leaking memory.

#### 3. `/extract` Endpoint Fails with HTTP 502 on Cache Hits
- **Location:** `src/handlers/extract.go` (L46–50), `src/scraper/scraper.go` (L245–252)
- **What this is:** The `/extract` HTTP endpoint takes a URL and CSS selectors, scrapes the page, and queries the raw HTML using GoQuery.
- **Problem:** `scraper.ScrapeSingleURL` returns `(models.ScrapeResult, string)`. On a cache hit, `scraper.go` returns `(cached, "")` with an empty raw HTML string. In `ExtractHandler`, the check `if htmlContent == ""` triggers an immediate HTTP 502 Bad Gateway (`"Failed to retrieve page content"`). Any URL already cached cannot be extracted unless `bypass_cache: true` is explicitly passed.
- **Fix:** In `ExtractHandler`, if `htmlContent == ""` but `result.Scraped` is true, fall back to parsing `result.Content` or reconstruct the HTML doc from the cached body.
- **If skipped:** Repeated `/extract` calls on existing pages fail with false 502 errors.

#### 4. Insecure TLS Certificate Verification Hardcoded Across All Scraper Requests
- **Location:** `src/scraper/antibot.go` (L116)
- **What this is:** `browserTLSConfig()` configures client TLS handshakes for outbound scraper connections.
- **Problem:** `InsecureSkipVerify: true` is hardcoded across all standard scraper transports. Any attacker on the local network, proxy path, or public Wi-Fi can spoof DNS/TLS certificates and perform Man-in-the-Middle (MitM) content injection against Searqon's crawler and search results.
- **Fix:** Set `InsecureSkipVerify: false` by default, or only enable it when explicitly permitted by a configuration flag (`insecure_skip_verify: true`) in `config.yml`.
- **If skipped:** Searqon can ingest altered web responses or credential exploits when routing through untrusted proxies.

---

### Should Fix

#### 5. Crawler Worker Goroutine Leak and Premature Completion Race
- **Location:** `src/crawler/crawler.go` (L132–198)
- **What this is:** `CrawlSite` spawns 5 worker goroutines reading from `jobs`, while the coordinator monitors `pendingTasks` and breaks when `pendingTasks <= 0`.
- **Problem:** If a slow worker is actively fetching a page while no other pending jobs are enqueued, the coordinator loop times out after 20ms of channel inactivity, decrements `pendingTasks`, reaches 0, and calls `close(jobs)`. The active worker can attempt to write to `discoveredLinksChan` after `CrawlSite` exits, and the workers are never tracked with a `sync.WaitGroup`.
- **Fix:** Use a `sync.WaitGroup` to coordinate worker lifecycles and ensure tasks are properly tracked before closing channels.
- **If skipped:** Deep crawls terminate before processing all discovered pages, and orphaned crawler goroutines linger in the background.

#### 6. Robots.txt Cache in Memory Lacks TTL Expiration
- **Location:** `src/scraper/robots.go` (L27–28)
- **What this is:** `robotsCache` caches parsed `robots.txt` rules keyed by domain hostname.
- **Problem:** The map has no eviction policy or TTL. Once a domain's rules are parsed, changes to the host's `robots.txt` are never refreshed until the server restarts, and every visited domain remains in memory indefinitely.
- **Fix:** Add a TTL check (e.g. 24 hours) or store robots entries with an expiration timestamp and purge stale entries.
- **If skipped:** Crawlers will follow stale permissions or permanently cache temporary crawl errors.

---

### Nice to Have

#### 7. Duplicated Browser User-Agent Collections
- **Location:** `src/scraper/antibot.go` (L41–97), `src/scraper/robots.go` (L44–53)
- **What this is:** Both `antibot.go` and `robots.go` maintain hardcoded lists of browser User-Agent strings.
- **Problem:** Updating browser versions in one file leaves the other out of sync (e.g., Chrome 128 in `antibot.go` vs Chrome 122 in `robots.go`).
- **Fix:** Unify all browser User-Agent strings in `antibot.go` and reuse `PickRandomPersona().UserAgent` inside `robots.go`.
- **If skipped:** Inconsistent browser fingerprints are presented during robots inspection versus page scraping.

---

## Verdict & Recommendation

- **Verdict:** Address items **1, 2, 3, and 4** first to eliminate memory leaks on high-volume runs, resolve the cache-hit bug in `/extract`, and enforce TLS integrity on outbound requests.
- **Lean Potential:** ~40 lines reducible by consolidating duplicate user-agent collections and deduplicating robots rule parsing.
- **Not checked:** External runtime binaries (live Camoufox Firefox processes, remote SearXNG instance queries, Ollama local inference endpoint).
