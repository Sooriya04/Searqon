package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"src/config"
	"src/db"
	"src/handlers"
	"src/mcp"
	"src/scraper"
	"src/utcp"
	"src/utils"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Println("Searqon v1.2.1")
			return
		case "help", "--help", "-h":
			fmt.Println("Searqon - Self-hosted Web Intelligence Engine for AI Agents")
			fmt.Println("\nUsage:")
			fmt.Println("  searqon            Start HTTP REST, SSE & UTCP server")
			fmt.Println("  searqon mcp        Start Model Context Protocol (MCP) JSON-RPC stdio server")
			fmt.Println("  searqon version    Display current Searqon version")
			return
		}
	}

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
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		// Direct execution logs to stderr to preserve stdout for JSON-RPC
		log.SetOutput(os.Stderr)
		scraper.InitProxyPool()
		db.InitDB()
		defer db.CloseDB()

		mcpServer := mcp.NewServer()
		if err := mcpServer.StartStdioLoop(); err != nil {
			fmt.Fprintf(os.Stderr, "[Searqon MCP] Stdio loop exited: %v\n", err)
			os.Exit(1)
		}
		return
	}

	utils.InitLogger()
	defer utils.CloseLogger()

	scraper.InitProxyPool()
	db.InitDB()
	defer db.CloseDB()

	// 3. Setup HTTP router
	mux := http.NewServeMux()

	// Model Context Protocol (MCP) over HTTP/SSE
	mcpServer := mcp.NewServer()
	mux.HandleFunc("/mcp", mcpServer.SSEHandler())
	mux.HandleFunc("/mcp/sse", mcpServer.SSEHandler())

	// Universal Tool Calling Protocol (UTCP) Manual & Discovery
	utcpServer := utcp.NewServer()
	mux.HandleFunc("/utcp", utcpServer.ManualHandler)
	mux.HandleFunc("/utcp/manual", utcpServer.ManualHandler)

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

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("[Searqon] Server starting on %s:%s (SearXNG: enabled=%v, DB: %s, Redis: enabled=%v, Logs: %s)",
			cfg.Server.Host, port, cfg.SearXNG.Enabled, db.GetBackend(), db.IsRedisEnabled(), cfg.Logs.Storage)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Fatalf("[Searqon] Server failed to start: %v", err)
	case sig := <-shutdown:
		log.Printf("[Searqon] Received signal %v, initiating graceful shutdown...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("[Searqon] Graceful shutdown error: %v", err)
			server.Close()
		}
		log.Println("[Searqon] Server stopped cleanly")
	}
}
