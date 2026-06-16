# Searqon Web Intelligence Engine: Architecture Specifications

This document outlines the system specifications, data models, error-handling routines, network normalization policies, and optimization parameters governing the Searqon search and extraction pipeline.

---

## 1. Database Schema & Cache Topology

When `DATABASE_URL` is configured, Searqon boots a PostgreSQL connection pool (`pgxpool`) configured with a max of 15 connections, a minimum of 2 idle connections, and a 15-minute idle timeout. Tables and indexes are auto-initialized at startup using the following DDL rules.

### Search Query Cache Table
Stores aggregated metadata mapping search strings to parsed source listings:
```sql
CREATE TABLE IF NOT EXISTS search_cache (
    query TEXT PRIMARY KEY,
    results JSONB NOT NULL,
    provider TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_search_cache_created_at ON search_cache(created_at);
```

### Page Scrape Cache Table
Stores extracted markdown, plain text, and processing status metadata for scraped URLs:
```sql
CREATE TABLE IF NOT EXISTS scrape_cache (
    url TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    markdown TEXT NOT NULL,
    word_count INTEGER NOT NULL,
    scraped BOOLEAN DEFAULT TRUE,
    error_msg TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_scrape_cache_created_at ON scrape_cache(created_at);
```

### Stale Entry Eviction Policies
A background worker runs **every 1 hour** on the host. It queries both tables and hard-deletes items that exceed the TTL limits:
* **Search Entries**: Evaluated against `SEARCH_CACHE_TTL_HOURS` (Default: 24h).
* **Scraped Webpages**: Evaluated against `SCRAPE_CACHE_TTL_DAYS` (Default: 7 days).

---

## 2. Error Handling & Circuit Breaking

The discovery fallback chain handles network failures, provider bans, and DNS outages to maintain high uptime.

```
       [Client Query]
             │
             ▼
      [Try SearXNG] ──(403 Forbidden / Timeout)──► [Try DuckDuckGo Lite]
             │                                              │
      (HTTP 200 OK)                                   (HTTP 200 OK)
             │                                              │
             ▼                                              ▼
      [Return Results]                               [Return Results]
```

### Fallback Rules:
1. **Primary SearXNG Failure**: If SearXNG times out (15s limit) or returns a block code (like `HTTP 403 Forbidden`), the pipeline catches the error, logs it, and immediately forwards the query to DuckDuckGo Lite.
2. **DuckDuckGo Fallback Failure**: If DuckDuckGo also fails (12s timeout or CAPTCHA block), the pipeline returns an HTTP `200 OK` empty result envelope with `"provider": "none"` rather than crashing the client's request with a server error.
3. **HTTP Status Codes**:
   * **`400 Bad Request`**: Sent if the `q` query string parameter is missing or the JSON POST body is malformed.
   * **`405 Method Not Allowed`**: Sent if a route is queried using unsupported methods (e.g., trying `PUT /search`).
   * **`500 Internal Server Error`**: Reserved for system exceptions, database driver crashes, or write errors.

---

## 3. URL Normalization & Deduplication Rules

To avoid duplicate network load and scraping redundant targets, discovered links are sanitized using the following rules:

1. **Protocol Standardization**: Mismatching protocols (e.g., `http://example.com` vs `https://example.com`) are normalized by stripping trailing separators.
2. **Trailing Slashes**: All trailing slashes (`/`) are removed from the URL string.
3. **Deduplication Hash**: Before scheduling goroutines, unique URLs are loaded into a temporary Boolean lookup map (`seenURLs := make(map[string]bool)`). Duplicate matches are logged and skipped.

---

## 4. Scraping Concurrency & Resource Management

The Stage 3 scraper coordinates multi-threaded workers under local system limits:

* **Concurrency Cap**: Restricts scraping tasks to the **top 3 results** to limit CPU usage and avoid IP blocks.
* **Global Request Deadline**: The Go orchestration routines operate inside a `context.WithTimeout` context capped at **8 seconds**. If any worker is still fetching when the timer expires, the context is cancelled, and only the successfully resolved pages return.
* **Concurrency Locks**: Because workers write to a shared results array asynchronously, writes are wrapped in a mutual exclusion lock (`sync.Mutex`) to prevent memory corruption.

---

## 5. Boilerplate Removal & Extraction Engine

The document purifier strips non-content elements from the HTML DOM to isolate readability segments:

1. **Elements Dropped**: `<script>`, `<style>`, `<noscript>`, `<iframe>`, `<svg>`, navigation sidebars, advertisement grids, header templates, and footers.
2. **Readability Heuristics**: Adapts Mozilla's Readability parser to score nodes based on link density, class name markers (e.g., stripping elements containing class names like `comment`, `sidebar`, `ad`), and text density.
3. **Format Export**: Purified HTML elements are converted to Markdown formatting and stored.

---

## 6. Environment Variables & Configuration

Below is a configuration guide for tuning the engine parameters:

| Variable | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `PORT` | Integer | `4001` | Port Searqon listens on. |
| `SEARXNG_URL` | URL | `http://localhost:8080` | Endpoint of the SearXNG search aggregator instance. |
| `DATABASE_URL` | Connection String | *None* | Connection URL for PostgreSQL cache. If empty, database caching is disabled. |
| `SEARCH_CACHE_TTL_HOURS` | Integer | `24` | Cache retention lifetime for search query results in hours. |
| `SCRAPE_CACHE_TTL_DAYS` | Integer | `7` | Cache retention lifetime for scraped page contents in days. |
