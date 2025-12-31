# Searqon

**Searqon** is an open-source, self-hosted web intelligence engine that searches, crawls, extracts, ranks, and synthesizes information from the internet. Built with a microservice architecture, it provides full transparency and control over every step of the search and retrieval pipeline.

## What is Searqon?

Searqon is **not a chatbot**. It is the foundational search and web intelligence layer designed to power AI agents, RAG (Retrieval-Augmented Generation) systems, and knowledge-driven applications.

Think of it as a **transparent alternative to services like Tavily** — where you control the entire pipeline, from how sources are selected to how answers are synthesized.

## Why Searqon Exists

Most AI search APIs operate as black boxes:
- You don't control which sources are selected
- You don't see how content is extracted or cleaned
- You can't customize ranking or relevance algorithms
- You can't verify how answers are generated

**Searqon changes this.** Every component is transparent, modular, and customizable.

## What Searqon Does

Searqon processes web searches through five core stages:

1. **Search** — Queries multiple sources (Bing, DuckDuckGo, Reddit) and collects relevant URLs
2. **Crawl** — Loads webpages using real browsers to handle JavaScript-heavy sites
3. **Extract** — Removes noise (ads, navigation, scripts) and extracts clean, readable content
4. **Rank** — Uses semantic embeddings to filter and prioritize the most relevant information
5. **Synthesize** — Generates concise, citation-backed answers using an LLM

## Architecture

Searqon uses a **microservice architecture** where each service has a single, well-defined responsibility:

```
Client Request
      ↓
  API Gateway
      ↓
  Orchestrator
      ↓
  ┌───┴───┬─────────┬─────────┬──────────┐
  ↓       ↓         ↓         ↓          ↓
Search  Crawl   Extract    Rank    Synthesis
```

This design makes the system:
- **Scalable** — Services can be scaled independently
- **Maintainable** — Each service can be updated without affecting others
- **Extensible** — New features can be added without rewriting core logic
- **Testable** — Services can be tested in isolation

## Core Components

### API Gateway
Entry point for all requests. Handles validation, authentication, and rate limiting.

### Orchestrator
Coordinates the entire pipeline, deciding which services to call and in what order. Future versions will use LangGraph for more intelligent orchestration.

### Search Service
Fetches URLs from multiple search providers (Bing, DuckDuckGo, Reddit).

### Crawl Service
Loads webpages using Playwright, executing JavaScript for modern, dynamic websites.

### Extract Service
Parses HTML using Cheerio and extracts structured, readable text content.

### Rank Service
Uses embedding models to measure semantic relevance and filter low-quality content.

### Synthesis Service
Uses an LLM to generate final answers with enforced citations for transparency.

## Technology Stack

**Backend**
- Node.js & Express.js
- Playwright (browser automation)
- Cheerio (HTML parsing)
- Ollama (embeddings & LLMs)
- Docker & Docker Compose

**Frontend** (Demo UI)
- React
- Vite
- Tailwind CSS

## Use Cases

Searqon is designed for:
- **Learning** — Understanding how modern AI search systems work under the hood
- **RAG Systems** — Providing high-quality, cited web content for retrieval pipelines
- **AI Agents** — Serving as the web intelligence layer for autonomous agents
- **Research Tools** — Building custom search and synthesis workflows
- **Enterprise Applications** — Self-hosted search for sensitive or specialized domains

## Current Status

**What's Working:**
- Single-query web search with citation-backed answers
- Multi-source crawling and extraction
- Semantic ranking and content filtering

**What's Coming:**
- LangGraph-based orchestrator for agent-like behavior
- Conversational memory and multi-turn queries
- Autonomous retry logic and self-reflection
- Advanced caching and optimization

## Project Philosophy

> **Build the system first. Add intelligence later.**

Searqon prioritizes transparency, modularity, and control. Every decision in the pipeline is visible and customizable — no black boxes, no magic.

## Getting Started

*(Instructions for setup and usage will go here)*

## Roadmap

**Phase 1** — Core Engine
- Stable microservices
- Reliable search, crawl, extract, rank, synthesize pipeline
- Citation-based answer generation

**Phase 2** — Intelligent Orchestration
- LangGraph integration
- Agent planning and self-reflection
- Multi-turn conversational interface

**Phase 3** — Enterprise Features
- Advanced caching strategies
- Queue-based job processing
- Multi-user management
- API analytics and monitoring

## Contributing

Searqon is open source and contributions are welcome. Whether you're improving extraction logic, adding new search providers, or enhancing the synthesis quality — your input helps make Searqon better for everyone.

## License

MIT License

Copyright (c) 2026 Sooriya B

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
