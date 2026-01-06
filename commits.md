## DuckDuckGo Search Microservice – Initial Implementation

Implemented a standalone DuckDuckGo Search microservice using Node.js and Express, designed as the first building block of the Searqon search pipeline. The service follows a clean, production-oriented folder structure with clear separation of concerns between routing, controllers, services, configuration, and shared utilities.

The microservice exposes a REST endpoint that accepts natural-language search queries via a JSON request body and executes real DuckDuckGo searches using the HTML endpoint. Search results are parsed server-side using Cheerio, normalized into a consistent JSON structure, and returned to the client with titles, URLs, short preview snippets, and source metadata.

The implementation includes defensive request handling to account for DuckDuckGo’s non-API HTML responses, proper request encoding, browser-like headers to avoid silent blocking, and result filtering to exclude advertisements and redirect URLs. Input validation is performed at the controller layer to ensure only valid, non-empty search queries are processed.

This service is intentionally scoped to search-only responsibilities, returning lightweight preview data rather than full article content. It is designed to integrate cleanly with future microservices such as a crawler and extractor, which will handle full-page retrieval and content extraction in subsequent phases.

## Reddit Search Microservice – Initial Implementation

Implemented a standalone Reddit Search microservice as an additional data source within the Searqon search pipeline, following the same clean, production-oriented architecture used across the system. The implementation is structured with clear separation of concerns between routing, controllers, and services to ensure maintainability and future extensibility.

The microservice exposes a REST endpoint that accepts Reddit-optimized search queries via a JSON request body and retrieves live discussion data using Reddit’s public JSON search endpoint. Search results are normalized server-side into a consistent JSON format, including post titles, subreddit names, scores, timestamps, permalinks, and text previews where available.

The service includes robust input validation at the controller layer, explicit User-Agent headers to comply with Reddit’s access requirements, and defensive error handling to gracefully manage network or upstream failures. The implementation avoids HTML scraping entirely, relying on structured JSON responses for stability and reduced breakage risk.

This microservice is intentionally scoped to discussion discovery and opinion-based signal extraction, returning lightweight preview data rather than full comment threads. It is designed to integrate seamlessly with downstream ranking, aggregation, and synthesis layers in later phases of the Searqon pipeline.
