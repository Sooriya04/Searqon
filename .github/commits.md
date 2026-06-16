## ISSUSE 1 : DuckDuckGo Search Microservice

Added a DuckDuckGo search microservice as the first component of the Searqon pipeline. It accepts search queries via a REST endpoint, performs live DuckDuckGo searches, and parses HTML responses using Cheerio. Results are normalized into a simple JSON format with titles, URLs, and short snippets. The service is scoped strictly to search previews and is designed to plug into future crawler and extractor services.

## ISSUSE 2 : Reddit Search Microservice

Added a Reddit search microservice to collect discussion-based signals for Searqon. It accepts Reddit-style queries and fetches results from Reddit’s public JSON search endpoint. Responses are normalized with post metadata such as title, subreddit, score, timestamp, and permalink. The service focuses only on discovery and opinion signals, not full comment extraction.

## ISSUSE 3 : Wikipedia Extractor Microservice

Added a Wikipedia extractor microservice to retrieve full article content for URLs discovered during search. It fetches Wikipedia HTML pages and uses Cheerio to extract the main article body while ignoring non-content elements. The service returns clean, structured content and is scoped solely to extraction, enabling future processing such as sectioning and synthesis.

## ISSUSE 4 : GitHub Search with README Support

Updated the GitHub search service to fetch the top 10 matching repositories for a query and include each repository’s `README.md` in the response. Results now return basic repo metadata along with raw README content, providing clearer project context while keeping the service search-focused.

## ISSUSE 5 : Search & Crawl Pipeline

Implemented Bing and DuckDuckGo search scraping to discover URLs, followed by full webpage crawling and plain-text extraction using Cheerio. Added a hardened HTTP client to handle bot responses and improve Bing reliability. All non-content elements (scripts, nav, ads) are removed, and main article content is extracted with safe fallbacks, returning clean, engine-agnostic text for downstream processing.

## ISSUSE 6 : added multiple search

### ISSUSE 15 : centralize all scraping into worker-pool Scrapper module

Refactored the application’s scraping architecture by consolidating all web extraction logic into a centralized, worker-pool-based Scrapper module. Updated DuckDuckGo, HackerNews, Wikipedia, PubMed, and Reddit services to delegate scraping to this system, eliminating duplicated, service-specific implementations (e.g., direct Cheerio usage) and enforcing a standardized output format. Enhanced the Reddit scraper with browser-grade headers to bypass API blocking and expanded extraction to include full HTML bodies and external link metadata, enabling richer and more reliable content ingestion across the platform.

### ISSUSE 16 : OpenAlex Search & URL Discovery

Implemented OpenAlex search scraping to discover academic work URLs without relying on API keys. Added HTML-based query handling to extract canonical work links, enabling downstream crawling via the existing scraper pool. The service returns URL-only results, deferring full content extraction to the unified scraping pipeline. This extends academic coverage beyond Semantic Scholar while preserving a clean separation between discovery and extraction layers.

### ISSUSE 17 : add persistent thread pool and undici connection pooling for high-performance scraping

implement high-performance scraping engine with persistent thread pool and connection pooling — refactor the scraper architecture by replacing the previous BullMQ-based synchronous worker model with a persistent worker_threads pool to eliminate per-request thread spawn overhead and enable zero-latency CPU parsing, introduce undici.Pool for true keep-alive connection reuse and optimized network throughput, create a centralized ScrapUrl module to serve as the direct bridge between services and the scraping engine, resolve functional issues including missing result pushes in DuckDuckGo and API integration inconsistencies in HackerNews and Wiki, and improve overall performance by auto-scaling concurrency based on available CPU cores (N-1) to maximize parallelism while maintaining system stability

### ISSUSE 18 : replace undici Pool with native fetch; prefer OA URLs

The redirect error was resolved by replacing the undici Pool.request()–based fetcher with Node.js’s built-in fetch() API, which natively follows redirects and avoids deprecated options such as maxRedirections removed in newer undici versions. After this fix, DOI redirects function correctly; however, 403 responses from publisher sites (e.g., Taylor & Francis) are not code defects but the result of publisher-side bot detection. To address this at an architectural level, the fetcher now uses more realistic browser headers, and the OpenAlex service prioritizes open-access URLs (including OA PDFs) over DOI or publisher links. This ensures reliable, legal scraping by favoring publicly accessible sources and treating paywalled publisher URLs as a last-resort metadata reference rather than primary scrape targets.

