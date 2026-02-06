# Scrapper Module Documentation

This document explains the architecture and usage of the `scrapper` module in Searqon. The scrapper is designed for high-performance, concurrent web scraping using a customized Worker Pool pattern.

## Architecture Overview

The scraping system uses a **Master-Worker** architecture. The main application thread communicates with a pool of persistent child processes (Workers). This ensures that CPU-intensive parsing operations and network I/O do not block the main event loop.

### File Breakdown

| File              | Role                     | Description                                                                                                                                      |
| ----------------- | ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| **`ScrapUrl.js`** | **Facade (Entry Point)** | The public API. It accepts a URL and delegates the execution to the Worker Pool. It returns a Promise that resolves when the scrape is complete. |
| **`pool.js`**     | **Manager**              | Manages a pool of `child_process` forks. Handles task scheduling, concurrency limits, and queue management.                                      |
| **`worker.js`**   | **Worker Entry**         | The script running inside each child process. It listens for messages from the pool, executes the logic in `core.js`, and sends results back.    |
| **`core.js`**     | **Business Logic**       | Contains the actual scraping logic: URL routing (dispatching to specialized vs. generic scrapers), fetching, and processing.                     |
| **`fetcher.js`**  | **Network Layer**        | Handles low-level HTTP/HTTPS requests, User-Agent rotation, and timeouts.                                                                        |
| **`parser.js`**   | **Content Processing**   | Cleans raw HTML using DOM manipulation and cleans it into readable Markdown or text.                                                             |

---

## Technical Details

### Concurrency & Worker Pool

- **Spanning**: The pool spawns a fixed number of child processes (Workers) using Node.js `child_process.fork()`.
- **Count**: By default, it spawns **10 workers** (or matches CPU core count). This creates 10 separate V8 instances ready to handle tasks.
- **Queueing**: If all 10 workers are busy, new requests are pushed into an internal FIFO queue. As soon as a worker finishes a task, it picks the next one from the queue.
- **Communication**: Data is passed between the Main Process and Workers via IPC (Inter-Process Communication) serialization.

## Usage Guide

### Processing a Single URL

You can scrape a single URL by importing `ScrapUrl` and using `await`.

```javascript
const ScrapUrl = require('../scrapper/ScrapUrl');

async function getPage() {
  try {
    const result = await ScrapUrl('https://example.com');
    console.log(result.title);
    console.log(result.content);
  } catch (error) {
    console.error('Scrape failed:', error);
  }
}
```

### Processing Batch URLs (Recommended)

To scrape multiple URLs efficiently, use `Promise.all` to schedule them all at once. The Worker Pool will automatically manage the load.

```javascript
const ScrapUrl = require('../scrapper/ScrapUrl');

const targets = [
  'https://google.com',
  'https://github.com',
  'https://wikipedia.org',
  // ... add more URLs
];

async function scrapeBatch() {
  console.log(`Starting batch scrape of ${targets.length} URLs...`);

  // Map every URL to a ScrapUrl promise
  const promises = targets.map((url) => ScrapUrl(url));

  // Wait for all to complete
  const results = await Promise.all(promises);

  results.forEach((res) => {
    console.log(`Scraped: ${res.title} (${res.wordCount} words)`);
  });
}
```

### Why this approach?

1.  **Non-Blocking**: The main server stays responsive even while parsing heavy HTML.
2.  **Scalable**: It automatically utilizes multiple CPU cores.
3.  **Resilient**: If a worker crashes (rare), it doesn't bring down the main server.
