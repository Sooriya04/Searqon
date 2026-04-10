# Searqon

**Searqon** is an open-source, self-hosted web intelligence engine that searches, crawls, extracts, ranks, and synthesizes information from the internet. Built with a high-performance Go-based extraction engine and a native Node.js orchestration layer.

## What is Searqon?

Searqon is **not a chatbot**. It is the foundational search and web intelligence layer designed to power AI agents, RAG (Retrieval-Augmented Generation) systems, and knowledge-driven applications.

Think of it as a **transparent alternative to services like Tavily** — where you control the entire pipeline, from how sources are selected to how content is extracted or cleaned.

## Why Searqon Exists

Most AI search APIs operate as black boxes:
- You don't control which sources are selected
- You don't see how content is extracted or cleaned
- You can't customize ranking or relevance algorithms
- You can't verify how answers are generated

**Searqon changes this.** Every component is transparent, modular, and customizable.

## Searqon processes web searches through five core stages:

1. **Route** — Classifies the query using a native **Semantic Intent Engine** (weighted scoring, no LLM) to identify relevant domain sources in under 1ms.
2. **Search** — Queries specialized sources (GitHub, PubMed, Reddit, YouTube, etc.) based on intent and always includes DuckDuckGo as a baseline.
3. **Crawl** — Fetches webpage content efficiently using a concurrent **Go-based scraper** with parallel deep extraction per source.
4. **Extract** — Removes noise (ads, navigation, scripts) and delivers clean, flattened plain text or high-fidelity Markdown.
5. **Synthesize** — Provides raw, structured JSON intelligence directly to the client — optional synthesis can be performed client-side or via integrated LLM streaming.

## Architecture

Searqon uses a efficient **2-process architecture**:

- **Node.js (Port 3001)** — API orchestration, native semantic routing, and response assembly.
- **Go (Port 3002)** — High-performance parallel scraper binary using Goroutines.

**Total idle RAM: ~50MB.** Optimized for bare-metal and resource-constrained environments.

## Technology Stack

- **Backend**: Node.js & Express.js
- **Intelligent Routing**: Native JavaScript (Semantic Intent Engine, < 1ms)
- **Extraction Engine**: Go 1.22+ (GoQuery & Go-Readability)
- **Orchestration**: Node.js + Compiled Go Binary
- **Configuration**: YAML-based settings

## Getting Started

To get started with Searqon, please follow the detailed [Setup Guide](./docs/setup.md).

## Project Philosophy

> **Build the system first. Add intelligence later.**

Searqon prioritizes transparency, modularity, and control. Every decision in the pipeline is visible and customizable — no black boxes, no magic.

## Roadmap

**Phase 1** — Core Engine (Completed)
- High-performance **Go-based** parallel scraping binary
- Native JS-based intent classification
- Unified Search API (JSON-only mode)

**Phase 2** — Intelligent Orchestration (In Progress)
- Agent planning and self-reflection
- Multi-turn conversational interface
- Structured Knowledge Panel extraction

## License

MIT License. See [LICENSE](LICENSE) for details.