### ISSUSE 19 : implemented DOAJ search with text sanitization

The DOAJ integration was completed successfully, and results are now returned in the unified Searqon schema. During testing, minor formatting artifacts such as `\r`, `\n`, `\t`, and unnecessary whitespace were detected in certain metadata fields. A lightweight sanitization step was implemented to remove these control characters and normalize spacing before returning results. This ensures clean, consistent text output and prevents formatting noise from affecting indexing, word count calculations, or downstream processing.

### ISSUE 20: Implemented medRxiv service

The medRxiv service layer was implemented and integrated into the Searqon pipeline, responsible for constructing the search URL, invoking the centralized scraping engine, and returning results in the unified schema. A configurable limit parameter was introduced to control the number of similar results returned per request, with validation and normalization enforcing a safe range (minimum 5, maximum 10) and a default fallback of 5 when unspecified or invalid. This ensures predictable payload size, consistent response behavior, and improved stability for downstream aggregation and processing without modifying the underlying scraping infrastructure.

### ISSUE 21: Implemented Unified Search Controller

The unified search controller was implemented to orchestrate concurrent queries across ten distinct search services, including arXiv, PubMed, and OpenAlex. By utilizing `Promise.allSettled`, the architecture ensures system resilience, allowing successful retrieval of partial results even if individual upstream providers fail or timeout. A normalization layer was introduced to standardize diverse response structures into a consistent schema, while a customizable limit parameter controls the volume of aggregated data. This enhancement provides a single, robust entry point for comprehensive academic and web search results without compromising performance.

### ISSUE 22: Enhanced Text Cleaning for Arxiv and Reddit Services

The Arxiv and Reddit search services were updated to utilize a centralized cleanText utility, addressing data quality issues such as extraneous whitespace, tab characters, and newline inconsistencies. This enhancement ensures that titles, summaries, and scraped content are sanitized before being returned, providing a cleaner and more readable output structure for the unified search controller. This standardization improves the overall data integrity of the aggregated results without altering the underlying extraction logic

### ISSUE 23: Migration to Puppeteer-based Scraping Architecture

The scraping mechanism has been completely re-architected to use Puppeteer, replacing the previous fetch + worker_threads implementation. This migration enables the system to handle dynamic JavaScript-heavy websites, bypass simple bot protections via puppeteer-extra-plugin-stealth, and manage concurrency more effectively through a dedicated Limiter utility. Additionally, a new web service was introduced to allow direct URL scraping, and precise timing logs (start, end, duration) were added to the scraper output for better observability. These changes significantly enhance the robustness and capabilities of the extraction engine while maintaining the existing unified search interface.

### ISSUE 24: Implemented GeeksforGeeks Federated Service

The GeeksforGeeks service was integrated into the Searqon pipeline as a federated search module using DuckDuckGo query augmentation (<query> geeksforgeeks) rather than directly accessing restricted endpoints. Results are filtered to retain only geeksforgeeks.org URLs, with a fallback to the site: operator when necessary, and mapped into the unified schema with a normalized "geeksforgeeks" source tag. A dedicated content sanitization layer removes UI noise and footer artifacts to ensure clean, relevant output, while configurable result limiting and strict controller-level validation maintain predictable payload size, stable aggregation behavior, and consistency with existing service implementations.

### ISSUE 25: Implemented Performance Optimizations and Centralized Configuration

The Searqon scraper pipeline was optimized to resolve execution bottlenecks by transitioning from sequential to parallel service-level scraping using Promise.all across eight federated modules (Reddit, OpenAlex, DuckDuckGo, Arxiv, MedRxiv, PubMed, DOAJ, and GitHub). A centralized configuration layer was introduced via settings.yaml and configLoader.js, allowing for dynamic adjustment of browser parameters, resource-blocking rules, and concurrency limits without additional code changes. This upgrade increased the global concurrency threshold from 3 to 10 active sessions via Limiter.js, significantly improving throughput and reducing aggregation latency. Furthermore, the system's stability was bolstered by resolving critical import regressions and correcting service remapping logic to ensure consistent, filtered result output across the unified schema.

### ISSUE 26 : migrate to auto-scaling Python microservice architecture

