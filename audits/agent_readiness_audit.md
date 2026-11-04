# Searqon AI Agent Readiness Audit Report

**Target Repository:** Searqon (`/home/sooriya/Documents/Searqon` & `/media/workspace/barbara-workspace/Searqon`)  
**Auditor:** Pi (Oracle Execution Runtime) & Barbara Vane  
**Date:** November 2026  
**Scope:** Architecture, Model Context Protocol (MCP) server, 4-tier anti-bot scraping pipeline, Lightpanda/Camoufox hooks, crawler engine, meta-search RRF ranking, data structures, error resilience, and agentic workflows.

---

## 1. Executive Summary & Agent Readiness Score

### **Overall Agent Readiness Score: 87 / 100 (Grade: A- / Production-Ready with Gaps)**

Searqon represents a breakthrough in high-performance, ultra-lightweight (~8MB zero-CGO Go binary) web search and anti-bot scraping infrastructure tailored for AI agents (Claude Desktop, Cursor, Windsurf, Hermes, Oracle). Its native Model Context Protocol (MCP) implementation (`src/mcp/server.go`) combined with a 4-tier anti-bot escalation pipeline (`Fast HTTP` $\to$ `Spoofed Fingerprint` $\to$ `Lightpanda Headless Browser` $\to$ `Proxy`) positions it far ahead of legacy Python scrapers or heavy Node.js automation suites.

However, when evaluated strictly from the perspective of **autonomous multi-step reasoning agents** (such as Hermes/Oracle agent loops executing thousands of tool calls with strict latency, token budget, and verification constraints), several critical architectural gaps remain:
1. **Lack of Token-Stream Search:** Search and research return monolithic markdown blobs rather than incremental, chunked, or streaming JSON arrays.
2. **Missing Cryptographic / Provenance Citation Hooks:** Scraped documents lack verifiable source hash seals, exact fetch timestamps, and canonical citation ID tags required for strict grounding.
3. **Transport Resilience & JSON-RPC Error Isolation:** In `stdio` mode, unhandled panics or downstream provider timeouts can occasionally desynchronize JSON-RPC framing or return ambiguous `-32603` internal errors without recovery hints.
4. **Asynchronous Crawler Backpressure:** The recursive crawler and sitemap mapper (`src/crawler/crawler.go`) lack strict rate limiting per host domain and dynamic concurrency throttling, risking accidental denial-of-service or connection pool exhaustion under deep agent exploration.

---

## 2. Critical Gaps: What AI Agents Need vs. Searqon Current State

| Dimension | AI Agent Requirement (Barb / Oracle / Hermes) | Searqon Current Implementation | Gap & Impact |
| :--- | :--- | :--- | :--- |
| **Streaming / Incremental Output** | Stream search results and scraping progress incrementally to prevent LLM timeout on large payloads. | Monolithic JSON response returning full payload at once. | High latency on deep research queries; potential timeout on slow target sites. |
| **Citation Grounding & Provenance** | Strict citation anchors (`[source_id]`, SHA-256 content hash, exact retrieval UTC). | Markdown links with titles and URLs. | Risk of agent hallucination or stale cached content being cited without freshness proof. |
| **Structured Schema Contracts** | Strongly typed JSON schemas for tool arguments and responses with strict validation. | Flexible `map[string]interface{}` and dynamic JSON unmarshaling. | Prone to type assertion errors or malformed tool arguments from less disciplined LLMs. |
| **Anti-Bot Fallback Transparency** | Detailed telemetry on why escalation occurred (e.g. Cloudflare JS challenge detected at Tier 1, resolved at Tier 3). | `EscalationTier` string field in scrape result. | Good, but lacks structured error codes for bot challenge types (e.g. CAPTCHA vs 403 vs RateLimit). |
| **Domain Politeness & Rate Limits** | Autonomous respect for crawl delay, robots.txt, and concurrency budgets per target domain. | Global robots.txt cache and basic goroutine worker pool. | Missing per-domain token bucket ratelimiter during deep crawling (`searqon_map` / `searqon_research`). |

---

## 3. Deep Technical Findings & Recommendations

