# API Reference

Base URL: `http://localhost:4001`

All request bodies are JSON. All responses are JSON with `Content-Type: application/json`.

---

## `POST /search`

The main pipeline endpoint. Runs a web search and optionally scrapes the top result pages.

### Request

```json
{
  "query": "what is ycombinator",
  "limit": 5,
  "scrape": true,
  "bypass_cache": false
}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `query` | string | required | Search query |
| `limit` | int | `5` | Max results to return (1–10) |
| `scrape` | bool | `true` | Whether to fetch and extract full page content |
| `bypass_cache` | bool | `false` | If `true`, bypasses cache and performs live search/scrapes |

### Response

```json
{
  "query": "what is ycombinator",
  "provider": "duckduckgo",
  "total": 5,
  "duration": 3980,
  "results": [
    {
      "title": "Y Combinator - Wikipedia",
      "url": "https://en.wikipedia.org/wiki/Y_Combinator",
      "snippet": "Y Combinator is an American startup accelerator...",
      "source": "duckduckgo",
      "scraped": true,
      "content": "Y Combinator, LLC (YC) is an American technology startup...",
      "markdown": "# Y Combinator\n\nY Combinator, LLC (YC) is an American..."
    },
    {
      "title": "Y Combinator",
      "url": "https://www.ycombinator.com",
      "snippet": "Y Combinator created a new model for funding early stage startups.",
      "source": "duckduckgo",
      "scraped": false
    }
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `provider` | string | `"searxng"` or `"duckduckgo"` or `"none"` |
| `duration` | int | Total pipeline time in milliseconds |
| `results[].scraped` | bool | `true` = full content extracted, `false` = snippet only |
| `results[].content` | string | Plain text extracted from page (only when `scraped: true`) |
| `results[].markdown` | string | Markdown-formatted page content (only when `scraped: true`) |

### GET variant

```bash
curl "http://localhost:4001/search?q=golang+channels&scrape=false"
```

---

## `POST /scrape`

Scrape a single URL and return clean text + markdown.

### Request

```json
{
  "url": "https://en.wikipedia.org/wiki/Y_Combinator",
  "format": "markdown",
  "bypass_cache": false
}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `url` | string | required | Target URL to scrape |
| `format` | string | `"markdown"` | `"markdown"` or `"text"` |
| `bypass_cache` | bool | `false` | If `true`, bypasses cache and performs a live scrape |

### Response

```json
{
  "title": "Y Combinator - Wikipedia",
  "url": "https://en.wikipedia.org/wiki/Y_Combinator",
  "content": "Y Combinator, LLC (YC) is an American technology startup accelerator...",
  "markdown": "# Y Combinator\n\nY Combinator, LLC (YC) is an American...",
  "wordCount": 1842,
  "startTime": "2026-06-06T17:30:00Z",
  "endTime": "2026-06-06T17:30:01Z",
  "duration": 980
}
```

If scraping fails (blocked, timeout, robots.txt disallowed):
```json
{
  "url": "https://example.com",
  "error": "disallowed by robots.txt",
  "duration": 12
}
```

---

## `POST /scrape/batch`

Scrape multiple URLs concurrently (up to 20 at once).

### Request

```json
{
  "urls": [
    "https://en.wikipedia.org/wiki/Y_Combinator",
    "https://www.ycombinator.com/about/"
  ],
  "format": "markdown",
  "bypass_cache": false
}
```

### Response

Returns an **array** of scrape results in the same order as input URLs:

```json
[
  { "title": "Y Combinator - Wikipedia", "url": "...", "scraped": true, ... },
  { "title": "About - Y Combinator", "url": "...", "scraped": true, ... }
]
```

---

## `POST /scrape/html`

Parse raw HTML you already have. No network request is made.

### Request

```json
{
  "html": "<html><body><h1>Hello</h1><p>World</p></body></html>",
  "url": "https://example.com",
  "format": "markdown"
}
```

### Response

Same as `/scrape`.

---

## `GET /r/<url>`

Jina Reader compatibility endpoint. Scrapes the given URL and returns its content as raw Markdown (default) or JSON.

### Request

```bash
# Get raw markdown
curl http://localhost:4001/r/https://en.wikipedia.org/wiki/Y_Combinator

# Get raw markdown (query parameter style)
curl "http://localhost:4001/r?url=https://en.wikipedia.org/wiki/Y_Combinator"

# Get JSON format
curl -H "Accept: application/json" http://localhost:4001/r/https://en.wikipedia.org/wiki/Y_Combinator
```

### Query Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `bypass_cache` | string | `"false"` | Pass `"true"` to bypass database scrape cache. |
| `json` | string | `"false"` | Pass `"true"` to force a JSON response. |

### Response (Default: text/markdown)

```markdown
# Y Combinator

Y Combinator, LLC (YC) is an American technology startup accelerator...
```

### Response (JSON: Accept header or `json=true`)

```json
{
  "code": 200,
  "status": "success",
  "data": {
    "title": "Y Combinator - Wikipedia",
    "url": "https://en.wikipedia.org/wiki/Y_Combinator",
    "content": "# Y Combinator\n\nY Combinator, LLC (YC) is an American...",
    "raw": "Y Combinator, LLC (YC) is an American technology startup accelerator...",
    "usage": {
      "tokens": 1842
    }
  }
}
```

---

## `POST /crawl`

Crawl an entire site starting from a seed URL. Follows internal links up to `depth` levels deep.

### Request

```json
{
  "url": "https://docs.example.com",
  "limit": 30,
  "depth": 2,
  "format": "markdown",
  "stream": false
}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `url` | string | required | Seed URL to start crawling |
| `limit` | int | `30` | Max pages to crawl (hard cap: 50) |
| `depth` | int | `2` | Max link depth from seed (hard cap: 3) |
| `format` | string | `"markdown"` | Output format per page |
| `stream` | bool | `false` | Stream pages as SSE events as they're scraped |

### Response (non-streaming)

```json
{
  "sourceUrl": "https://docs.example.com",
  "total": 18,
  "duration": 12400,
  "pages": [
    { "title": "...", "url": "...", "content": "...", "markdown": "..." },
    ...
  ]
}
```

### Streaming Mode (`stream: true`)

Returns Server-Sent Events. Each scraped page is sent as it completes:

```
data: {"title":"...","url":"...","content":"..."}

data: {"title":"...","url":"...","content":"..."}

event: done
data: {}
```

---

## `POST /map`

Discover all internal URLs on a site without scraping content.

### Request

```json
{
  "url": "https://docs.example.com",
  "limit": 50
}
```

### Response

```json
{
  "sourceUrl": "https://docs.example.com",
  "count": 23,
  "duration": 1200,
  "links": [
    { "url": "https://docs.example.com/getting-started", "title": "Getting Started" },
    { "url": "https://docs.example.com/api", "title": "API Reference" }
  ]
}
```

---

## `GET /health`

Health check endpoint.

### Response

```json
{
  "status": "ok",
  "engine": "src",
  "endpoints": ["/scrape", "/scrape/html", "/scrape/batch", "/map", "/crawl", "/health", "/r/"]
}
```

---

## Error Responses

All errors return a JSON body:

```json
{ "error": "query is required" }
```

| HTTP Status | Meaning |
|---|---|
| `400` | Bad request (missing required field, invalid JSON) |
| `405` | Method not allowed |
| `504` | Scrape failed (timeout, network error) |
