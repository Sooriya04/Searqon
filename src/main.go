package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"src/config"
	"src/db"
	"src/handlers"
	"src/scraper"
	"src/utils"
)

func main() {
	// 1. Load configuration from settings.yml / config.yml
	cfg := config.LoadConfig()

	port := fmt.Sprintf("%d", cfg.Server.Port)
	if port == "0" {
		port = "4001"
	}

	readTimeout := time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second
	if readTimeout <= 0 {
		readTimeout = 30 * time.Second
	}
	writeTimeout := time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second
	if writeTimeout <= 0 {
		writeTimeout = 120 * time.Second
	}

	// 2. Initialize core system modules
	utils.InitLogger()
	defer utils.CloseLogger()

	scraper.InitProxyPool()
	db.InitDB()
	defer db.CloseDB()

	// 3. Setup HTTP router
	mux := http.NewServeMux()

	// Discovery and Search
	mux.HandleFunc("/search", handlers.SearchHandler)
	mux.HandleFunc("/search/stream", handlers.SearchStreamHandler)
	mux.HandleFunc("/search/explain", handlers.SearchExplainHandler)
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

	// Utility, Logs and Telemetry
	mux.HandleFunc("/screenshot", handlers.ScreenshotHandler)
	mux.HandleFunc("/summarize", handlers.SummarizeHandler)
	mux.HandleFunc("/extract", handlers.ExtractHandler)
	mux.HandleFunc("/feed", handlers.FeedHandler)
	mux.HandleFunc("/stats", handlers.StatsHandler)
	mux.HandleFunc("/metrics", handlers.MetricsHandler)
	mux.HandleFunc("/logs", handlers.LogsHandler)

	// OpenAPI API Documentation
	mux.HandleFunc("/openapi.json", handlers.OpenAPIHandler)
	mux.HandleFunc("/", handlers.SwaggerUIHandler)

	// System Health
	mux.HandleFunc("/health", handlers.HealthHandler)

	// Wrap mux with RateLimit, Gzip & Logger middleware
	handlerStack := utils.InboundRateLimitMiddleware(utils.GzipCompressionMiddleware(utils.HTTPLogger(mux)))

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handlerStack,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("[Searqon] Server starting on %s:%s (SearXNG: enabled=%v, DB: %s, Redis: enabled=%v, Logs: %s)",
		cfg.Server.Host, port, cfg.SearXNG.Enabled, db.GetBackend(), db.IsRedisEnabled(), cfg.Logs.Storage)

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[Searqon] Server failed to start: %v", err)
	}
}
