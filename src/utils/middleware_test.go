package utils

import (
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
