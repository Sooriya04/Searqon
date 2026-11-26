# Searqon Python SDK

Official Python client library for **Searqon** — the self-hosted Web Intelligence and Deep Search Engine built for AI Agents.

---

## Highlights

- **⚡ Zero Bloat:** Built purely on `httpx` and `pydantic`.
- **🔄 Sync & Async:** First-class `Searqon` and `AsyncSearqon` clients.
- **🛡️ 4-Tier Anti-Bot Scraping:** Automatic escalation from fast HTTP to headless browsers.
- **🧠 One-Shot RAG Pipeline:** Search, scrape, and BM25 rank chunks in a single API call.
- **🤖 Agent-Ready:** Clean function signatures that drop directly into standard LLM tool calling (OpenAI, Anthropic, Gemini, LangChain, CrewAI).

---

## Installation

```bash
pip install searqon
```

Or install in development mode from the Searqon repository:

```bash
cd sdk/python
pip install -e .
```

---

## Running the Searqon Server

The Python SDK communicates with your running Searqon instance over HTTP (default port: `7493`).

### Option 1: Docker (Recommended for production)
```bash
docker run -d -p 7493:7493 searqon/searqon
```

### Option 2: Standalone Binary (Fastest & lowest memory)
Download the precompiled binary from Releases and run:
```bash
./searqon
```

---

## Quickstart

### 1. Multi-Source Search & Inline Scraping
```python
from searqon import Searqon

client = Searqon("http://localhost:7493")

# Search across search engines and scrape top matches in one request
resp = client.search(
    query="latest connectome transformer research",
    limit=5,
    scrape=True,
    max_words=500
)

print(f"Total results: {resp.total}")
for result in resp.results:
    print(f"[{result.score:.2f}] {result.title}")
    print(f"URL: {result.url}")
    if result.markdown:
        print(f"Snippet: {result.markdown[:150]}...\n")
```

### 2. High-Performance Web Scraping
```python
# Scrapes and extracts clean markdown with noise and cookie banners stripped
page = client.scrape("https://news.ycombinator.com", format="markdown")

print(f"Title: {page.title}")
print(f"Word Count: {page.word_count}")
print(f"Render Method: {page.render_method}")
print(page.markdown[:300])
```

### 3. One-Shot RAG Pipeline (Search → Scrape → BM25 Chunking)
```python
# Discovers top pages, scrapes them concurrently, and chunks them with BM25 ranking
pipeline = client.pipeline("golang memory management and GC tuning", max_sources=3)

print(f"Generated {pipeline.total_chunks} ranked chunks in {pipeline.duration_ms}ms")
for source in pipeline.sources:
    print(f"Source: {source.title} ({source.url})")
    for chunk in source.chunks:
        print(f"  Chunk #{chunk.index} (Tokens: {chunk.token_count}): {chunk.text[:120]}...")
```

### 4. Async Client (`AsyncSearqon`)
```python
import asyncio
from searqon import AsyncSearqon

async def main():
    async with AsyncSearqon("http://localhost:7493") as client:
        # Run searches or batch scrapes with sub-second concurrency
        resp = await client.search("reinforcement learning with verifiers", limit=3)
        print(f"Discovered: {resp.total} sources")

asyncio.run(main())
```

---

## Agent Integration Examples

Because Searqon methods return standard Pydantic models, you can wrap them in any AI framework with zero extra adapter libraries:

### Standard OpenAI Function Calling
```python
from searqon import Searqon

client = Searqon()

tools = [
    {
        "type": "function",
        "function": {
            "name": "web_search",
            "description": "Search the web and retrieve clean markdown content",
            "parameters": {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "The search query"},
                    "limit": {"type": "integer", "default": 5}
                },
                "required": ["query"]
            }
        }
    }
]

def execute_tool(name, args):
    if name == "web_search":
        return client.search(**args).model_dump()
```

### LangChain / CrewAI Tool
```python
from langchain.tools import tool
from searqon import Searqon

searqon_client = Searqon()

@tool
def web_search(query: str) -> str:
    """Search the web for up-to-date documentation and articles."""
    res = searqon_client.search(query=query, limit=3, scrape=True)
    return "\n\n".join([f"### {r.title}\n{r.markdown}" for r in res.results if r.markdown])
```

---

## Error Handling

The SDK exposes clean, typed exceptions:

```python
from searqon import Searqon
from searqon.exceptions import SearqonAPIError, SearqonConnectionError, SearqonRateLimitError

client = Searqon("http://localhost:7493")

try:
    results = client.search("python performance")
except SearqonConnectionError:
    print("Searqon server is not running or unreachable on port 7493.")
except SearqonRateLimitError as err:
    print(f"Rate limited by target engine: {err.message}")
except SearqonAPIError as err:
    print(f"API Error [{err.status_code}]: {err.message}")
```

---

## License

MIT License. See [LICENSE](../../LICENSE) for details.
