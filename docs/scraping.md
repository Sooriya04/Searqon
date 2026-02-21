# Scraping Architecture

Searqon uses a high-performance, auto-scaling scraping engine built with Python. 

## Overview

To maximize throughput and efficiency, Searqon decouples the search orchestration (Node.js) from the actual web crawling (Python). This allows the system to leverage Python's mature scraping ecosystem and intelligent auto-scaling capabilities.

## Key Components

### 1. Python Scraper Service (`crawler/`)
The heart of the scraping engine. It is built using an asynchronous architecture:
- **`BeautifulSoupCrawler`**: Handles the actual fetching and parsing of HTML.
- **`Autoscaler`**: Monitors system CPU and Memory usage in real-time. It dynamically adds or removes worker tasks to ensure maximum throughput without crashing the host machine.
- **`SessionPool`**: Manages rotation of cookies and headers. It tracks "bad" sessions (e.g., those getting 403s or CAPTCHAs) and retires them automatically to prevent IP shadow-bans.

### 2. Node.js ScrapUrl Client (`scrapper/ScrapUrl.js`)
A lightweight bridge that delegates scraping tasks to the Python service. 
- It communicates via a local HTTP API on port `3002`.
- It maintains the same interface as the previous Puppeteer version, ensuring backward compatibility with all existing search services.

## Data Flow

1. A **Service** (e.g., `reddit.js`) identifies a URL to scrape.
2. It calls `ScrapUrl(url)`.
3. `ScrapUrl` sends an HTTP POST request to the Python microservice.
4. The Python **Autoscaler** assigns a worker to the task.
5. The worker fetches the content, cleans the HTML with **BeautifulSoup**, and returns the structured text.
6. The Node.js service receives the clean content and proceeds with ranking/synthesis.

## Technical Highlights

- **Dynamic Scaling**: Unlike static concurrency limits, the system scales based on actual hardware pressure.
- **Anti-Blocking**: Session rotation and score-based retirement minimize detection.
- **Low Overhead**: By avoiding full browser automation (Puppeteer) in favor of raw HTTP + HTML parsing, the system uses 90% less RAM and CPU.
