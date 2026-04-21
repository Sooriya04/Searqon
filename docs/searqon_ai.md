# Building Perplexity-style AI Search with Searqon

Searqon is designed to be the high-performance retrieval engine that powers advanced AI search experiences. While Perplexity uses proprietary models and search indices, Searqon gives you the same "Query-to-Synthesized-Knowledge" pipeline in a self-hosted, transparent architecture.

## The Perplexity Workflow
A "Perplexity-style" search is not just a standard web search. It follows a multi-step orchestration that Searqon handles natively:

1.  **Intent Classification**: Deciding if the user needs general web results, academic papers, technical documentation, or community forum discussions.
2.  **Multi-Source Discovery**: Running parallel searches across Google/DuckDuckGo and specialized hubs (Reddit, GitHub, ArXiv).
3.  **Full-Page Extraction**: Fetching the actual content of the top results and converting it into clean Markdown (avoiding the "snippet-only" limitation of standard search APIs).
4.  **Reranking**: Using semantic similarity to ensure the most relevant content is moved to the top of the context window.
5.  **Synthesis (The Final Step)**: Feeding this rich context into an LLM to generate the final answer with citations.

---

## 🚀 The Primary Route: `/api/v2/research`

For developers building AI search interfaces, the **`/api/v2/research`** endpoint is your one-stop-shop. It encapsulates the entire retrieval pipeline in a single call.

### Why use `/research`?
*   **Context Density**: Unlike standard search APIs that return 200-character snippets, Searqon returns the **full extracted content** of the pages. This allows the LLM to "read" the source, not just guess from a headline.
*   **Automatic Specialized Routing**: If a user asks "What is the consensus on the M4 chip?", Searqon automatically pulls from Reddit threads to get human opinions. If they ask about "Transformer architectures," it hits ArXiv and Wikipedia.
*   **LLM-Ready Metadata**: Every result includes a `title`, `url`, and `content`, making it trivial to implement clickable citations.

### Example Integration
```javascript
// Step 1: Get the context from Searqon
const response = await fetch('http://your-searqon-instance/api/v2/research', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ query: "What are the latest breakthroughs in fusion energy?" })
});

const context = await response.json();

// Step 2: Pass context to your LLM (OpenAI, Anthropic, or Local)
const prompt = `
Answer the user's question based ONLY on the provided search results. 
Use citations like [1], [2] corresponding to the source URLs.

User Question: ${userQuery}

Search Results:
${context.map((c, i) => `[${i+1}] ${c.title} (${c.url}): ${c.content}`).join('\n\n')}
`;
```

---

## 🛠️ Advanced: Custom AI Pipelines
If you need more granular control (e.g., streaming results as they are found), you can compose your own "Perplexity-lite" experience using Searqon's primitives:

1.  **`/api/v2/search`**: Use this for initial "Discovery" to show the user which URLs you've found before you start scraping.
2.  **`/api/v2/scrape`**: Use this to fetch specific high-value pages in the background.
3.  **`/api/v2/extract`**: Use this if you need to turn raw web data into a specific JSON schema (e.g., comparing product specifications or pricing).

---

## Why Searqon is Better for AI Search
*   **Self-Hosted & Private**: No data leaves your infrastructure during the retrieval phase.
*   **Cost Efficiency**: Replace expensive $0.05/search black-box APIs with a system that costs $0.00 to run.
*   **Transparent Sources**: You control exactly where the data comes from, avoiding "hallucinating" search results.
