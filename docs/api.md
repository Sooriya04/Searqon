# API Reference

Base URL: `http://localhost:4001`

All request bodies are JSON. All successful responses are JSON unless noted otherwise.

---

## `POST /search`

Multi-engine search pipeline. Expands the query, merges results across providers, optionally scrapes the top matches, and can return a synthesized summary.

### Request

```json
{
  "query": "what is ycombinator",
  "limit": 5,
  "scrape": true,
  "bypass_cache": false,
  "max_words": 800,
  "summarize": false,
  "extract_schema": "{ \"name\": \"string\", \"summary\": \"string\" }"
}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `query` | string | required | Search query |
| `limit` | int | `5` | Max results to return, clamped to 1-10 |
| `scrape` | bool | `true` | Whether to fetch and extract full page content |
| `bypass_cache` | bool | `false` | If `true`, bypasses query and scrape cache |
| `max_words` | int | `0` | Truncate scraped content to this many words when `> 0` |
| `summarize` | bool | `false` | If `true`, returns an LLM summary of the ranked results |
| `extract_schema` | string | empty | Optional schema-guided structured extraction prompt |

### Response

```json
{
  "query": "what is ycombinator",
  "provider": "multi",
  "total": 2,
  "duration": 3980,
  "summary": "Y Combinator is a startup accelerator that funds early-stage companies.",
  "results": [
    {
      "title": "Y Combinator - Wikipedia",
      "url": "https://en.wikipedia.org/wiki/Y_Combinator",
      "snippet": "Y Combinator is an American startup accelerator...",
      "source": "multi:searxng,wikipedia",
      "scraped": true,
      "content": "Y Combinator, LLC (YC) is an American technology startup accelerator...",
      "markdown": "# Y Combinator\n\nY Combinator, LLC (YC) is an American...",
      "metadata": {
        "title": "Y Combinator",
        "canonical_url": "https://www.ycombinator.com/",
        "description": "Startup accelerator",
        "author": "Y Combinator",
        "language": "en",
        "outbound_links": [
          "https://www.ycombinator.com/"
        ]
      }
    },
    {
      "title": "Y Combinator",
      "url": "https://www.ycombinator.com",
      "snippet": "Y Combinator created a new model for funding early stage startups.",
      "source": "multi:duckduckgo",
      "scraped": false
    }
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `provider` | string | `multi` or `none` |
| `duration` | int | Total pipeline time in milliseconds |
| `summary` | string | Optional synthesized answer when `summarize=true` |
| `results[].scraped` | bool | `true` means full content was extracted |
| `results[].content` | string | Plain text extracted from page |
| `results[].markdown` | string | Markdown-formatted page content |
| `results[].metadata` | object | Normalized document metadata |

### GET variant

```bash
curl "http://localhost:4001/search?q=golang+channels&scrape=false"
```

---

## `POST /pipeline`

Unified Search, Fetch, Chunk, and Rank pipeline. Discovers URLs, scrapes pages concurrently, chunks markdown text into overlapping sentence-boundary chunks, and scores them using BM25.

### Request

```json
{
  "query": "golang concurrency",
  "max_sources": 3,
  "bypass_cache": false
}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `query` | string | required | Query to search and rank chunks against |
| `max_sources` | int | `5` | Max target URLs to discover and scrape |
| `bypass_cache` | bool | `false` | Bypass cached scraping content |

### Response

```json
{
  "success": true,
  "data": {
    "query": "golang concurrency",
    "fetched_at": "2026-07-09T11:33:50Z",
    "sources": [
      {
        "url": "https://go.dev/wiki/LearnConcurrency",
        "title": "Go Wiki: LearnConcurrency",
        "render_method": "go",
        "cached": true,
        "chunks": [
          {
            "index": 1,
            "text": "Go concurrency features are ...",
            "token_count": 546,
            "word_count": 411,
            "bm25_score": 1.2374,
            "metadata": {
              "source_url": "https://go.dev/wiki/LearnConcurrency",
              "source_title": "Go Wiki: LearnConcurrency",
              "chunk_index": 1,
              "scraped_at": "2026-07-09T06:03:57Z"
            }
          }
        ]
      }
    ],
    "total_chunks": 6,
    "duration_ms": 321,
    "context": "[1] Source: Go Wiki: LearnConcurrency (https://go.dev/wiki/LearnConcurrency)\nGo concurrency features are ...\n\n[2] Source: ... (...)\n..."
  }
}
```

---

## `POST /scrape`

Scrape a single URL and return clean text, markdown, metadata, and optional structured extraction.

### Request

```json
{
  "url": "https://en.wikipedia.org/wiki/Y_Combinator",
  "format": "markdown",
  "bypass_cache": false,
  "chunk": false,
  "extract_schema": "{ \"name\": \"string\", \"summary\": \"string\" }"
}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `url` | string | required | Target URL to scrape |
| `format` | string | `markdown` | `markdown` or `text` |
| `bypass_cache` | bool | `false` | If `true`, bypasses cache and performs a live scrape |
| `chunk` | bool | `false` | If `true`, includes chunks in the response |
| `extract_schema` | string | empty | Optional structured extraction schema for the page |

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
  "duration": 980,
  "metadata": {
    "title": "Y Combinator - Wikipedia",
    "canonical_url": "https://en.wikipedia.org/wiki/Y_Combinator",
    "description": "American startup accelerator",
    "author": "Y Combinator",
    "language": "en",
    "outbound_links": [
      "https://www.ycombinator.com/"
    ]
  }
}
```

If scraping fails:

```json
{
  "url": "https://example.com",
  "error": "disallowed by robots.txt",
  "duration": 12
}
```

---

## `POST /scrape/chunked`

Scrape a single URL with chunking enabled for downstream RAG ingestion.

### Request

```json
{
  "url": "https://en.wikipedia.org/wiki/Y_Combinator",
  "format": "markdown",
  "bypass_cache": false,
  "extract_schema": "{ \"title\": \"string\", \"summary\": \"string\" }"
}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `url` | string | required | Target URL to scrape |
| `format` | string | `markdown` | `markdown` or `text` |
| `bypass_cache` | bool | `false` | If `true`, bypasses cache and performs a live scrape |
| `extract_schema` | string | empty | Optional structured extraction schema for the page |

### Response

Same structure as `/scrape`, with chunks preserved in the payload.

---

## `POST /scrape/batch`

Scrape multiple URLs concurrently, up to 20 at once.

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

Returns an array of scrape results in the same order as input URLs.

```json
[
  { "title": "Y Combinator - Wikipedia", "url": "...", "scraped": true },
  { "title": "About - Y Combinator", "url": "...", "scraped": true }
]
```

---

## `POST /scrape/html`

Parse raw HTML directly. No network request is made.

### Request

```json
{
  "html": "<html><body><h1>Hello</h1><p>World</p></body></html>",
  "url": "https://example.com",
  "format": "markdown",
  "extract_schema": "{ \"title\": \"string\" }"
}
```

### Response

Same as `/scrape`, including `metadata` and `structured_data` when extraction is requested.

---

## `GET /r/<url>`

Jina Reader compatibility endpoint. Scrapes the given URL and returns raw Markdown by default, or JSON when requested.

### Request

```bash
# Raw markdown
curl http://localhost:4001/r/https://en.wikipedia.org/wiki/Y_Combinator

# Query parameter form
curl "http://localhost:4001/r?url=https://en.wikipedia.org/wiki/Y_Combinator"

# JSON
curl -H "Accept: application/json" http://localhost:4001/r/https://en.wikipedia.org/wiki/Y_Combinator
```

### Query Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `bypass_cache` | string | `false` | Pass `true` to bypass database scrape cache |
| `json` | string | `false` | Pass `true` to force a JSON response |

### Response

Default response is `text/markdown`.

```markdown
# Y Combinator

Y Combinator, LLC (YC) is an American technology startup accelerator...
```

JSON response:

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
| `limit` | int | `30` | Max pages to crawl, hard cap 50 |
| `depth` | int | `2` | Max link depth from seed, hard cap 3 |
| `format` | string | `markdown` | Output format per page |
| `stream` | bool | `false` | Stream pages as SSE events as they're scraped |

### Response

```json
{
  "sourceUrl": "https://docs.example.com",
  "total": 18,
  "duration": 12400,
  "pages": [
    { "title": "...", "url": "...", "content": "...", "markdown": "..." }
  ]
}
```

### Streaming mode

Returns Server-Sent Events. Each scraped page is sent as it completes:

```text
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
  "endpoints": ["/search", "/scrape", "/scrape/chunked", "/scrape/batch", "/scrape/html", "/map", "/crawl", "/health", "/r/", "/pipeline"]
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
| `400` | Bad request, missing required field, or invalid JSON |
| `405` | Method not allowed |
| `504` | Scrape failed, timeout, or network error |
