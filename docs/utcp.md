# Universal Tool Calling Protocol (UTCP) & Code-Mode Integration

Searqon provides native, first-class support for the **Universal Tool Calling Protocol (UTCP v1.0.1+)** and client-side **Code-Mode** execution patterns.

Any autonomous agent, orchestration framework (Python, TypeScript, Go), or Code-Mode sandbox can discover Searqon's web intelligence capabilities dynamically over HTTP (`GET /utcp` or `GET /utcp/manual`) and execute multi-step search, scrape, map, and extraction tasks in a single turn without running intermediary MCP daemons or client-side SDK wrappers.

---

## 1. Dynamic Discovery (`GET /utcp` and `GET /utcp/manual`)

Searqon exposes its authoritative tool configuration manual at:
- `GET /utcp`
- `GET /utcp/manual`

### Dynamic Host & Base URL Resolution
Searqon dynamically computes the `url` in every `tool_call_template` according to your deployment environment:
1. **`SEARQON_PUBLIC_URL`**: Explicit environment variable override (e.g. `https://searqon.example.com`).
2. **Reverse Proxy Headers**: Honors `X-Forwarded-Proto` (`https`/`http`) and `X-Forwarded-Host` (taking the primary upstream host from comma-delimited proxy headers).
3. **HTTP Host Fallback**: Inspects incoming `r.Host` or falls back to `localhost:7493`.

### CORS & Pre-Flight Support
Both discovery endpoints emit full CORS headers:
- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: GET, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type, Authorization`

`OPTIONS` requests return `204 No Content` for immediate pre-flight resolution in browser and desktop web environments.

---

## 2. Exposed UTCP Tools Reference

The table below lists all tools published in Searqon's UTCP manual:

| Tool Name | Transport | Endpoint / Method | Description | Key Parameters |
| :--- | :--- | :--- | :--- | :--- |
| `searqon_search` | `http` | `POST /search` | Real-time meta-search across SearXNG, DuckDuckGo, Arxiv, and Wikipedia with reciprocal rank fusion. | `query` (str, req), `limit` (int, def: 5), `scrape` (bool, def: false), `category` (enum: general/science/it/news), `max_tokens` (int, def: 4000) |
| `searqon_scrape` | `http` | `POST /scrape` | 4-tier anti-bot scraper (Cloudflare/bot shield bypass) returning clean Markdown. | `url` (str, req), `format` (enum: markdown/text/html, def: markdown), `max_tokens` (int, def: 8000), `bypass_cache` (bool, def: false) |
| `searqon_research` | `http` | `POST /pipeline` | One-shot deep research: search, concurrently scrape top authoritative pages, and synthesize chunked sources. | `query` (str, req), `max_sources` (int, def: 5), `bypass_cache` (bool, def: false) |
| `searqon_map` | `http` | `POST /map` | Discover and map internal URLs and sitemap hierarchy across a target website or domain. | `url` (str, req), `limit` (int, def: 50, max: 150) |
| `searqon_extract` | `http` | `POST /extract` | Targeted CSS selector or structured JSON schema extraction for DOM elements and tables. | `url` (str, req), `schema` (str, req), `bypass_cache` (bool, def: false) |
| `searqon_crawl` | `http` | `POST /crawl` | Recursively discover and scrape linked pages under a target domain hierarchy up to a max depth. | `url` (str, req), `max_depth` (int, def: 2), `limit` (int, def: 20) |
| `searqon_search_stream` | `sse` | `GET /search/stream?query={query}` | Real-time streaming web search emitting discovery, scraping, and synthesis events over SSE. | `query` (str, req) |

---

## 3. Client Recipes

### Recipe A: Code-Mode TypeScript (`@utcp/code-mode`)
Execute a complete multi-step deep research pipeline in a **single LLM generation turn**:

```typescript
import { CodeModeUtcpClient } from "@utcp/code-mode";