### A. MCP Server & Transport Layer (`src/mcp/server.go`)
* **Current State:** Implements JSON-RPC 2.0 over `stdio` and HTTP SSE/POST. Exposes 5 core tools (`searqon_search`, `searqon_scrape`, `searqon_research`, `searqon_map`, `searqon_extract`).
* **Vulnerabilities / Friction Points:**
  1. **Error Propagation:** When an underlying tool fails (e.g., DNS lookup failure or upstream SearXNG timeout), the server wraps it in `CallToolResult{IsError: true}` but returns JSON-RPC `code: 0` (success status at the transport layer). Standard MCP agents expect strict JSON-RPC error objects (`code: -32603`) for tool execution crashes so they can trigger self-correction loops.
  2. **Context Cancellation:** Handlers do not propagate the incoming request's context (`r.Context()`) down into `search.RunSearchPipeline` or `scraper.ScrapeSingleURLWithOptions`, meaning cancelled client requests continue burning server CPU and network bandwidth.

### B. Anti-Bot Scraper & Lightpanda Integration (`src/scraper/`)
* **Current State:** 4-tier escalation engine (`Fast HTTP` $\to$ `Spoofed Chrome/TLS` $\to$ `Lightpanda Headless` $\to$ `Proxy`).
* **Vulnerabilities / Friction Points:**
  1. **Lightpanda Binary Path Dependency:** Relies on the local Lightpanda binary (`lightpanda/lightpanda` or system PATH). If binary execution fails or hangs, fallback timeout handling can stall goroutines.
  2. **Memory Leak / Zombie Processes:** Headless browser subprocess invocation must guarantee signal cleanup (`os.Kill` / context timeout kill) to prevent orphan processes accumulating in long-running agent daemon mode.

### C. Search & Meta-Search Ranking (`src/search/`)
* **Current State:** Aggregates SearXNG, DuckDuckGo, Wikipedia, Arxiv using Reciprocal Rank Fusion (RRF).
* **Vulnerabilities / Friction Points:**
  1. **Query Intent Classification:** Basic keyword intent detection (`intent.go`). Agents performing multi-faceted research often require multi-query expansion (sub-queries generated automatically for different facets of a complex prompt). Searqon executes the exact query string provided without internal semantic query decomposition.

---

## 4. Actionable Engineering Roadmap (Must Fix, Should Fix, Agentic Enhancements)

### Phase 1: Must Fix (Immediate Reliability & Safety)
1. **Context Propagation:** Pass `ctx` through all MCP handlers into scraper and search pipelines to respect agent timeouts and cancellations.
2. **JSON-RPC Error Codes for Tool Failures:** Return proper JSON-RPC error codes (`-32603` Internal Error) when tool execution fails fundamentally, allowing agent agentic retry loops to catch tool faults cleanly.
3. **Process Supervision:** Ensure Lightpanda and HTTP scraper subprocesses have strict execution timeouts and PID cleanup routines.

### Phase 2: Should Fix (Performance & Observability)
4. **Per-Domain Rate Limiting:** Implement a token-bucket rate limiter per target domain in the crawler and scraper to prevent accidental rate-limiting or IP blacklisting during `searqon_map` and `searqon_research`.
5. **Enhanced Citation Metadata:** Include content content-hash (`sha256`), exact fetch timestamp (`RFC3339`), and HTTP status code in all markdown response headers for strict citation verification.

### Phase 3: Agentic Enhancements (Premier Capabilities)
6. **Multi-Query Decomposition:** Add automatic sub-query generation inside `searqon_search` and `searqon_research` when complex multi-faceted prompts are received.
7. **Streaming MCP Chunking:** Support chunked transfer encoding for large research dossiers so agents can begin parsing headers and early sources while subsequent sources are still scraping.

---

## 5. Conclusion
Searqon is already a high-performance, elegantly engineered search and scraping engine. Implementing the Phase 1 and Phase 2 recommendations outlined in this audit will elevate it to a bulletproof, zero-friction foundational layer for autonomous agent ecosystems like Oracle, Hermes, and Claude.
