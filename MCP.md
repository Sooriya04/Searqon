# Searqon Model Context Protocol (MCP) Quick Reference

Searqon provides a native **Model Context Protocol (MCP)** server built directly in Go for AI agent platforms like **Claude Desktop**, **Cursor**, **Windsurf**, and custom autonomous agents.

For full architecture details and advanced parameters, see [`docs/MCP.md`](docs/MCP.md).

---

## ⚡ Quickstart: Docker MCP Setup

Add Searqon to your Claude Desktop configuration (`claude_desktop_config.json`) or Cursor MCP settings:

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

---

## 🛠️ Tools Exposed to AI Agents

| Tool | Purpose | Key Features |
| :--- | :--- | :--- |
| **`searqon_search`** | Real-time Web Search | SearXNG meta-search + DDG/Wikipedia fallback, deduplication, reciprocal rank fusion. |
| **`searqon_scrape`** | Ultra-Sophisticated Scraper | 4-tier anti-bot escalation (Fast HTTP $\to$ Chrome spoof $\to$ Headless $\to$ Proxy), clean Markdown converter, `max_tokens` context limit guard. |
| **`searqon_research`** | Deep Agent Research Dossier | Searches meta-engines, concurrently scrapes top sources, and formats a complete research dossier. |

---

## 🌐 Endpoints

* **CLI `stdio`:** `searqon mcp`
* **HTTP SSE:** `http://localhost:7493/mcp/sse`
* **HTTP POST:** `http://localhost:7493/mcp`