Replace legacy Node.js Puppeteer scraper with distributed Python-based asynchronous crawler. Introduce persistent aiohttp API with session pooling and integrated autoscaling. Refactor ScrapUrl bridge into lightweight HTTP client and reorganize project into modular crawler/src structure. Update local orchestration and documentation to reflect new hybrid scraping architecture.

### ISSUE 27 : upgrade to high-performance Scrapling-based extraction layer

Replace legacy BeautifulSoup parsing with Scrapling’s lxml-backed CSS selector engine to significantly improve extraction speed and accuracy. Introduce a hardware-aware architecture optimized for 4GB RAM systems by reducing concurrency and implementing a hybrid pipeline that uses aiohttp for ultra-fast network fetching and Scrapling for efficient HTML parsing. Eliminate latency bottlenecks and hanging requests by refining Node.js–Python interprocess communication and removing redundant retry stacking, ensuring fast-fail behavior. Enhance data quality by transitioning from snippet-based scraping to full-page visible text extraction with intelligent filtering to strip scripts, styles, and navigation noise. Harden external integrations such as MedRxiv and DuckDuckGo with extended timeouts and improved reliability handling. Finalize with a full codebase cleanup, removing deprecated BeautifulSoup modules and temporary scripts to deliver a lean, production-ready extraction system.

### ISSUE 28 : enhance DuckDuckGo reliability and parallel scraping performance

Introduce a Puppeteer Stealth fallback in DuckDuckGo service to bypass CAPTCHA blocks and ensure consistent query success. Optimize response time by limiting full-page scraping to the top 2 results while returning instant snippets for others. Redesign the scraping pipeline with a two-tier strategy: attempt Python-based crawling with a strict timeout, then fall back to a fast Node.js Cheerio extractor to prevent hanging requests. Enable true parallel resolution of results to eliminate bottlenecks. Add a native GitHub interceptor using the GitHub API to fetch READMEs instantly, significantly reducing latency for repository links.

### ISSUE 29 : migrate to high-performance Go scraper microservice

Replace legacy Python crawler with a high-performance Go-based microservice running on port `3002` to eliminate bottlenecks in parallel scraping. Implement a dual-extraction pipeline using `go-readability` for semantic content parsing and `goquery` for precise DOM cleanup, removing ads, navigation elements, and other noise. Introduce concurrency using Goroutines with `sync.WaitGroup` to enable efficient parallel scraping of 10+ URLs with sub-second response times. Add manual Brotli, Gzip, and Deflate decoding to handle compressed responses and prevent binary output issues, along with Content-Type validation to skip non-text resources. Integrate an aggressive text normalization layer to strip newlines, Markdown artifacts, and special characters, producing clean flat text output. Refactor the Node.js integration via `ScrapUrl.js` into a lightweight HTTP bridge and remove the legacy Python crawler, resulting in a faster, memory-efficient, and scalable scraping architecture.

### ISSUE 30 : Updated project documentation

Updated the project's documentation files, including `readme.md`, `setup.md`, and `scraping.md`, to accurately reflect the changes made to the scraping architecture and setup requirements.

### ISSUE 31 : YouTube Search Service with Full Description Extraction

Implemented a dedicated YouTube search service that scrapes video metadata without an API key by parsing `ytInitialData`. Developed a robust, parallelized extraction strategy that retrieves full, un-truncated video descriptions directly from individual video pages, bypassing the limited snippets found in search results. Integrated a specialized YouTube text cleaner to strip hashtags, social media URLs, and promotional boilerplate, ensuring high-quality, sanitized content is delivered through the unified search controller.

### ISSUE 32 : Integrated Intelligent Query Classification System

Transformed Searqon from a static search aggregator into an intelligent search engine by introducing a query classification layer powered by a Python microservice. Implemented LLM-based intent detection using Ollama (`qwen2.5:0.5b`) with a robust keyword fallback mechanism and synonym normalization via an alias map.

Refactored the Node.js architecture to follow the Route → Controller → Service pattern by adding `classifierService.js` and `classifierController.js`, exposing a new `/api/classify` endpoint with proper validation.

Enhanced the search pipeline by implementing a two-phase retrieval strategy in `unifiedController.js`, prioritizing domain-specific sources (e.g., GitHub, PubMed) before falling back to DuckDuckGo for comprehensive coverage.

Improved system orchestration by updating `package.json` to enable unified startup of the Node API, Go scraper, and Python classifier via `npm run dev`. Also resolved Ollama SDK compatibility issues (v0.6) and optimized timeout handling for local LLM inference.

