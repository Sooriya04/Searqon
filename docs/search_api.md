# 🔍 Searqon Unified Search API (High-Performance JSON)

The Searqon Unified Search API is a high-performance intelligence layer designed to function as a private, localized web extraction engine. It orchestrates multiple specialized search services and delivers raw, structured JSON intelligence directly from the web.

## Endpoint: `/api/v1/unified` [POST]

The unified endpoint executes an optimized 3-phase retrieval pipeline to deliver clean data in under 5 seconds.

### Request Body

```json
{
  "query": "Kotlin programming",
  "limit": 5
}
```

| Field | Type | Description |
|---|---|---|
| `query` | `string` | The natural language search query. |
| `limit` | `number` | The number of results to retrieve per data source (default: 5). |

---

## 🏗️ Architecture: The Optimized Pipeline

Searqon’s intelligence layer processes every query through three distinct phases to ensure maximum speed and resource efficiency.

### Phase 1: Native Intent Routing
The **Native JS Intelligence Engine** classifies the query to determine the best data sources (e.g., academic, news, tech, or general web) in under 1ms. No external LLM or Python process is required.

### Phase 2: Parallel Domain Retrieval
Executes concurrent searches across specialized providers (GitHub, PubMed, Arxiv, Reddit, etc.) based on the routing strategy and always includes a broad web search baseline.

### Phase 3: High-Performance Extraction
The **Go Scraper Binary** fetches and cleans the content of the top matches in parallel. It strips away ads, navigation, and junk to deliver only the relevant text or Markdown.

---

## 📦 Direct JSON Response Format

By default, the Unified API returns a flat, ultra-minimal JSON array for immediate use in client-side applications or LLM prompts.

**X-Search-Duration Header**: Indicates total execution time (e.g., `2.5s`).

### Response Example

```json
[
  {
    "title": "Kotlin Programming Language",
    "url": "https://kotlinlang.org/",
    "content": "Kotlin is a modern, cross-platform, multi-purpose programming language...",
    "source": "duckduckgo"
  },
  {
    "title": "Kotlin Tutorial - GeeksforGeeks",
    "url": "https://www.geeksforgeeks.org/kotlin-programming-language/",
    "content": "Kotlin is a statically typed, general-purpose programming language developed by JetBrains...",
    "source": "duckduckgo"
  }
]
```

---

## 🛠️ Extended Features (SSE & Chat)

While the default endpoint returns raw JSON, Searqon supports advanced RAG features via dedicated routes:

- **Streaming API (`/api/search/stream`)**: Real-time token streaming using LLMs (Ollama, Gemini, OpenAI).
- **Chat API (`/api/chat`)**: Multi-turn conversational search where the AI synthesizes answers from search context.

See the [Setup Guide](./setup.md) for configuring LLM backends.
