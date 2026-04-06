# 🚀 Searqon API V1 Endpoints Reference

All endpoints are versioned under `/api/v1`.

## 🛡️ Core Engine
These endpoints provide the primary scraping, crawling, and AI extraction capabilities.

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/api/v1/scrape` | Scrape a single URL into Markdown or Text. |
| `POST` | `/api/v1/crawl` | Async crawl of an entire domain. Returns `jobId`. |
| `GET` | `/api/v1/crawl/:id` | Get status and results for a crawl job. |
| `POST` | `/api/v1/map` | Discover all URLs on a given domain. |
| `POST` | `/api/v1/extract` | Schema-based structured JSON extraction via LLM. |

## 🔍 Global Search
Unified search across web and specialized sources.

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/api/v1/search` | General web search (DuckDuckGo based). |
| `POST` | `/api/v1/unified` | Multi-source unified search hub. |
| `POST` | `/api/v1/classify` | AI-powered query intent classification. |

## 📚 Specialized Search Sources
Access specific data pools directly under the sources marketplace.

| Method | Endpoint | Source |
| :--- | :--- | :--- |
| `POST` | `/api/v1/sources/reddit` | Reddit Community Search |
| `POST` | `/api/v1/sources/github` | GitHub Code & Repo Search |
| `POST` | `/api/v1/sources/wiki` | Wikipedia Knowledge Search |
| `POST` | `/api/v1/sources/hackernew` | Hacker News Discussions |
| `POST` | `/api/v1/sources/arxiv` | ArXiv Research Papers |
| `POST` | `/api/v1/sources/pubmed` | PubMed Medical Journals |
| `POST` | `/api/v1/sources/openalex` | OpenAlex Global Research |
| `POST` | `/api/v1/sources/doaj` | Directory of Open Access Journals |
| `POST` | `/api/v1/sources/medrxiv` | MedRxiv Health Sciences |
| `POST` | `/api/v1/sources/geeksforgeeks` | GeeksforGeeks Tech Articles |
| `POST` | `/api/v1/sources/youtube` | YouTube Video Search |

---
*Note: All search endpoints predominantly accept a `q` (query) or `query` parameter in a JSON body.*
