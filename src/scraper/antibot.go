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

// BrowserPersona models a completely consistent modern browser identity.
// Crucial for anti-bot systems: UA, Client Hints (Sec-CH-UA), and platform must match.
type BrowserPersona struct {
	Name            string
	UserAgent       string
	SecChUa         string
	SecChUaMobile   string
	SecChUaPlatform string
	Accept          string
	AcceptLanguage  string
	AcceptEncoding  string
	IsChromium      bool
}

var modernPersonas = []BrowserPersona{
	{
		Name:            "Chrome 128 on Windows 11",
		UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
		SecChUa:         `"Chromium";v="128", "Not;A=Brand";v="24", "Google Chrome";v="128"`,
		SecChUaMobile:   "?0",
		SecChUaPlatform: `"Windows"`,
		Accept:          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		AcceptLanguage:  "en-US,en;q=0.9",
		AcceptEncoding:  "gzip, deflate, br",
		IsChromium:      true,
	},
	{
		Name:            "Chrome 128 on macOS Sonoma",
		UserAgent:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
		SecChUa:         `"Chromium";v="128", "Not;A=Brand";v="24", "Google Chrome";v="128"`,
		SecChUaMobile:   "?0",
		SecChUaPlatform: `"macOS"`,
		Accept:          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		AcceptLanguage:  "en-US,en;q=0.9",
		AcceptEncoding:  "gzip, deflate, br",
		IsChromium:      true,
	},
	{
		Name:            "Safari 17.6 on macOS",
		UserAgent:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Safari/605.1.15",
		SecChUa:         "", // Safari does not send sec-ch-ua headers
		SecChUaMobile:   "",
		SecChUaPlatform: "",
		Accept:          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		AcceptLanguage:  "en-US,en;q=0.9",
		AcceptEncoding:  "gzip, deflate, br",
		IsChromium:      false,
	},
	{
		Name:            "Firefox 129 on Windows 11",
		UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:129.0) Gecko/20100101 Firefox/129.0",
		SecChUa:         "", // Firefox does not send sec-ch-ua headers
		SecChUaMobile:   "",
		SecChUaPlatform: "",
		Accept:          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		AcceptLanguage:  "en-US,en;q=0.5",
		AcceptEncoding:  "gzip, deflate, br",
		IsChromium:      false,
	},
	{
		Name:            "Edge 128 on Windows 11",
		UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36 Edg/128.0.0.0",
		SecChUa:         `"Chromium";v="128", "Not;A=Brand";v="24", "Microsoft Edge";v="128"`,
		SecChUaMobile:   "?0",
		SecChUaPlatform: `"Windows"`,
		Accept:          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		AcceptLanguage:  "en-US,en;q=0.9",
		AcceptEncoding:  "gzip, deflate, br",
		IsChromium:      true,
	},
}

var searchReferers = []string{
	"https://www.google.com/",
	"https://duckduckgo.com/",
	"https://www.bing.com/",
	"https://search.yahoo.com/",
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

// PickRandomPersona returns a realistic random browser persona.
func PickRandomPersona() BrowserPersona {
	headerMu.Lock()
	defer headerMu.Unlock()
	return modernPersonas[headerRand.Intn(len(modernPersonas))]
}

// GetFastHTTPHeaders builds lightweight standard headers for Tier 1 Fast HTTP requests.
func GetFastHTTPHeaders(targetURL string) map[string]string {
	return map[string]string{
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.9",
		"Accept-Encoding": "gzip, deflate, br",
		"Cache-Control":   "no-cache",
		"Pragma":          "no-cache",
	}
}

// GetStealthSpoofedHeaders creates coherent, high-stealth headers for Tier 2 anti-bot evasion.
func GetStealthSpoofedHeaders(targetURL string, persona *BrowserPersona) map[string]string {
	if persona == nil {
		p := PickRandomPersona()
		persona = &p
	}

	headers := map[string]string{
		"User-Agent":                persona.UserAgent,
		"Accept":                    persona.Accept,
		"Accept-Language":           persona.AcceptLanguage,
		"Accept-Encoding":           persona.AcceptEncoding,
		"Upgrade-Insecure-Requests": "1",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "cross-site",
		"Sec-Fetch-User":            "?1",
		"Cache-Control":             "max-age=0",
	}

	if persona.IsChromium {
		headers["Sec-Ch-Ua"] = persona.SecChUa
		headers["Sec-Ch-Ua-Mobile"] = persona.SecChUaMobile
		headers["Sec-Ch-Ua-Platform"] = persona.SecChUaPlatform
		headers["Priority"] = "u=0, i"
	}

	// Referer spoofing: high authority search engine or domain origin
	parsed, err := url.Parse(targetURL)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		headerMu.Lock()
		refIdx := headerRand.Intn(len(searchReferers) + 1)
		headerMu.Unlock()

		if refIdx < len(searchReferers) {
			headers["Referer"] = searchReferers[refIdx]
		} else {
			headers["Referer"] = parsed.Scheme + "://" + parsed.Host + "/"
		}
	}

	return headers
}

func pickBrowserHeaders(userAgent string) map[string]string {
	p := PickRandomPersona()
	if userAgent != "" {
		p.UserAgent = userAgent
	}
	return GetStealthSpoofedHeaders("", &p)
}
