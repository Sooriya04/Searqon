package scraper

import (
	"context"
	"crypto/tls"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

var domainLimiters sync.Map

// WaitDomainRateLimit enforces token-bucket rate limits per domain (5 req/sec).
func WaitDomainRateLimit(ctx context.Context, domain string) error {
	if domain == "" {
		return nil
	}
	val, _ := domainLimiters.LoadOrStore(domain, rate.NewLimiter(rate.Limit(5.0), 5))
	limiter := val.(*rate.Limiter)
	return limiter.Wait(ctx)
}

var browserUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 13_6) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (Windows NT 10.0; rv:126.0) Gecko/20100101 Firefox/126.0",
}

var browserAcceptLanguages = []string{
	"en-US,en;q=0.9",
	"en-GB,en;q=0.8",
	"en;q=0.9",
}

var headerRand = rand.New(rand.NewSource(time.Now().UnixNano()))
var headerMu sync.Mutex

func browserTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion:               tls.VersionTLS12,
		MaxVersion:               tls.VersionTLS13,
		PreferServerCipherSuites: true,
		NextProtos:               []string{"h2", "http/1.1"},
		CurvePreferences:         []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384},
		InsecureSkipVerify:       true,
	}
}

func newBrowserTransport(proxy func(*http.Request) (*url.URL, error)) *http.Transport {
	return &http.Transport{
		Proxy:                 proxy,
		TLSClientConfig:       browserTLSConfig(),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxConnsPerHost:       10,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   4 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
	}
}

func initBrowserTransport() {
	httpClient.Transport = newBrowserTransport(nil)
}

func pickBrowserHeaders(userAgent string) map[string]string {
	headerMu.Lock()
	defer headerMu.Unlock()

	ua := userAgent
	if ua == "" {
		ua = browserUserAgents[headerRand.Intn(len(browserUserAgents))]
	}

	headers := make(map[string]string, len(defaultHeaders))
	for k, v := range defaultHeaders {
		headers[k] = v
	}
	headers["User-Agent"] = ua
	headers["Accept-Language"] = browserAcceptLanguages[headerRand.Intn(len(browserAcceptLanguages))]

	return headers
}
