# Scraping Architecture

Searqon uses a high-performance, concurrent scraping engine built with **Go**.

## Overview

To maximize throughput and efficiency, Searqon decouples search orchestration (Node.js) from actual web extraction (Go). This allows the system to leverage Go's native parallelism (Goroutines) and extremely low memory footprint, making it ideal for high-concurrency scraping on hardware with 4GB-16GB of RAM.

## Key Components

### 1. Go Scraper Microservice (`go_scraper/`)
The core extraction engine. It is built as a lightweight HTTP microservice:
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

1. A **Service** (e.g., `duckduckgo.js`) identifies a list of URLs to scrape.
2. It calls `ScrapUrlBatch(urls)`.
3. The request hits the Go microservice on port `3002`.
4. The Go server spawns parallel goroutines to fetch all URLs simultaneously.
5. Content is cleaned (noise removed, flattened to plain text) and returned as a JSON array.
6. The Node.js service receives the clean content and proceeds with ranking/synthesis.

## Technical Highlights

- **Flat Text Extraction**: The engine is optimized for LLMs, stripping all formatting (newlines, asterisks, markdown) into a clean, unbroken plain-text string.
- **Instant Batches**: Thanks to Go's concurrency model, a batch of 5-10 URLs usually completes in under 2 seconds.
- **Low Footprint**: The entire scraping service typically uses less than 50MB of RAM, leaving maximum resources for your LLM and database.
