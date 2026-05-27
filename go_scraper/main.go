package main

import (
	"log"
	"net/http"
	"time"
)

// ─── Main ────────────────────────────────────────────────────────────────────

func main() {
	port := "3002"

	mux := http.NewServeMux()
	mux.HandleFunc("/scrape", scrapeHandler)
	mux.HandleFunc("/scrape/html", scrapeHTMLHandler)
	mux.HandleFunc("/scrape/batch", batchScrapeHandler)
	mux.HandleFunc("/map", mapHandler)
	mux.HandleFunc("/crawl", crawlHandler)
	mux.HandleFunc("/health", healthHandler)

	log.Printf("[Go Scraper] Starting on port %s", port)
	log.Printf("[Go Scraper] Endpoints: POST /scrape, POST /scrape/html, POST /scrape/batch, POST /map, POST /crawl, GET /health")

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[Go Scraper] Failed to start: %v", err)
	}
}
