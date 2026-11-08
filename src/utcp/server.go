package utcp

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
)

const maxCachedManuals = 16

// Server handles UTCP manual discovery requests over HTTP.
type Server struct {
	mu            sync.RWMutex
	cachedManuals map[string][]byte
	cacheOrder    []string // FIFO / LRU eviction tracker
}

// NewServer initializes a new Server for UTCP discovery.
func NewServer() *Server {
	return &Server{
		cachedManuals: make(map[string][]byte),
		cacheOrder:    make([]string, 0, maxCachedManuals),
	}
}

// ResolveBaseURL dynamically calculates the public base URL based on environment variables,
// reverse-proxy headers, and host headers.
func (s *Server) ResolveBaseURL(r *http.Request) string {
	// 1. Explicit override via SEARQON_PUBLIC_URL environment variable
	if envURL := os.Getenv("SEARQON_PUBLIC_URL"); envURL != "" {
		return strings.TrimRight(envURL, "/")
	}

	// 2. Resolve scheme (reverse-proxy aware)
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}

	// 3. Resolve host (reverse-proxy aware)
	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		// If multiple comma-separated hosts are provided, use the first one
		if commaIdx := strings.Index(fwdHost, ","); commaIdx != -1 {
			host = strings.TrimSpace(fwdHost[:commaIdx])
		} else {
			host = strings.TrimSpace(fwdHost)
		}
	}

	if host == "" {
		host = "localhost:7493"
	}

	// 4. Strip redundant default ports matching scheme to prevent https://domain:80 or http://domain:443
	if scheme == "https" && strings.HasSuffix(host, ":443") {
		host = strings.TrimSuffix(host, ":443")
	} else if scheme == "http" && strings.HasSuffix(host, ":80") {
		host = strings.TrimSuffix(host, ":80")
	} else if scheme == "https" && strings.HasSuffix(host, ":80") {
		// When upstream reverse-proxy terminates TLS and sets X-Forwarded-Proto: https,
		// but client/proxy forwarded Host retains internal plaintext port :80, strip it.
		host = strings.TrimSuffix(host, ":80")
	}

	return scheme + "://" + host
}

// ManualHandler serves the UTCP manual as formatted JSON with CORS headers.
func (s *Server) ManualHandler(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for web-based or local agent runtimes
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	baseURL := s.ResolveBaseURL(r)

	s.mu.RLock()
	data, ok := s.cachedManuals[baseURL]
	s.mu.RUnlock()

	if !ok {
		s.mu.Lock()
		// Double-checked locking to avoid redundant serialization and race conditions
		if existing, found := s.cachedManuals[baseURL]; found {
			data = existing
		} else {
			manual := BuildManual(baseURL)
			marshaled, err := json.MarshalIndent(manual, "", "  ")
			if err != nil {
				s.mu.Unlock()
				http.Error(w, "Failed to encode UTCP manual", http.StatusInternalServerError)
				return
			}
			// Enforce LRU / FIFO cap of maxCachedManuals (16) to avoid host header DoS
			if len(s.cachedManuals) >= maxCachedManuals && len(s.cacheOrder) > 0 {
				oldest := s.cacheOrder[0]
				s.cacheOrder = s.cacheOrder[1:]
				delete(s.cachedManuals, oldest)
			}
			s.cachedManuals[baseURL] = marshaled
			s.cacheOrder = append(s.cacheOrder, baseURL)
			data = marshaled
		}
		s.mu.Unlock()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
