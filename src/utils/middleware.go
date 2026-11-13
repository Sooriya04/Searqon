package utils

import (
	"compress/gzip"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
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

// GetClientIP extracts the real client IP address.
// If the direct remote client IP is a trusted proxy (e.g. loopback, private network, or configured TRUSTED_PROXIES),
// it inspects X-Forwarded-For or X-Real-IP.
// If the direct remote IP is untrusted (e.g. direct public internet client), proxy headers are ignored to prevent rate limit bypass spoofing.
func GetClientIP(r *http.Request) string {
	remoteIP := extractRemoteIP(r.RemoteAddr)

	if remoteIP != "" && isTrustedProxy(remoteIP) {
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
	}

	if remoteIP != "" {
		return remoteIP
	}

	return "127.0.0.1"
}

func extractRemoteIP(rawRemote string) string {
	rawRemote = strings.TrimSpace(rawRemote)
	if ip, _, err := net.SplitHostPort(rawRemote); err == nil {
		if parsed := net.ParseIP(ip); parsed != nil {
			return ip
		}
	} else {
		// Clean possible bracketed IPv6 or plain IP without port
		cleanRemote := strings.Trim(rawRemote, "[]")
		if parsed := net.ParseIP(cleanRemote); parsed != nil {
			return cleanRemote
		}
	}
	return ""
}

func isTrustedProxy(ipStr string) bool {
	parsed := net.ParseIP(ipStr)
	if parsed == nil {
		return false
	}

	// 1. Check explicit TRUSTED_PROXIES env var if configured
	if env := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES")); env != "" {
		if env == "*" {
			return true
		}
		for _, item := range strings.Split(env, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, cidr, err := net.ParseCIDR(item); err == nil {
				if cidr.Contains(parsed) {
					return true
				}
			} else if targetIP := net.ParseIP(item); targetIP != nil {
				if targetIP.Equal(parsed) {
					return true
				}
			}
		}
		return false
	}

	// 2. Check if proxy trusting is explicitly disabled
	if trustEnv := strings.ToLower(strings.TrimSpace(os.Getenv("TRUST_PROXY_HEADERS"))); trustEnv == "false" || trustEnv == "0" {
		return false
	}

	// 3. By default, trust loopback and private network proxies (Docker, Nginx, K8s internal IPs)
	return parsed.IsLoopback() || parsed.IsPrivate()
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
