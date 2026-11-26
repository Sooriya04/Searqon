# Searqon Python SDK - API Reference

Detailed technical documentation for the `searqon` Python package.

---

## Table of Contents

1. [Client Initialization](#client-initialization)
2. [Core Client Methods](#core-client-methods)
   - [`search()`](#search)
   - [`scrape()`](#scrape)
   - [`batch_scrape()`](#batch_scrape)
   - [`pipeline()`](#pipeline)
   - [`map()`](#map)
   - [`crawl()`](#crawl)
   - [`extract()`](#extract)
   - [`health()`](#health)
3. [Asynchronous Client (`AsyncSearqon`)](#asynchronous-client-asyncsearqon)
4. [Data Models](#data-models)
5. [Exceptions](#exceptions)

---

## Client Initialization

```python
from searqon import Searqon, AsyncSearqon

client = Searqon(
    base_url="http://localhost:7493",
    api_key=None,
    timeout=30.0,
    client=None
)
```

### Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `base_url` | `str` | `"http://localhost:7493"` | URL where your Searqon instance is hosted. |
| `api_key` | `Optional[str]` | `None` | Optional Bearer token if Searqon authentication is configured. |
| `timeout` | `float` | `30.0` | Request timeout in seconds. |
| `client` | `Optional[httpx.Client]` | `None` | Pass a custom pre-configured `httpx.Client` instance if desired. |

---

## Core Client Methods

### `search()`

Executes a multi-engine search, deduplicating and ranking results across providers, with optional inline page scraping and AI summarization.

```python
resp = client.search(
    query="langchain vs smolagents",
    limit=5,
    scrape=True,
    bypass_cache=False,
    max_words=600,
    max_tokens=0,
    summarize=False,
    extract_schema=None
)
```

#### Parameters

- **`query`** (`str`, required): The search terms.
- **`limit`** (`int`, default `5`): Maximum search results (clamped between 1 and 10).
- **`scrape`** (`bool`, default `True`): If `True`, automatically scrapes and converts full text into clean markdown.
- **`bypass_cache`** (`bool`, default `False`): Force fresh results from upstream engines.
- **`max_words`** (`int`, default `0`): Truncate scraped page text to this many words (`0` = full text).
- **`max_tokens`** (`int`, default `0`): Maximum token budget for scraped text.
- **`summarize`** (`bool`, default `False`): When `True`, returns an LLM-synthesized summary across results.
- **`extract_schema`** (`Optional[str]`, default `None`): Optional schema for structured data extraction.

#### Returns
`SearchResponse` object.

---

### `scrape()`

Scrapes a target web URL using Searqon's 4-tier anti-bot escalation pipeline (fast HTTP -> spoofed headers -> headless browser -> proxy).

```python
page = client.scrape(
    url="https://en.wikipedia.org/wiki/Artificial_intelligence",
    format="markdown",
    bypass_cache=False,
    max_words=1000,
    max_tokens=0,
    chunk=False,
    extract_schema=None,
    stealth_level="auto"
)
```

#### Parameters

- **`url`** (`str`, required): Web page to scrape.
- **`format`** (`str`, default `"markdown"`): Output format: `"markdown"`, `"text"`, or `"html"`.
- **`bypass_cache`** (`bool`, default `False`): Force fresh scrape, bypassing local cache.
- **`max_words`** (`int`, default `0`): Truncate content to word count limit.
- **`max_tokens`** (`int`, default `0`): Truncate content to token limit.
- **`chunk`** (`bool`, default `False`): If `True`, returns semantic pre-chunked passages.
- **`stealth_level`** (`str`, default `"auto"`): Anti-bot level (`"auto"`, `"fast_http"`, `"spoofed_headers"`, `"headless"`, `"proxy"`).

#### Returns
`ScrapeResult` object.

---

### `batch_scrape()`

Scrapes multiple URLs in parallel using Searqon's internal concurrency worker pool.

```python
results = client.batch_scrape(
    urls=["https://example.com/page1", "https://example.com/page2"],
    format="markdown",
    bypass_cache=False,
    max_words=500
)
```

#### Returns
`List[ScrapeResult]`.

---

### `pipeline()`

One-shot automated RAG pipeline: discovers top relevant URLs via meta-search, fetches and scrapes them in parallel, chunks content, and scores them using BM25 ranking.

```python
rag = client.pipeline(
    query="postgresql connection pooling pgcat pgbouncer",
    max_sources=3,
    bypass_cache=False
)
```

#### Parameters

- **`query`** (`str`, required): Question or topic to research.
- **`max_sources`** (`int`, default `5`): Maximum URLs to discover and scrape (1-10).
- **`bypass_cache`** (`bool`, default `False`): Force fresh search and scrape.

#### Returns
`PipelineResponse` object with ranked `.sources` and `.chunks`.

---

### `map()`

Discovers domain link hierarchy and outbound links.

```python
links = client.map("https://fastapi.tiangolo.com", limit=50)
for link in links.links:
    print(link.url, link.title)
```

#### Returns
`MapResult` object.

---

### `crawl()`

Performs a recursive web crawl up to a configured depth.

```python
crawl_job = client.crawl("https://docs.searxng.org", limit=10, depth=1, max_depth=2)
```

#### Returns
`CrawlResult` containing a list of `ScrapeResult` pages.

---

### `extract()`

Extracts structured data from a target page using CSS selectors or a schema definition.

```python
data = client.extract(
    url="https://news.ycombinator.com",
    selectors={"headlines": ".titleline > a"}
)
```

#### Returns
`Dict[str, Any]` containing extracted structured fields.

---

### `health()`

Performs a lightweight ping check to verify the Searqon engine is running.

```python
status = client.health()
print(status)  # {"status": "ok"}
```

---

## Asynchronous Client (`AsyncSearqon`)

`AsyncSearqon` mirrors every method in `Searqon` with identical arguments, fully utilizing `async`/`await` for high-throughput asynchronous execution:

```python
import asyncio
from searqon import AsyncSearqon

async def main():
    async with AsyncSearqon() as client:
        # Search & scrape concurrently
        search_task = client.search("deepseek r1 architecture", limit=3)
        scrape_task = client.scrape("https://arxiv.org", max_words=300)
        
        search_res, scrape_res = await asyncio.gather(search_task, scrape_task)
        print("Search:", search_res.total)
        print("Scrape:", scrape_res.title)

asyncio.run(main())
```

---

## Data Models

All responses are validated using Pydantic v2 data models:

### `SearchResponse`
- `query` (`str`): Executed query string.
- `results` (`List[SearchResult]`): List of discovered matches.
- `total` (`int`): Count of results found.
- `duration` (`int`): Execution time in milliseconds.
- `provider` (`str`): Upstream provider name (e.g. `multi`, `searxng`).
- `summary` (`Optional[str]`): Synthesized LLM summary if requested.

### `SearchResult`
- `title` (`str`): Page title.
- `url` (`str`): Target page URL.
- `snippet` (`str`): Search snippet.
- `markdown` (`Optional[str]`): Scraped page content in markdown.
- `content` (`Optional[str]`): Scraped plain text content.
- `score` (`float`): Relevance rank score.
- `scraped` (`bool`): `True` if inline scrape succeeded.
- `metadata` (`Optional[DocumentMetadata]`): Canonical URL, author, publish date, language.
- `headings` (`List[Heading]`): Extracted document outline headers.

### `ScrapeResult`
- `title` (`str`): Extracted article title.
- `content` (`str`): Clean plain text.
- `markdown` (`Optional[str]`): Formatted markdown.
- `url` (`str`): Fetched URL.
- `word_count` (`int`): Word count of extracted text.
- `status_code` (`Optional[int]`): HTTP response code.
- `render_method` (`Optional[str]`): Fetch method (`fast_http`, `lightpanda`, `headless`).
- `bot_detected` (`bool`): Whether anti-bot defenses triggered.
- `cached` (`bool`): `True` if served from cache.
- `chunks` (`List[Chunk]`): Pre-chunked passages if chunking was requested.

### `PipelineResponse`
- `query` (`str`): Search query.
- `fetched_at` (`str`): Timestamp.
- `sources` (`List[PipelineSource]`): Extracted sources with titles and URLs.
- `total_chunks` (`int`): Total chunk count.
- `duration_ms` (`int`): Total pipeline time.

---

## Exceptions

The SDK provides a clean inheritance hierarchy for error handling:

```
SearqonError (Base)
├── SearqonConnectionError  # Server offline or request timed out
└── SearqonAPIError         # Server returned HTTP 4xx or 5xx
    ├── SearqonRateLimitError  # HTTP 429
    └── SearqonBlockedError    # HTTP 401/403 or anti-bot challenge
```
