package handlers

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

var (
	metricsTotalRequests   uint64
	metricsSearchRequests  uint64
	metricsScrapeRequests  uint64
	metricsCacheHits       uint64
	metricsCacheMisses     uint64
	metricsStartTime       = time.Now()
)

// IncTotalRequests increments total request count.
func IncTotalRequests() {
	atomic.AddUint64(&metricsTotalRequests, 1)
}

// IncSearchRequests increments total search query count.
func IncSearchRequests() {
	atomic.AddUint64(&metricsSearchRequests, 1)
}

// IncScrapeRequests increments total scrape query count.
func IncScrapeRequests() {
	atomic.AddUint64(&metricsScrapeRequests, 1)
}

// IncCacheHit increments cache hit count.
func IncCacheHit() {
	atomic.AddUint64(&metricsCacheHits, 1)
}

// IncCacheMiss increments cache miss count.
func IncCacheMiss() {
	atomic.AddUint64(&metricsCacheMisses, 1)
}

// MetricsHandler exports Prometheus-formatted telemetry metrics.
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	uptime := time.Since(metricsStartTime).Seconds()

	out := fmt.Sprintf(`# HELP searqon_uptime_seconds Total uptime in seconds.
# TYPE searqon_uptime_seconds counter
searqon_uptime_seconds %.2f

# HELP searqon_requests_total Total HTTP requests served.
# TYPE searqon_requests_total counter
searqon_requests_total %d

# HELP searqon_search_requests_total Total search queries processed.
# TYPE searqon_search_requests_total counter
searqon_search_requests_total %d

# HELP searqon_scrape_requests_total Total scrape requests processed.
# TYPE searqon_scrape_requests_total counter
searqon_scrape_requests_total %d

# HELP searqon_cache_hits_total Total in-memory / DB cache hits.
# TYPE searqon_cache_hits_total counter
searqon_cache_hits_total %d

# HELP searqon_cache_misses_total Total cache misses.
# TYPE searqon_cache_misses_total counter
searqon_cache_misses_total %d
`, uptime, atomic.LoadUint64(&metricsTotalRequests), atomic.LoadUint64(&metricsSearchRequests), atomic.LoadUint64(&metricsScrapeRequests), atomic.LoadUint64(&metricsCacheHits), atomic.LoadUint64(&metricsCacheMisses))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(out))
}
