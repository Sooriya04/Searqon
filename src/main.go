package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	port := "4001"

	// Initialize Postgres Cache
	InitDB()
	defer CloseDB()

	mux := http.NewServeMux()

	// ── Search Aggregator ────────────────────────────────────────────────────
	mux.HandleFunc("/search", searchHandler)   // POST { "query": "..." } → search DDG + scrape top results

	// ── Raw Scraper ──────────────────────────────────────────────────────────
	mux.HandleFunc("/scrape", scrapeHandler)         // POST { "url": "..." }
	mux.HandleFunc("/scrape/batch", batchScrapeHandler) // POST { "urls": [...] }
	mux.HandleFunc("/scrape/html", scrapeHTMLHandler)   // POST { "html": "...", "url": "..." }

	// ── Crawler / Mapper ─────────────────────────────────────────────────────
	mux.HandleFunc("/crawl", crawlHandler) // POST { "url": "...", "limit": 30, "depth": 2 }
	mux.HandleFunc("/map", mapHandler)     // POST { "url": "...", "limit": 50 }

	// ── Health ───────────────────────────────────────────────────────────────
	mux.HandleFunc("/health", healthHandler)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("[Searqon] Starting on :%s", port)
	log.Printf("[Searqon] Endpoints: POST /search, POST /scrape, POST /scrape/batch, POST /scrape/html, POST /crawl, POST /map, GET /health")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[Searqon] Failed to start: %v", err)
	}
}
