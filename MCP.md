# Searqon Model Context Protocol (MCP) Integration Guide

Searqon provides a native **Model Context Protocol (MCP)** server built in Go. It equips AI coding agents with high-performance real-time search, 4-tier anti-bot evasion scraping, and deep research dossiers.

---

## ⚡ Supported AI Coding Agents & CLIs

### 1. OpenAI Codex CLI (`codex`)

#### Option A: CLI Command (Fastest)
```bash
# Local binary
codex mcp add searqon -- searqon mcp

# Via Docker
codex mcp add searqon -- docker run -i --rm \
  -e CACHE_ENABLED=true \
  -e SEARXNG_URL=http://host.docker.internal:8080 \
  -v searqon-data:/app/data \
  sooriya04/searqon:latest mcp

# Via Running HTTP / SSE Service (Port 7493)
codex mcp add searqon --url http://localhost:7493/mcp/sse
```

#### Option B: `~/.codex/config.toml`
```toml
[mcp_servers.searqon]
command = "searqon"
args = ["mcp"]
```
*Verify with:* `codex mcp list`

---

### 2. Claude Code CLI (`claude`)

#### Option A: CLI Command
```bash
# Local binary
claude mcp add searqon -- searqon mcp

# Via Docker
claude mcp add searqon -- docker run -i --rm \
  -e CACHE_ENABLED=true \
  -e SEARXNG_URL=http://host.docker.internal:8080 \
  -v searqon-data:/app/data \
  sooriya04/searqon:latest mcp

# Via Running HTTP / SSE Service
claude mcp add --transport sse searqon http://localhost:7493/mcp/sse
```

#### Option B: Project Scoped (`.mcp.json`)
Create `.mcp.json` in your repository root:
```json
{
  "mcpServers": {
    "searqon": {
      "command": "searqon",
      "args": ["mcp"]
    }
  }
}
```
*Verify with:* `claude mcp list`

---

### 3. Google Antigravity CLI (`agy`)

#### CLI Command
```bash
# Local binary
agy mcp add searqon searqon mcp

# Via Docker
agy mcp add searqon -- docker run -i --rm \
  -e CACHE_ENABLED=true \
  -e SEARXNG_URL=http://host.docker.internal:8080 \
  -v searqon-data:/app/data \
  sooriya04/searqon:latest mcp

# Via Running HTTP / SSE Service
agy mcp add searqon http://localhost:7493/mcp/sse
```
*Verify with:* `agy mcp list`

---

### 4. OpenCode (`opencode`)

#### Option A: CLI
```bash
opencode mcp add
```

#### Option B: Configuration File (`opencode.json` or `~/.config/opencode/opencode.jsonc`)
```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "searqon": {
      "type": "local",
      "command": ["searqon", "mcp"],
      "enabled": true
    }
  }
}
```
*For Docker:*
```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "searqon": {
      "type": "local",
      "command": [
        "docker", "run", "-i", "--rm",
        "-e", "CACHE_ENABLED=true",
        "-e", "SEARXNG_URL=http://host.docker.internal:8080",
        "-v", "searqon-data:/app/data",
        "sooriya04/searqon:latest",
        "mcp"
      ],
      "enabled": true
    }
  }
}
```
*Verify inside OpenCode with:* `/mcp`

---

### 5. Pi Agent (`pi`)

#### Option A: CLI Command
```bash
# Global configuration
pi mcp add searqon -- searqon mcp

# Project-scoped configuration
pi mcp add -l searqon -- searqon mcp

# Via Docker
pi mcp add searqon -- docker run -i --rm \
  -e CACHE_ENABLED=true \
  -e SEARXNG_URL=http://host.docker.internal:8080 \
  -v searqon-data:/app/data \
  sooriya04/searqon:latest mcp

# Via HTTP / SSE
pi mcp add searqon --url http://localhost:7493/mcp/sse
```

#### Option B: `~/.pi/agent/mcp.json` or `.pi/mcp.json`
```json
{
  "mcpServers": {
    "searqon": {
      "command": "searqon",
      "args": ["mcp"]
    }
  }
}
```
*Verify with:* `pi mcp list` (or `/mcp` inside the interactive session).

---

### 6. Claude Desktop, Cursor & Windsurf

#### Claude Desktop Configuration
File path:
* **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
* **Linux:** `~/.config/Claude/claude_desktop_config.json`
* **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "searqon": {
      "command": "searqon",
      "args": ["mcp"]
    }
  }
}
```

#### Cursor & Windsurf Settings
Add under MCP Settings as a new **Command** provider:
* **Name:** `searqon`
* **Command:** `searqon`
* **Args:** `mcp`

*(Or use `docker` as the command with `run -i --rm ... sooriya04/searqon:latest mcp` as args).*

---

## 🛠️ Tools Exposed to AI Agents

| Tool | Purpose | Key Parameters |
| :--- | :--- | :--- |
| **`searqon_search`** | Real-time Web Search | `query` *(string, required)*, `limit` *(int, opt)*, `scrape` *(bool, opt)* |
| **`searqon_scrape`** | 4-Tier Anti-Bot Web Scraper & Markdown Cleaner | `url` *(string, required)*, `max_tokens` *(int, opt)*, `format` *(opt)*, `stealth_level` *(opt)* |
| **`searqon_research`** | Deep Multi-Source Agent Research Dossier | `query` *(string, required)*, `max_sources` *(int, opt)*, `max_tokens_per_source` *(opt)* |
| **`searqon_map`** | Sitemap & Internal Link Discovery Crawler | `url` *(string, required)*, `limit` *(int, opt)* |
| **`searqon_extract`** | CSS Selector & JSON Schema Extraction | `url` *(string, required)*, `selectors` *(opt)*, `schema` *(opt)* |

---

## 🌐 Endpoints & Direct Testing

* **CLI `stdio`:** `searqon mcp`
* **HTTP SSE:** `http://localhost:7493/mcp/sse`
* **HTTP POST:** `http://localhost:7493/mcp`

### Test with JSON-RPC ping:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | searqon mcp
```
