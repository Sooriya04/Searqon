# Searqon API V2 Endpoints Reference

## Overview

Searqon is a self-hosted web intelligence engine designed for AI systems. It provides the foundational discovery, extraction, and synthesis layers required to power AI agents, RAG pipelines, and knowledge-driven applications. This API is versioned under `/api/v2`.

## Design Philosophy

The V2 API architecture strictly separates atomic data tools from high-level orchestrations. By decentralizing these concerns, developers can easily piece together basic low-level primitives (`/scrape`, `/search`) to build custom pipelines, or utilize our powerful orchestration engine (`/research`) for a complete, end-to-end extraction workflow. Searqon serves as a transparent, open-source alternative to opaque, proprietary AI search APIs.

## High-Level Orchestration Endpoint

Designed for maximum developer velocity. This endpoint handles the entire "query-to-synthesis" lifecycle in a single call.

| Method | Endpoint           | Description                                                                                                                                                                                                 |
| :----- | :----------------- | :---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST` | `/api/v2/research` | The unified extraction pipeline. Accepts a raw query, classifies the intent, performs multi-source discovery, fetches page content, extracts text, and ranks findings into a structured, LLM-ready payload. |

## Core Endpoints

Core primitives provide atomic, granular control over web scraping and sub-domain topology.

| Method | Endpoint            | Description                                                                                                                                     |
| :----- | :------------------ | :---------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST` | `/api/v2/search`    | **Discovery**. Returns a ranked array of highly relevant URLs, metadata, and SERP snippets without executing deep HTML scraping.                |
| `POST` | `/api/v2/scrape`    | **Extraction**. Converts a specific target URL into pristine, high-fidelity Markdown. Includes dynamic fallback rendering for JS-reliant pages. |
| `POST` | `/api/v2/crawl`     | **Recursion**. Initiates an autonomous, rate-limited crawl of a domain. Returns all processed pages as an aggregated Markdown dataset.          |
| `GET`  | `/api/v2/crawl/:id` | **State Tracking**. Fetches the execution status and results of a background crawl job.                                                         |
| `POST` | `/api/v2/map`       | **Topology**. Spiders a target domain to rapidly map and expose all valid internal URLs.                                                        |
| `POST` | `/api/v2/extract`   | **Structuring**. Passes scraped Markdown payload to an LLM provider to extract structured JSON entities conforming to a defined schema.         |

_Note: Searqon's `/classify` intent-routing mechanism runs natively to drive the `/research` endpoint, but is kept internal to minimize public API complexity._

## Specialized Source Endpoints

For complex technical or niche queries, normalized web indices often return SEO-optimized noise. These endpoints bypass general search to query specialized platforms directly, fetching high-signal, source-aware data.

| Method | Endpoint                     | Description                                                                          |
| :----- | :--------------------------- | :----------------------------------------------------------------------------------- |
| `POST` | `/api/v2/sources/reddit`     | Retrieve authentic community consensus, forum threads, and sentiment.                |
| `POST` | `/api/v2/sources/github`     | Search source code, repository metadata, and project documentation directly.         |
| `POST` | `/api/v2/sources/wikipedia`  | Extract verified baseline factual knowledge and entity summaries.                    |
| `POST` | `/api/v2/sources/hackernews` | Retrieve high-signal technical thread discussions and article metadata.              |
| `POST` | `/api/v2/sources/arxiv`      | Search and extract abstracts from pre-print quantitative and computational sciences. |
| `POST` | `/api/v2/sources/pubmed`     | Retrieve certified biological, medical, and life-science journal abstracts.          |
| `POST` | `/api/v2/sources/youtube`    | Extract video metadata, titles, descriptions, and available transcriptions.          |

## Suggested Future Endpoints

The following specialized sources are categorized for planned future integration as the sources marketplace expands:

- **Academics & Research:** OpenAlex, DOAJ, MedRxiv
- **Technical Documentation:** StackOverflow, GeeksforGeeks
- **Financial & Enterprise:** SEC EDGAR, Crunchbase
