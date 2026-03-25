# Searqon

**Searqon** is an open-source, self-hosted web intelligence engine that searches, crawls, extracts, ranks, and synthesizes information from the internet. Built with a high-performance Go-based extraction engine and a Node.js orchestration layer.

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

## What Searqon Does

Searqon processes web searches through five core stages:

1. **Search** — Queries multiple sources (Bing, DuckDuckGo, Reddit, GitHub) and collects relevant URLs.
2. **Crawl** — Fetches webpage content efficiently using a concurrent **Go-based scraper**.
3. **Extract** — Removes noise (ads, navigation, scripts) and delivers clean, flattened plain text.
4. **Rank** — Uses semantic embeddings to prioritize the most relevant information.
5. **Synthesize** — Generates concise, citation-backed answers using an LLM.

## Architecture

Searqon uses a **microservice architecture**. The extraction layer is powered by a Go-based microservice that leverages **Goroutines** for true parallel scraping, ensuring extremely low latency and memory usage.

## Technology Stack

- **Backend**: Node.js & Express.js
- **Extraction Engine**: Go 1.22+ (GoQuery & Go-Readability)
- **Orchestration**: Node.js + Go (via HTTP microservice)
- **LLM/Embeddings**: Ollama (Interchangeable)
- **Configuration**: YAML-based settings

## Getting Started

To get started with Searqon, please follow the detailed [Setup Guide](./docs/setup.md).

## Project Philosophy

> **Build the system first. Add intelligence later.**

Searqon prioritizes transparency, modularity, and control. Every decision in the pipeline is visible and customizable — no black boxes, no magic.

## Roadmap

**Phase 1** — Core Engine (Completed)
- Stable microservices & Unified Search
- High-performance **Go-based** parallel scraping engine
- Citation-backed answer generation

**Phase 2** — Intelligent Orchestration (In Progress)
- LangGraph integration for complex reasoning
- Agent planning and self-reflection
- Multi-turn conversational interface

## License

MIT License. See [LICENSE](LICENSE) for details.
