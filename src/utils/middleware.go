package utils

import (
	"compress/gzip"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type clientLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	clientLimiters    sync.Map
	clientCleanupOnce sync.Once
)

func startClientLimiterCleanup() {
	clientCleanupOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(10 * time.Minute)
			for range ticker.C {
				now := time.Now()
				clientLimiters.Range(func(key, val interface{}) bool {
					if entry, ok := val.(clientLimiterEntry); ok {
						if now.Sub(entry.lastSeen) > 15*time.Minute {
							clientLimiters.Delete(key)
						}
					}
					return true
				})
			}
		}()
	})
}

func getClientLimiter(ip string) *rate.Limiter {
	startClientLimiterCleanup()
	now := time.Now()
	if val, found := clientLimiters.Load(ip); found {
		entry := val.(clientLimiterEntry)
		entry.lastSeen = now
		clientLimiters.Store(ip, entry)
		return entry.limiter
	}
	limiter := rate.NewLimiter(rate.Limit(2.0), 30) // 2 req/sec average, burst 30
	entry := clientLimiterEntry{
		limiter:  limiter,
		lastSeen: now,
	}
	actual, _ := clientLimiters.LoadOrStore(ip, entry)
	return actual.(clientLimiterEntry).limiter
}

// GetClientIP extracts the real client IP address from reverse proxy headers (X-Forwarded-For, X-Real-IP)
// or falls back to RemoteAddr.
func GetClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for _, part := range parts {
			ip := strings.TrimSpace(part)
			if parsed := net.ParseIP(ip); parsed != nil {
				return ip
			}
		}
	}

	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		if parsed := net.ParseIP(xrip); parsed != nil {
			return xrip
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if parsed := net.ParseIP(ip); parsed != nil {
		return ip
	}
	if ip != "" {
		return ip
	}
	return "127.0.0.1"
}

// InboundRateLimitMiddleware protects Searqon from client API rate limit abuse.
func InboundRateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := GetClientIP(r)

		limiter := getClientLimiter(ip)
		if !limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"too many requests, rate limit exceeded"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	return g.Writer.Write(b)
}

func (g *gzipResponseWriter) Flush() {
	if gz, ok := g.Writer.(*gzip.Writer); ok {
		_ = gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (g *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return g.ResponseWriter
}

// GzipCompressionMiddleware automatically compresses JSON HTTP responses when client sends Accept-Encoding: gzip.
func GzipCompressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") ||
			strings.Contains(r.Header.Get("Accept"), "text/event-stream") ||
			strings.HasPrefix(r.URL.Path, "/search/stream") ||
			strings.HasPrefix(r.URL.Path, "/mcp") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		gzw := &gzipResponseWriter{ResponseWriter: w, Writer: gz}
		next.ServeHTTP(gzw, r)
	})
}

// responseWriter is a minimal wrapper to capture the HTTP status code.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
	rw.wroteHeader = true
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// HTTPLogger is a middleware that logs all incoming HTTP requests using slog.
func HTTPLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		RecordEndpointCall(r.URL.Path)
		if onTotalRequestsHook != nil {
			onTotalRequestsHook()
		}

		start := time.Now()

		rw := &responseWriter{
			ResponseWriter: w,
			status:         http.StatusOK, // Default to 200 OK
		}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		slog.Info("HTTP Request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", duration.Milliseconds(),
			"client_ip", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}
