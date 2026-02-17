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