async function main() {
  // 1. Initialize Code-Mode client and register Searqon's discovery endpoint
  const client = await CodeModeUtcpClient.create();
  await client.registerManual({
    name: "searqon",
    url: "http://localhost:7493/utcp",
  });

  // 2. Execute multi-step research pipeline in a single sandbox turn
  const { result } = await client.callToolChain(`
    // Search top sources
    const search = await searqon.searqon_search({
      query: "state of generative AI in robotics 2026",
      limit: 3
    });

    // Concurrently scrape discovered URLs with anti-bot evasion
    const scrapedDocs = await Promise.all(
      search.results.map((r) =>
        searqon.searqon_scrape({
          url: r.url,
          format: "markdown",
          max_tokens: 3000
        })
      )
    );

    return {
      query: search.query,
      sourcesCount: scrapedDocs.length,
      dossier: scrapedDocs.map((doc) => ({
        title: doc.title,
        url: doc.url,
        markdown: doc.markdown
      }))
    };
  `);

  console.log("Synthesized Research Result:", JSON.stringify(result, null, 2));
}

main().catch(console.error);
```

---

### Recipe B: Python UTCP Client (`utcp`)
Direct manual registration and tool invocation using the official Python UTCP library:

```python
import asyncio
from utcp import UtcpClient

async def main():
    # 1. Initialize client and register Searqon manual
    client = UtcpClient()
    await client.register_manual_from_url("http://localhost:7493/utcp")

    # 2. Invoke searqon_search directly over native HTTP
    search_res = await client.call_tool(
        "searqon.searqon_search",
        {
            "query": "vector databases performance benchmarks",
            "limit": 3,
            "scrape": False,
        }
    )
    print("Search Results:", search_res)

    # 3. Scrape top resulting URL with anti-bot bypass
    if search_res.get("results"):
        top_url = search_res["results"][0]["url"]
        scrape_res = await client.call_tool(
            "searqon.searqon_scrape",
            {
                "url": top_url,
                "format": "markdown",
                "max_tokens": 4000,
            }
        )
        print("Scraped Title:", scrape_res.get("title"))
        print("Scraped Content Preview:\n", scrape_res.get("markdown", "")[:500])

if __name__ == "__main__":
    asyncio.run(main())
```

---

### Recipe C: Go UTCP Client (`go-utcp`)
Direct integration within Go microservices or agent runtimes:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/universal-tool-calling-protocol/go-utcp"
)

func main() {
	ctx := context.Background()

	// 1. Initialize client and register Searqon discovery endpoint
	client := utcp.NewClient()
	err := client.RegisterManualFromURL(ctx, "http://localhost:7493/utcp")
	if err != nil {
		log.Fatalf("Failed to register Searqon UTCP manual: %v", err)
	}

	// 2. Execute search call
	params := map[string]interface{}{
		"query": "sovereign local search and rag pipelines",
		"limit": 5,
	}

	result, err := client.CallTool(ctx, "searqon_search", params)
	if err != nil {
		log.Fatalf("Tool call failed: %v", err)
	}

	fmt.Printf("Search output: %+v\n", result)
}
```

---

## 4. MCP vs. UTCP Architectural Trade-Offs

Searqon provides native dual-protocol support for both **MCP** and **UTCP**:

| Protocol | Transport | Best For | Architecture | Round-Trip Characteristics |
| :--- | :--- | :--- | :--- | :--- |
| **MCP** | Stdio / HTTP SSE | Desktop chat interfaces (Claude Desktop, Cursor, Windsurf) | Stateful daemon or long-lived stdio subprocess | Sequential round-trips: `Model -> Tool Call -> Model -> Tool Call` (1 turn per call) |
| **UTCP** | Stateless HTTP & SSE | Autonomous agent backends, multi-container deployments, and Code-Mode batching | Declarative discovery manual with direct HTTP endpoint invocation | Single-turn batching via Code-Mode (`Promise.all`), saving 60–80% network latency and context tokens |