### ISSUE 33 : High-Performance Two-Phase Research Engine with Optimized Scraping Pipeline

Redesigned Searqon into a high-performance, intelligent research engine by introducing a two-phase search architecture. Built a lightweight Semantic Intent Engine in Python to instantly classify queries and route them to specialized data sources without relying on a local LLM daemon, significantly reducing latency. Optimized the scraping pipeline to extract full-page content only from the top 3 results per source, balancing depth with speed. Implemented strict 10-second per-source timeouts in the Node.js coordination layer to prevent slow or unresponsive APIs from blocking execution. Strengthened system reliability by enabling address reuse across microservices and eliminating zombie processes, resulting in a stable, multi-service stack that delivers faster, deeper insights at scale.

### ISSUE 34 : Zero-LLM High-Performance Research System with Optimized Resource Footprint

Re-architected Searqon into a Zero-LLM research system by eliminating the dependency on Ollama and external LLM daemons. Introduced a sub-millisecond Semantic Intent Engine for efficient query routing and implemented a custom TF-IDF Extractive Summarizer to generate research highlights without generative models. This transition significantly reduced the overall RAM footprint to under 150MB, improving system efficiency, lowering infrastructure costs, and enabling lightweight, scalable deployments while maintaining effective data synthesis.

### ISSUE 35 : High-Fidelity Markdown Extraction and Advanced Web Intelligence Capabilities

Re-architected Searqon into a high-performance, autonomous web-scraping and data-extraction engine with an AI-ready processing pipeline. Upgraded the core Go-based scraper with advanced Markdown extraction (html-to-markdown/v2) to produce clean, structured outputs for downstream consumption. Expanded the API layer to support recursive site-wide crawling (/api/crawl), instant site mapping (/api/map), and an advanced extraction workflow (/api/extract) capable of converting simple text queries into structured JSON by aggregating and processing top web results. Improved system reliability with automatic JavaScript rendering fallback using Puppeteer for dynamic content, and optimized the orchestration layer by replacing axios with Node’s native fetch API, reducing dependencies and improving performance. The system now functions as a complete "Second Brain" pipeline, capable of transforming any query or domain into clean, structured intelligence.

### ISSUE 36 : Real-Time RAG-Based Smart Answer System with Multi-LLM Synthesis

Implemented a real-time RAG-based smart answer system in Searqon that transforms search results into a single, concise AI-generated response. A new synthesizeAnswer engine aggregates context from top-ranked results and uses multiple LLMs (Gemini, OpenAI, Ollama) to generate accurate summaries. The search pipeline was enhanced with an additional AI synthesis phase, shifting the experience from fragmented snippets to a unified, intelligent answer for queries involving pricing, specs, and general knowledge.

### ISSUE 37 : Production-Grade Asynchronous API Architecture and Versioned Search Hub

Transformed the Searqon API from a flat, synchronous prototype into a production-grade asynchronous architecture versioned under /api/v1. Implemented a centralized routing hub and Job ID lifecycle for long-running tasks, moving from blocking requests to an efficient, scalable polling model with webhook support. Standardized the entire multi-source search marketplace by grouping specialized integrations under a unified /sources namespace and introduced a comprehensive endpoints.md reference. Additionally, hardened the AI extraction pipeline with a specialized "self-healing" JSON parser and standardized metadata responses, ensuring high-reliability structured data output across all search, crawl, and extraction endpoints.

### ISSUE 38 : Add SSE streaming, conversational search, and Python LLM proxy integration

Implemented real-time token streaming for LLM responses using Server-Sent Events via a new /api/search/stream endpoint to enable a live "typing" experience. Added conversational follow-up search support with dedicated chat routes and controllers to maintain context across queries. Replaced complex JavaScript SSE handling with a lightweight Python microservice (llm_stream.py) that efficiently streams responses from Gemini, OpenAI, and Ollama, with Node.js acting as a proxy layer. Integrated all endpoints into the main router and updated package scripts to orchestrate the full stack (Node server, Python streamer, Go scraper, and TF-IDF engine) through a unified startup command.

### ISSUE 39 : Optimize search latency, introduce async extraction pipeline, and enforce structured LLM outputs

