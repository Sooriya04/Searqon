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
