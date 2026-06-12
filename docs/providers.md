# Search Providers

Searqon uses a two-provider fallback chain for web search discovery. Providers supply **URLs + snippets** — the scraping layer then fetches the actual page content separately.

---

## Provider Chain

```
Request comes in
      │
      ▼
1. SearXNG  ──────────────────────── Is it running at localhost:8080?
      │ YES → use it                        │ NO
      │                                     ▼
      │                          2. DuckDuckGo Lite
      │                             html.duckduckgo.com/html/
      │                             parsed with goquery
      ▼
   Results (title, url, snippet, source)
```

---

## 1. SearXNG (Primary)

**SearXNG** is a self-hosted, open-source metasearch engine. It aggregates results from multiple search engines simultaneously (Google, Bing, Qwant, DuckDuckGo, etc.) without tracking users.

### Why SearXNG is better
- Aggregates **multiple engines** — more diverse, higher quality results
- Never rate-limited (it's running on your own machine)
- Returns results in clean **JSON format** directly — no HTML parsing needed
- Typically responds in **~500ms** vs ~2s for DDG
- Handles CAPTCHA/IP-blocking on your behalf (it rotates between engines)

### Setup

```bash
# Standalone Docker container (independent from Searqon)
docker run -d --name searxng -p 8080:8080 searxng/searxng

# Or via Makefile
make searxng

# Verify it's working
curl "http://localhost:8080/search?q=golang&format=json" | jq '.results[0]'
```

### How Searqon calls it

```
GET http://localhost:8080/search?q=<query>&format=json&language=en&engines=google,bing,duckduckgo,qwant
```

Response field used: `results[].title`, `results[].url`, `results[].content`

---

## 2. DuckDuckGo Lite (Fallback)

When SearXNG is offline, Searqon automatically uses DuckDuckGo's lightweight HTML endpoint at `https://html.duckduckgo.com/html/`.

### Why not the normal DDG page?
- `duckduckgo.com` uses heavy JavaScript — the results are rendered in the browser, not in the HTML
- `html.duckduckgo.com/html/` is a no-JS, accessibility-friendly version that returns actual HTML results
- Parsed with **goquery** using the `.result__title`, `.result__url`, `.result__snippet` selectors

### Limitations
- Rate-limited after multiple rapid requests from the same IP
- Returns ~2s (slower than SearXNG)
- Only aggregates DuckDuckGo's index (not multi-engine)
- May return ads as the first result (filtered in post-processing)

### No configuration needed
The fallback is fully automatic. If SearXNG responds with a connection error or HTTP 5xx, Searqon immediately retries with DDG.

---

## Provider in Response

The response from `/search` always tells you which provider was used:

```json
{
  "provider": "searxng",   // or "duckduckgo" or "none"
  ...
}
```

`"none"` means both providers failed. Results will be empty but the request won't crash.

---

## Recommended Setup

For the best results and reliability:

```bash
# Run SearXNG once (stays running in background)
docker run -d --name searxng --restart unless-stopped -p 8080:8080 searxng/searxng

# Run Searqon
cd src && go run .
```

Both are completely independent services — stopping one does not affect the other.
