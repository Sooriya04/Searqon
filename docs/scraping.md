# Scraping Architecture

Searqon uses a high-performance, concurrent scraping engine built with **Go** to deliver high-fidelity data in minimal time.

## Overview

To maximize throughput and efficiency, Searqon decouples search orchestration (Node.js) from actual web extraction (Go). This allows the system to leverage Go's native parallelism (Goroutines) and an extremely low memory footprint, making it ideal for high-concurrency scraping on hardware with limited resources.

## Key Components

### 1. Go Scraper Engine (`go_scraper/`)
The core extraction engine. It is built as a compiled native binary:
- **Goroutine Parallelism**: Instead of managed worker pools, the Go scraper spawns a lightweight goroutine for every URL in a batch. This allows for near-instant parallel processing of 10+ URLs at once.
- **Extraction Engines**:
    - **Go-Readability**: A Go port of Mozilla's Readability, used for high-quality semantic article extraction.
    - **GoQuery**: A jQuery-like selector engine used for surgical noise removal (removing ads, nav, footer, scripts).
- **Stealth Mode**: Uses custom HTTP headers and automatic decompression (Gzip, Deflate, Brotli) to bypass common bot detections and handle complex server encodings.

### 2. Node.js ScrapUrl Client (`scrapper/ScrapUrl.js`)
A bridge that delegates scraping tasks to the Go service.
- **Single Scrape**: `POST /scrape` for individual URLs.
- **Batch Scrape**: `POST /scrape/batch` for high-speed parallel extraction of search result lists.
- **GitHub Intercept**: A specialized native handler that fetches READMEs directly via the GitHub API for 100% accuracy and speed.

## Data Flow

1. **Route** — (Phase 1) The **Native JS Intelligence Engine** analyzes the categorical intent of the query and selects the most relevant domain-specific sources (e.g., `pubmed` for medicine, `github` for tech).
2. A **Service** (e.g., `github.js`) is triggered for the specific domain sources, followed by the `duckduckgo.js` baseline.
3. Search results are batched and sent to the **Go Scraper Binary** on port `3002`.
4. The Go engine spawns parallel goroutines to fetch and clean all URLs simultaneously.
5. Content is cleaned (noise removed, converted to Markdown or plain text) and returned to Node.js.
6. The Node.js service flattens the results and delivers them as a direct JSON array.

## Technical Highlights

- **Markdown Output**: The engine can deliver high-fidelity Markdown, preserving document structure (headers, lists, links) which is ideal for LLM context.
- **Instant Batches**: Thanks to Go's concurrency model, a batch of 5-10 URLs usually completes in under 2-5 seconds.
- **Low Footprint**: The entire scraping service typically uses less than **30MB - 50MB of RAM**.
- **No Heavy Browsers**: By using Go's raw HTTP client instead of Puppeteer/Playwright, we eliminate hundreds of megabytes of RAM bloat.