Reduced meta-search latency by lowering engine timeouts from 15 seconds to 1.5 seconds, enabling near-instant browser results. Decoupled heavy scraping and extraction into an asynchronous background daemon that triggers alongside search requests and returns a summary_job_id for non-blocking frontend polling. Enhanced the extraction engine by enforcing a strict native JSON schema with high-detail output, minimizing hallucinations from local LLMs while preserving complete raw markdown for transparency and deeper user access.

### ISSUE 40 : Re-architect Searqon to Node.js + Go, eliminate Python, achieve sub-100MB footprint

Re-engineered Searqon into a streamlined, high-efficiency system by consolidating its multi-language architecture into a minimal 2-process model built on Node.js and Go. All Python dependencies were eliminated by migrating semantic intent classification and LLM streaming into native JavaScript, removing interpreter overhead entirely. Heavy dependencies like Puppeteer and Mongoose were decommissioned, and web extraction was rebuilt using a compiled Go-based scraper for near-zero runtime overhead. The Unified Search API was simplified to return raw JSON results, reducing processing latency, while system-wide optimizations brought response times under five seconds. With an integrated Go build pipeline and updated deployment workflow, the system is now optimized for fast, reliable execution in bare-metal environments with a total memory footprint under 100MB.

### ISSUE 41 : enable batch crawling support and align documentation with new 2-process architecture

Upgraded the crawling pipeline by enhancing crawlController.js to support high-speed batch processing, allowing the scrape endpoint to handle both individual URLs and arrays of links while returning results in a consistent, ultra-minimal flat JSON format aligned with the unified search engine. In parallel, performed a comprehensive overhaul of the entire documentation suite, including README.md, setup.md, and all supporting docs, to accurately reflect the new streamlined 2-process architecture built on Node.js and Go. All legacy references to deprecated Python microservices, outdated port configurations, and Puppeteer-based workflows were removed and replaced with updated specifications highlighting the system’s optimized ~50MB RAM footprint and sub-5-second performance benchmarks.

### ISSUE 42: integrate Talven meta-search, standardize query payloads, and add configurable search-layer toggle

Integrated Talven as the primary meta-search engine, replacing DuckDuckGo as the default general web source and enhancing search coverage. Standardized all search endpoints to exclusively accept a `query` parameter within a JSON body payload, ensuring consistent request formatting. Added a new configuration option in `settings.yaml` to enable or disable the Talven provider, providing users with flexibility to control search sources while maintaining backward compatibility.

### ISSUE 43: integrate Talven meta-search, standardize query payloads, and add configurable search-layer toggle

Upgraded Searqon into a more resilient and modular extraction pipeline by adding Talven as a parallel meta-search provider alongside DuckDuckGo. Implemented a new `provider/talven.js` module, introduced intelligent result merging and deduplication across routing controllers, and ensured that higher-quality URLs are passed to the Go Scraper. Standardized the codebase by replacing `q` with `query` across request payloads for consistency, and improved architectural flexibility by documenting the flow in `endpoints.md` and adding a dynamic `settings.yaml` toggle to enable or disable the Talven search layer without code changes.

### ISSUE 44: introduce modular V2 API architecture with unified research endpoint alongside legacy V1 routes

Redesigned and implemented a production-ready V2 API architecture for Searqon while preserving full backward compatibility with the existing V1 system. The work began with a careful analysis of the original API surface to identify overlapping responsibilities, bloated endpoint behavior, and opportunities for cleaner separation of concerns. Based on that, a modular V2 structure was designed to clearly distinguish low-level data primitives such as scraping and mapping from higher-level orchestration logic, highlighted by the introduction of a unified /research endpoint for end-to-end AI agent workflows. This new architecture was formally documented in docs/v2_endpoints.md using an open-source-friendly format, then integrated into the backend through a dedicated routes/v2.js router and safe mounting changes in app.js, allowing the new V2 endpoints to run cleanly in parallel with the legacy V1 routes.


### ISSUE 45: Re-engineer scraping engine for high-performance crawling and low-memory search

Searqon was re-engineered by replacing the memory-heavy Puppeteer and Chromium scraping stack with a lightweight Go-based static scraping engine, significantly improving performance and reducing resource consumption. The architecture was modularized into dedicated components for handlers, crawler queues, and robots.txt compliance to improve maintainability and scalability. A memory-efficient Server-Sent Events (SSE) streaming API was implemented for real-time site-wide crawling while operating under a strict 100MB RAM limit. The search and unified routes were refactored to separate query discovery from scraping, enabling controllers to fetch raw snippets from multiple search providers in parallel, deduplicate and rerank top candidates, and execute a single parallelized batch scraping pipeline via Go with aggressive timeout controls.


