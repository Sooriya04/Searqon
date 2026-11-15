package utils

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expectedIP string
	}{
		{
			name:       "Direct RemoteAddr without proxy",
			remoteAddr: "203.0.113.195:45821",
			headers:    nil,
			expectedIP: "203.0.113.195",
		},
		{
			name:       "X-Forwarded-For single IP",
			remoteAddr: "172.18.0.1:51234",
			headers:    map[string]string{"X-Forwarded-For": "198.51.100.42"},
			expectedIP: "198.51.100.42",
		},
		{
			name:       "X-Forwarded-For chain selects leftmost client IP",
			remoteAddr: "10.0.0.1:443",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.5, 198.51.100.1, 10.0.0.1"},
			expectedIP: "203.0.113.5",
		},
		{
			name:       "X-Real-IP fallback when X-Forwarded-For absent",
			remoteAddr: "172.18.0.1:51234",
			headers:    map[string]string{"X-Real-IP": "198.51.100.77"},
			expectedIP: "198.51.100.77",
		},
		{
			name:       "Malformed IP in X-Forwarded-For skips to valid IP",
			remoteAddr: "172.18.0.1:51234",
			headers:    map[string]string{"X-Forwarded-For": "unknown, 198.51.100.88"},
			expectedIP: "198.51.100.88",
		},
		{
			name:       "Raw IP without port in RemoteAddr",
			remoteAddr: "203.0.113.88",
			headers:    nil,
			expectedIP: "203.0.113.88",
		},
		{
			name:       "Bracketed IPv6 without port in RemoteAddr",
			remoteAddr: "[2001:db8::1]",
			headers:    nil,
			expectedIP: "2001:db8::1",
		},
		{
			name:       "Bracketed IPv6 with port in RemoteAddr",
			remoteAddr: "[2001:db8::1]:8080",
			headers:    nil,
			expectedIP: "2001:db8::1",
		},
		{
			name:       "Untrusted public client spoofed X-Forwarded-For rejected",
			remoteAddr: "203.0.113.195:45821",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4"},
			expectedIP: "203.0.113.195",
		},
		{
			name:       "Untrusted public client spoofed X-Real-IP rejected",
			remoteAddr: "198.51.100.22:12345",
			headers:    map[string]string{"X-Real-IP": "10.0.0.99"},
			expectedIP: "198.51.100.22",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			ip := GetClientIP(req)
			if ip != tt.expectedIP {
				t.Errorf("GetClientIP() = %s; want %s", ip, tt.expectedIP)
			}
		})
	}
}

func TestMiddleware_PreservesFlusher(t *testing.T) {
	flushed := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected w to implement http.Flusher through middleware stack")
		}
		flusher.Flush()
		flushed = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	stack := InboundRateLimitMiddleware(GzipCompressionMiddleware(HTTPLogger(handler)))

	req := httptest.NewRequest(http.MethodGet, "/search/stream?q=test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()

	stack.ServeHTTP(rec, req)

	if !flushed {
		t.Errorf("expected flusher.Flush() to be called successfully")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
}

func TestGetClientIP_TrustedProxiesConfig(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "198.51.100.200/32")

	req := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
	req.RemoteAddr = "198.51.100.200:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.77")

	ip := GetClientIP(req)
	if ip != "203.0.113.77" {
		t.Errorf("expected 203.0.113.77 from explicitly trusted proxy, got %s", ip)
	}

	// Untrusted proxy should still be rejected
	req2 := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
	req2.RemoteAddr = "198.51.100.201:12345"
	req2.Header.Set("X-Forwarded-For", "203.0.113.77")

	ip2 := GetClientIP(req2)
	if ip2 != "198.51.100.201" {
		t.Errorf("expected 198.51.100.201 from untrusted proxy, got %s", ip2)
	}
}

func TestGetClientIP_DisabledProxyTrust(t *testing.T) {
	t.Setenv("TRUST_PROXY_HEADERS", "false")

	// Even private IP should not have headers trusted when disabled
	req := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
	req.RemoteAddr = "172.18.0.1:51234"
	req.Header.Set("X-Forwarded-For", "198.51.100.42")

	ip := GetClientIP(req)
	if ip != "172.18.0.1" {
		t.Errorf("expected 172.18.0.1 when proxy headers disabled, got %s", ip)
	}
}

func TestGzipCompressionMiddleware_Scenarios(t *testing.T) {
	t.Run("Compresses response with body", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":"hello compressed world"}`))
		})
		ts := GzipCompressionMiddleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		ts.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		if enc := rec.Header().Get("Content-Encoding"); enc != "gzip" {
			t.Errorf("expected Content-Encoding=gzip, got %q", enc)
		}

		gzReader, err := gzip.NewReader(rec.Body)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer gzReader.Close()
		decompressed, err := io.ReadAll(gzReader)
		if err != nil {
			t.Fatalf("failed to read gzipped body: %v", err)
		}
		if string(decompressed) != `{"message":"hello compressed world"}` {
			t.Errorf("unexpected decompressed body: %s", string(decompressed))
		}
	})

	t.Run("Does not compress 204 No Content", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
		ts := GzipCompressionMiddleware(handler)

		req := httptest.NewRequest(http.MethodDelete, "/api/item/1", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		ts.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204 No Content, got %d", rec.Code)
		}
		if enc := rec.Header().Get("Content-Encoding"); enc != "" {
			t.Errorf("expected empty Content-Encoding for 204, got %q", enc)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("expected 0-length body for 204, got %d bytes", rec.Body.Len())
		}
	})

	t.Run("Does not compress 304 Not Modified", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotModified)
		})
		ts := GzipCompressionMiddleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/resource", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		ts.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotModified {
			t.Fatalf("expected 304 Not Modified, got %d", rec.Code)
		}
		if enc := rec.Header().Get("Content-Encoding"); enc != "" {
			t.Errorf("expected empty Content-Encoding for 304, got %q", enc)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("expected 0-length body for 304, got %d bytes", rec.Body.Len())
		}
	})

	t.Run("Does not compress empty body with 200 OK", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		ts := GzipCompressionMiddleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/empty", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		ts.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		if enc := rec.Header().Get("Content-Encoding"); enc != "" {
			t.Errorf("expected empty Content-Encoding for empty body 200, got %q", enc)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("expected 0-length body, got %d bytes", rec.Body.Len())
		}
	})

	t.Run("Does not compress zero-byte write", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte{})
		})
		ts := GzipCompressionMiddleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/zerobyte", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		ts.ServeHTTP(rec, req)

		if enc := rec.Header().Get("Content-Encoding"); enc != "" {
			t.Errorf("expected empty Content-Encoding for zero-byte write, got %q", enc)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("expected 0-length body, got %d bytes", rec.Body.Len())
		}
	})
}
