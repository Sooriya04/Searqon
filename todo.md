# 🚀 Searqon to Firecrawl Transformation

This TODO list tracks the implementation of Firecrawl-like features into the Searqon architecture.

- [x] **Phase 1: High-Fidelity Markdown Extraction**
    - [x] Add `html-to-markdown` dependency to Go scraper.
    - [x] Update `go_scraper/main.go` to support Markdown conversion.
    - [x] Modify `scrapeSingleURL` to accept `format` (markdown/text) parameter.
    - [x] Update `ScrapUrl.js` (Node) to pass options to Go engine.
    - [x] Refine noise-removal to preserve Markdown structural elements.
- [x] **Phase 2: Discovery & Mapping (`/map` and `/crawl`)**
    - [x] Create `controller/crawlController.js` for orchestration.
    - [x] Create `routes/crawl.js` and mount in `app.js`.
    - [x] Implement site mapping (link discovery on same domain).
    - [x] Implement recursive crawling (batch scrape map results).
    - [x] Add depth and limit constraints to prevent OOM.
- [/] **Phase 3: Headless Browser Support (JS Rendering)**
    - [x] Implement fallback logic: if static GET fails or is too short, retry with JS.
    - [x] Use existing Puppeteer integration for rendering.
    - [x] Add basic "Action" support (wait) in the scrape schema.
    - [ ] Optional: Migrate to `chromedp` in Go engine for lower memory usage.
- [x] **Phase 4: Structured Intelligence (`/extract`)**
    - [x] Create `services/extractionService.js`.
    - [x] Implement LLM schema-based extraction (JSON output).
    - [x] Add support for Ollama/Gemini/OpenAI backends.
    - [x] Add autonomous search-then-extract mode (Query -> JSON).

---
*Last updated: 2026-04-02*