### ISSUE 46: migrate to PostgreSQL cache, optimize orchestration, and enhance robots compliance

Searqon was re-architected by migrating the persistence layer from Mongoose/MongoDB to a self-initializing PostgreSQL connection pool designed for high-performance search caching through automatically created tables and optimized indexes. The Node.js orchestration layer was streamlined by removing Cheerio DOM parsing overhead and legacy JavaScript search utilities, shifting all heavy content extraction responsibilities to a concurrent Go daemon while replacing axios with native fetch using AbortSignal-based timeout management. Deprecated helper files were removed, .gitignore was updated to exclude local agent and graphify directories, and a robust robots.txt auditing system was implemented in Go to respect crawl-delay directives and dynamically rotate HTTP User-Agent identities between major search crawlers such as Googlebot, Bingbot, and Yandexbot based on path permissions. The unified controller was further optimized through PostgreSQL-backed query caching, enabling instant cache lookups for repeated searches, live scraping only on cache misses, and background persistence to deliver sub-millisecond response times for subsequent requests.


### ISSUE 47: Re-implement Searqon as unified Go application, optimize search fallback, and add documentation

Searqon was re-architected into a single, high-performance Go application listening on port 4001, completely deprecating the Node.js orchestration layer and eliminating node_modules overhead. The search discovery pipeline was restructured around a resilient fallback chain, using a local SearXNG microservice as the primary aggregator with automatic fallback to DuckDuckGo Lite HTML parsed with goquery to avoid JavaScript-obfuscated API endpoints. Scraping latency was optimized by executing parallel page fetches capped at the top three search results, governed by a global 2.5-second context deadline that falls back to search snippets for slow or blocked pages. Project orchestration was improved by adding a multi-stage Dockerfile, a profile-configured docker-compose.yml, a CI workflow for automated testing, and a Makefile for lifecycle management. Finally, complete technical documentation was created detailing the system architecture, API endpoints, providers, and configurations.


### ISSUE 48: Implement PostgreSQL caching, modularize Docker structure, and optimize search pipeline

Searqon was optimized by implementing a native PostgreSQL caching layer in Go using pgxpool for connection pooling, enabling automatic table initialization and graceful degradation to a no-cache mode when the database is offline. The directory structure was refactored by renaming go_scraper to src and updating all associated project configurations and tools. Containerization was modularized by moving build files to docker/app and docker/database subdirectories, introducing an init.sql schema setup script, and updating docker-compose.yml with persistent volumes. SearXNG integration was resolved by deploying a custom settings.yml config that maps the json format search output to port 4002, resolving 403 Forbidden errors. Finally, caching lookup checks and URL deduplication were integrated into the core Go pipeline, saving successfully scraped markdown elements and reducing subsequent query times from seconds to sub-15 milliseconds.


### ISSUE 49: Implement cache TTL eviction, cache-bypass controls, Jina Reader compatibility endpoint, and environment-based configuration


Searqon was hardened and extended across four areas. Cache lifecycle management was introduced by adding configurable TTL policies (SEARCH_CACHE_TTL_HOURS defaulting to 24 hours, SCRAPE_CACHE_TTL_DAYS defaulting to 7 days) read from environment variables, with both retrieval functions updated to filter expired entries inline via SQL timestamp comparison. A background goroutine worker was added that runs hourly to hard-delete stale rows from both cache tables and shuts down cleanly via context cancellation when the server exits. Cache bypass support was added across all scraping and search endpoints by introducing a bypass_cache boolean field in SearchRequest, ScrapeRequest, and BatchScrapeRequest, propagated through runSearchPipeline and scrapeSingleURL to skip cache reads while still writing fresh results back to the database. A Jina Reader-compatible GET /r/<url> endpoint was implemented, supporting both path-style and query-style URL input, returning raw text/markdown by default and a structured JSON envelope when the Accept: application/json header or json=true query parameter is set. Project configuration was improved by creating a .env file for environment-based secrets, updating the Makefile to auto-load and export all .env variables on every make run, printing the active SearXNG URL at startup, and adding an auto-kill step so repeated make run calls never fail with address-already-in-use errors. The make build target was updated to compile a self-contained binary directly to bin/searqon. API documentation was updated to reflect all new fields and the new endpoint.


