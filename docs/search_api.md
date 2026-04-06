# 🔍 Searqon Unified Search API (AI Search Engine)

The Searqon Unified Search API is a high-performance, RAG-driven intelligence layer designed to function as a private, localized version of **Google Search AI** (AI Overviews). It orchestrates multiple specialized search services and synthesizes the results into a single, cohesive "Smart Answer" using LLMs like Gemini, OpenAI, or Ollama.

## Endpoint: `/api/search/unified` [POST]

The unified endpoint executes a 5-phase retrieval and synthesis pipeline to deliver structured intelligence directly from the web.

### Request Body

```json
{
  "query": "what is the rate of firecrawl",
  "limit": 5
}
```

| Field | Type | Description |
|---|---|---|
| `query` | `string` | The natural language search query. |
| `limit` | `number` | The number of results to retrieve per data source (default: 5). |

---

## 🏗️ Architecture: The 5-Phase Pipeline

Searqon’s intelligence layer processes every query through five distinct phases to ensure speed, accuracy, and depth.

### Phase 0: Semantic Intent Routing
The **Python Intelligence Layer** classifies the query to determine the best data sources (e.g., academic, news, tech, or general web).

### Phase 1: Concurrent Domain Retrieval
Executes parallel searches across specialized providers (GitHub, PubMed, Arxiv, Reddit, etc.) based on the routing strategy.

### Phase 2: DuckDuckGo Baseline
Runs a broad web search via DuckDuckGo to fill metadata gaps and ensure comprehensive coverage.

### Phase 3: TF-IDF Research Highlights
Extracts the most relevant "snippets" from thousands of lines of scraped Markdown using a pure Python TF-IDF engine.

### Phase 4: Smart Answer AI (RAG)
**[NEW]** Synthesizes all retrieved context into a single, direct, and factual answer using the configured LLM backend. This provides a "direct response" experience instead of just a list of links.

---

## 🛠️ Configuration (LLM Backends)

The Smart Answer feature is controlled via environment variables in the Node.js layer:

| Variable | Values | Description |
|---|---|---|
| `EXTRACTION_BACKEND` | `ollama`, `gemini`, `openai` | Selects the AI synthesis engine. |
| `GEMINI_API_KEY` | `string` | Required if using Gemini backend. |
| `OPENAI_API_KEY` | `string` | Required if using OpenAI backend. |
| `OLLAMA_URL` | `string` | URL of the local Ollama instance (default: http://localhost:11434). |
| `OLLAMA_MODEL` | `string` | Model name (default: `qwen2.5:0.5b`). |

---

## 🔮 Future Roadmap: Google Search AI mode

Planned features to reach full parity with modern AI search overviews:

- [ ] **Inline Citations**: Clickable `[1]`, `[2]` links within the Smart Answer mapped directly to source URLs.
- [ ] **Knowledge Panel Extraction**: Automatic JSON extraction of key stats (price, HQ, founders, GitHub stars) into a structured sidebar.
- [ ] **Agentic Deep Search**: Allowing the LLM to autonomously trigger a second search if the initial results are insufficient.
- [ ] **Streaming Responses**: Real-time "typing" effect for the AI answer to reduce perceived latency.
- [ ] **Follow-up Chat**: Interactive search sessions where you can ask clarifying questions about the results.
