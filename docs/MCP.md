# Searqon Model Context Protocol (MCP) Guide

Searqon provides a native **Model Context Protocol (MCP)** server built in Go. It empowers AI agent environments—such as **Claude Desktop**, **Cursor**, **Windsurf**, and custom LLM agent frameworks—with ultra-sophisticated web search, 4-tier anti-bot evasion scraping, and deep research capabilities.

---

## ⚡ Key Highlights for AI Agents

1. **Ultra-Lightweight (~8 MB)**: Written in pure Go with zero CGO dependencies; starts instantly (<10ms) inside a `scratch` Docker container.
2. **Context Window Protection**: Web pages are automatically stripped of ads, cookie banners, navigation clutter, and boilerplate, converted into clean Markdown, and capped by an agent token budget (`max_tokens`) so you never exhaust LLM context limits.
3. **4-Tier Anti-Bot Bypass**: Escalates seamlessly through Fast HTTP $\to$ Spoofed Chrome/TLS Fingerprint $\to$ Headless Browser (Lightpanda) $\to$ Rotating Residential Proxies to solve Cloudflare, 403 Forbidden, and anti-scraping walls.
4. **SearXNG Meta-Search**: Aggregates results across SearXNG, DuckDuckGo, Arxiv, and Wikipedia with automatic reciprocal rank fusion (RRF).

---

## 🚀 Running via Docker (Recommended)

### 1. Claude Desktop Integration (`stdio` Transport)

Add Searqon to your Claude Desktop configuration file:
* **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
* **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
* **Linux**: `~/.config/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "searqon": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "-e", "CACHE_ENABLED=true",
        "-e", "SEARXNG_URL=http://host.docker.internal:8080",
        "-v", "searqon-data:/app/data",
        "sooriya04/searqon:latest",
        "mcp"
      ]
    }
  }
}
```

> **Note:** The `-i` flag keeps standard input open so Claude Desktop can send and receive JSON-RPC 2.0 messages over `stdio`.

---

### 2. Cursor / Windsurf Integration

In Cursor or Windsurf MCP Settings, configure a new **Command** provider:

* **Name:** `searqon`
* **Command:** `docker`
* **Args:**
  ```text
  run -i --rm -e CACHE_ENABLED=true -e SEARXNG_URL=http://host.docker.internal:8080 -v searqon-data:/app/data sooriya04/searqon:latest mcp
  ```

---

### 3. Remote Agents / HTTP SSE Transport

When Searqon runs as an HTTP service (e.g. `docker run -p 7493:7493 sooriya04/searqon:latest`), remote agents can connect without spinning up a local container:

* **SSE Streaming Endpoint:** `http://localhost:7493/mcp/sse`
* **JSON-RPC Message Endpoint:** `http://localhost:7493/mcp`

---

## 🛠️ Available MCP Tools

### 1. `searqon_search`
Searches across SearXNG and meta-engines with deduplication and relevance scoring.

**Parameters:**
* `query` *(string, required)*: The search query.
* `limit` *(integer, optional)*: Maximum results (default: `5`, max: `20`).
* `category` *(string, optional)*: `"general"`, `"science"`, `"it"`, `"news"`.
* `scrape` *(boolean, optional)*: If `true`, concurrently fetches and returns clean Markdown for top results.

---

### 2. `searqon_scrape`
The agent-grade web reader. Resolves bot challenges, extracts readable content into GitHub-flavored Markdown, and truncates safely.

**Parameters:**
* `url` *(string, required)*: Target URL to scrape.
* `max_tokens` *(integer, optional)*: Maximum token budget (default: `4000`). Prevents context overflow.
* `format` *(string, optional)*: `"markdown"` (default), `"text"`, or `"structured_json"`.
* `stealth_level` *(string, optional)*: `"auto"` (default), `"fast_http"`, `"spoofed_headers"`, `"headless"`, `"proxy"`.
* `bypass_cache` *(boolean, optional)*: Force a live scrape bypassing SQLite/Redis cache.

---

### 3. `searqon_research`
One-shot deep research tool for complex agent reasoning tasks. Queries SearXNG, selects authoritative pages, scrapes them concurrently with anti-bot bypass, and synthesizes a structured context dossier.

**Parameters:**
* `query` *(string, required)*: Topic or research question.
* `max_sources` *(integer, optional)*: Number of top sources to scrape and compile (default: `3`, max: `8`).
* `max_tokens_per_source` *(integer, optional)*: Maximum tokens to include per source document (default: `2000`).

---

### 4. `searqon_map`
Autonomous sitemap explorer. Discovers and maps internal URL structures across an entire domain or website hierarchy for agent navigation.

**Parameters:**
* `url` *(string, required)*: Website root or section URL to map.
* `limit` *(integer, optional)*: Maximum internal links to return (default: `50`, max: `150`).

---

### 5. `searqon_extract`
Targeted data extraction from any webpage using CSS selectors or structured JSON schema instructions.

**Parameters:**
* `url` *(string, required)*: Webpage to extract from.
* `selectors` *(object, optional)*: Map of field names to CSS selectors (e.g. `{"title": "h1", "price": ".price"}`).
* `schema` *(string, optional)*: Natural language prompt or schema description for structured JSON extraction.
* `bypass_cache` *(boolean, optional)*: Skip cache for a live fetch.

---

## 💻 Local CLI / Binary Usage

To run the MCP server directly from the compiled binary:

```bash
# Starts MCP server on stdio
./searqon mcp

# Test with JSON-RPC ping
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | ./searqon mcp
```
