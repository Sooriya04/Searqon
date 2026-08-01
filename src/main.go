package main

import (
	"log"
	"net/http"
	"time"

	"src/db"
	"src/handlers"
	"src/scraper"
	"src/utils"
)

func main() {
	port := "4001"

	// 1. Initialize core system modules
	utils.InitLogger()
	scraper.InitProxyPool()
	db.InitDB()
	defer db.CloseDB()

	// 2. Setup HTTP router
	mux := http.NewServeMux()

	// Discovery and Search
	mux.HandleFunc("/search", handlers.SearchHandler)
	mux.HandleFunc("/search/index", handlers.SearchIndexHandler)
	mux.HandleFunc("/pipeline", handlers.PipelineHandler)

	// Scraper API
	mux.HandleFunc("/scrape", handlers.ScrapeHandler)
	mux.HandleFunc("/scrape/chunked", handlers.ChunkedScrapeHandler)
	mux.HandleFunc("/scrape/batch", handlers.BatchScrapeHandler)
	mux.HandleFunc("/scrape/html", handlers.HTMLScrapeHandler)
	mux.HandleFunc("/r/", handlers.JinaReaderHandler)

	// Crawler and Site Mapper
	mux.HandleFunc("/crawl", handlers.CrawlHandler)
	mux.HandleFunc("/map", handlers.MapHandler)

	// Utility and Telemetry
	mux.HandleFunc("/screenshot", handlers.ScreenshotHandler)
	mux.HandleFunc("/summarize", handlers.SummarizeHandler)
	mux.HandleFunc("/extract", handlers.ExtractHandler)
	mux.HandleFunc("/feed", handlers.FeedHandler)
	mux.HandleFunc("/stats", handlers.StatsHandler)

	// OpenAPI API Documentation
	mux.HandleFunc("/openapi.json", handlers.OpenAPIHandler)
	mux.HandleFunc("/", handlers.SwaggerUIHandler)

	// System Health
	mux.HandleFunc("/health", handlers.HealthHandler)

	// Wrap mux with our HTTPLogger middleware
	loggedMux := utils.HTTPLogger(mux)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      loggedMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("[Searqon] Server starting on port %s", port)

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[Searqon] Server failed to start: %v", err)
	}
}
