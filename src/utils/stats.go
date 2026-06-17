package utils

import (
	"sync"
	"sync/atomic"
)

type StatsTracker struct {
	mu sync.RWMutex

	SearchCacheHits   uint64 `json:"searchCacheHits"`
	SearchCacheMisses uint64 `json:"searchCacheMisses"`
	ScrapeCacheHits   uint64 `json:"scrapeCacheHits"`
	ScrapeCacheMisses uint64 `json:"scrapeCacheMisses"`

	SearXNGRequests uint64 `json:"searXngRequests"`
	SearXNGSuccess  uint64 `json:"searXngSuccess"`
	DDGRequests     uint64 `json:"ddgRequests"`
	DDGSuccess      uint64 `json:"ddgSuccess"`

	EndpointCalls map[string]uint64 `json:"endpointCalls"`
}

var stats = StatsTracker{
	EndpointCalls: make(map[string]uint64),
}

// RecordEndpointCall increments the call counter for a specific route.
func RecordEndpointCall(endpoint string) {
	stats.mu.Lock()
	defer stats.mu.Unlock()
	stats.EndpointCalls[endpoint]++
}

// RecordSearchCacheHit increments the hit counter.
func RecordSearchCacheHit() {
	atomic.AddUint64(&stats.SearchCacheHits, 1)
}

// RecordSearchCacheMiss increments the miss counter.
func RecordSearchCacheMiss() {
	atomic.AddUint64(&stats.SearchCacheMisses, 1)
}

// RecordScrapeCacheHit increments the hit counter.
func RecordScrapeCacheHit() {
	atomic.AddUint64(&stats.ScrapeCacheHits, 1)
}

// RecordScrapeCacheMiss increments the miss counter.
func RecordScrapeCacheMiss() {
	atomic.AddUint64(&stats.ScrapeCacheMisses, 1)
}

// RecordSearXNGCall tracks SearXNG performance.
func RecordSearXNGCall(success bool) {
	atomic.AddUint64(&stats.SearXNGRequests, 1)
	if success {
		atomic.AddUint64(&stats.SearXNGSuccess, 1)
	}
}

// RecordDDGCall tracks DuckDuckGo fallback performance.
func RecordDDGCall(success bool) {
	atomic.AddUint64(&stats.DDGRequests, 1)
	if success {
		atomic.AddUint64(&stats.DDGSuccess, 1)
	}
}

// GetStats returns a thread-safe copy of system stats.
func GetStats() StatsTracker {
	stats.mu.RLock()
	defer stats.mu.RUnlock()

	calls := make(map[string]uint64)
	for k, v := range stats.EndpointCalls {
		calls[k] = v
	}

	return StatsTracker{
		SearchCacheHits:   atomic.LoadUint64(&stats.SearchCacheHits),
		SearchCacheMisses: atomic.LoadUint64(&stats.SearchCacheMisses),
		ScrapeCacheHits:   atomic.LoadUint64(&stats.ScrapeCacheHits),
		ScrapeCacheMisses: atomic.LoadUint64(&stats.ScrapeCacheMisses),
		SearXNGRequests:   atomic.LoadUint64(&stats.SearXNGRequests),
		SearXNGSuccess:    atomic.LoadUint64(&stats.SearXNGSuccess),
		DDGRequests:       atomic.LoadUint64(&stats.DDGRequests),
		DDGSuccess:        atomic.LoadUint64(&stats.DDGSuccess),
		EndpointCalls:     calls,
	}
}
