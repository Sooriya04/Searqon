# 🚀 Searqon API V1 Endpoints Reference

Searqon is built as a highly modular, microservice-based web extraction engine. All endpoints are versioned under `/api/v1`.

### 💡 Why this architecture?
Instead of a single "do-it-all" script, Searqon splits responsibilities into three major tiers:
1. **Core Processing**: The Go binary handles heavy HTML parsing and Markdown conversion extremely fast, saving memory.
2. **Global Discovery**: We use Talven (Meta-Search) + DuckDuckGo to scour the entire open web, ensuring maximum URL coverage.
3. **Specialized Sources**: We bypass search engines entirely for sites like PubMed or GitHub, querying their native systems to avoid SEO spam and get direct, accurate data.

This setup makes Searqon the ultimate, resilient data-ingestion pipeline for Natural Language Processing (NLP) models and AI Information Retrieval (IR) systems.

---

## 🛡️ Tier 1: Core Engine (Extraction & Crawling)
These endpoints provide the primary scraping, crawling, and extraction capabilities powered by the native Go engine. 

**Why needed?** Because AI models and NLP systems cannot read raw HTML cluttered with ads and scripts. These endpoints convert any raw website into clean, LLM-ready Markdown.

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/api/v1/scrape` | Scrape a single URL into high-fidelity Markdown. Automatically uses Puppeteer fallback if JS rendering is needed. |
| `POST` | `/api/v1/crawl` | Recursive, autonomous crawl of an entire domain. Returns all pages as a Markdown dataset. |
| `GET`  | `/api/v1/crawl/:id` | Get status and results for a background crawl job. |
| `POST` | `/api/v1/map` | "Spider" mode. Instantly extract and discover all internal URLs on a given domain. |
| `POST` | `/api/v1/extract` | Passes scraped Markdown to a local LLM (e.g., Ollama/Qwen) to extract structured JSON data. |

---

## 🔍 Tier 2: Global Search (Discovery)
Unified search across the general web. 

**Why needed?** When you don't have a specific URL, you need to "discover" it. These endpoints use **Talven** (a privacy Meta-Search aggregator) and DuckDuckGo in parallel to find the highest-quality links for your query, perfectly setting up the extraction phase.

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/api/v1/search` | Performs a general web search via Talven & DuckDuckGo, **and immediately scrapes** the top results into Markdown. |
| `POST` | `/api/v1/unified` | **Primary Unified Search Hub**. Intelligently routes the query, runs searches in parallel, and returns a flat JSON array directly to your NLP/IR system. |
| `POST` | `/api/v1/classify` | Searqon's Native Intent Classifier. Decides which sources to query in <1ms without hitting an external LLM. |

---

## 📚 Tier 3: Specialized Search Sources
Access specific data pools directly under the sources marketplace.

**Why needed?** General search engines (like Google/Talven) often miss deeply nested academic papers or code repositories, or they return SEO-optimized blog spam. By querying Reddit, GitHub, or ArXiv *directly*, Searqon guarantees pristine, highly-relevant data from the source.

| Method | Endpoint | Source Domain / Purpose |
| :--- | :--- | :--- |
| `POST` | `/api/v1/sources/reddit` | **Reddit**: Scrapes actual human discussions and community consensus. |
| `POST` | `/api/v1/sources/github` | **GitHub**: Searches code, repositories, and exact README documentation. |
| `POST` | `/api/v1/sources/wiki` | **Wikipedia**: Extracts factual baseline knowledge. |
| `POST` | `/api/v1/sources/hackernew` | **Hacker News**: Retrieves tech-focused news and thread discussions. |
| `POST` | `/api/v1/sources/arxiv` | **ArXiv**: Quantitative science, physics, and computer science papers. |
| `POST` | `/api/v1/sources/pubmed` | **PubMed**: Certified medical and life-science journal search. |
| `POST` | `/api/v1/sources/openalex` | **OpenAlex**: Massive open-source catalog of global research. |
| `POST` | `/api/v1/sources/doaj` | **DOAJ**: Directory of Open Access Journals. |
| `POST` | `/api/v1/sources/medrxiv` | **MedRxiv**: Pre-print health sciences and clinical research. |
| `POST` | `/api/v1/sources/geeksforgeeks` | **GeeksforGeeks**: Tutorials, code snippets, and tech explanations. |
| `POST` | `/api/v1/sources/youtube` | **YouTube**: Retrieves video metadata, titles, and descriptions. |

---
*Note: All search endpoints exclusively accept a `query` parameter packed inside a JSON body payload.*