### ISSUE 50: Implement Lightpanda headless browser integration, local config toggle, port mapping, and comprehensive pipeline documentation


Searqon's infrastructure, scraping capabilities, and documentation coverage were finalized. Infrastructure conflicts were resolved by re-mapping PostgreSQL to port `5433` in `docker-compose.yml` and updating `DATABASE_URL` in `.env`. SearXNG rate limits were bypassed by disabling its internal limiter (`limiter: false` in `docker/searxng/settings.yml`) and re-mapping port `8080`. The Lightpanda headless browser was integrated to handle dynamic client-side JavaScript pages. A `lightpanda/` directory was created, Git-ignored, and configured with a local `config.yaml` toggle. A zero-dependency YAML config loader was implemented in `src/utils.go` with relative path resolution. The scraper in `src/scraper.go` was updated to execute Lightpanda using `exec.CommandContext` with a 7-second timeout, dynamically waiting until `networkalmostidle` to minimize latency, and falling back gracefully to the native Go HTTP client on failure. An `install-lightpanda` target was added to the `Makefile` to automate binary installations. Finally, the documentation was fully restructured: `docs/workflow/workflow.md` maps request lifecycles and fallback chains, `docs/workflow/scraping.md` lists the 7-stage scraping pipeline, `docs/architecture.md` documents database DDL schemas and configurations, and `docs/index.md` and `readme.md` were updated and synced with `graphify update .`.


### ISSUE 51: Implement dynamic User-Agent rotation, robots.txt compliance routing, and design premium SearqonBot landing and compliance page


Searqon was hardened against anti-bot defenses by implementing dynamic User-Agent rotation and clarifying bot identification. A pool of realistic browser User-Agent strings (Chrome, Firefox, Safari on Windows, macOS, Android, iOS) was defined in `src/crawler.go`. The robots.txt matching engine in `findAllowedAgent` was updated to check if general crawlers (`*`) are allowed on a path; if so, it randomly selects and returns a browser User-Agent string from the pool to mimic human browser traffic. If `*` is blocked but a specific token is whitelisted, it checks for both `"searqonbot"` and `"searqon"` and falls back to our official token. The custom User-Agent for Searqon was updated to `"SearqonBot/1.0 (+https://sooriya04.github.io/Searqon/)"` in both the crawler and default scraper headers. The headless Lightpanda subprocess scraper in `src/scraper.go` was updated to accept and pass this resolved/rotated User-Agent dynamically using the `--user-agent` flag. An interactive, premium landing page was created as `index.html` at the root of the workspace for GitHub Pages hosting, providing webmasters with a visual system overview, a console showing log operations, and copy-pasteable robots.txt configuration recipes. The obsolete `botinfo.html` file was deleted. All changes were compiled, verified, and cataloged via `graphify`.


### ISSUE 52: Expand scrape cache metadata, modularize extraction pipeline, and refactor core services

Searqon's scraping and caching architecture was significantly expanded to support structured, crawler-frontier-ready metadata throughout the entire pipeline. The PostgreSQL scrape_cache schema was enhanced with rich metadata fields including canonical URLs, domains, descriptions, authors, publication dates, language information, outbound links stored as JSONB, HTTP status codes, content types, extraction methods, fetch durations, and direct expiration tracking. Cache retrieval and persistence logic was rewritten to properly handle nullable database values, JSON serialization and deserialization, and automatic domain extraction. Metadata extraction was moved into a dedicated extractor sub-package responsible for canonical URL normalization, language detection, author discovery, Open Graph and Twitter metadata parsing, recursive JSON-LD processing, publication date extraction, and outbound URL validation and deduplication. The scraping pipeline was further improved by removing tables, images, and code blocks before markdown conversion to generate cleaner text-focused output. Core services were modularized by splitting large source files into focused database, scraper, crawler, HTTP, Lightpanda, and robots components, while redundant helper functions and duplicate variables were removed to simplify maintenance. The cache eviction workflow was updated to operate directly on SQL expiration constraints, HTTP handling was enhanced to preserve content-type metadata, and endpoint logic was updated to correctly process final redirect destinations. All refactored components compiled successfully, passed linting and test validation, and were indexed for workspace graph analysis.
