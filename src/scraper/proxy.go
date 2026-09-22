package scraper

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"

	"src/config"
)

var (
	proxies            []string
	residentialProxies []string
	proxyIndex         uint32
	resProxyIndex      uint32
)

// InitProxyPool configures rotating and residential proxy lists.
func InitProxyPool() {
	cfg := config.Get()

	// 1. Standard / Datacenter Rotating Proxies
	var rawList []string
	if cfg != nil && len(cfg.Proxies.Rotating) > 0 {
		rawList = append(rawList, cfg.Proxies.Rotating...)
	}
	if envList := os.Getenv("ROTATING_PROXIES"); envList != "" {
		rawList = append(rawList, strings.Split(envList, ",")...)
	}
	if len(rawList) == 0 {
		if fileBytes, err := os.ReadFile("proxies.txt"); err == nil {
			lines := strings.Split(string(fileBytes), "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
					rawList = append(rawList, trimmed)
				}
			}
		}
	}
	for _, p := range rawList {
		p = normalizeProxyURL(p)
		if p != "" {
			proxies = append(proxies, p)
		}
	}

	// 2. Residential Proxies
	var rawResList []string
	if cfg != nil {
		if cfg.Proxies.ResidentialURL != "" {
			rawResList = append(rawResList, cfg.Proxies.ResidentialURL)
		}
		if len(cfg.Proxies.ResidentialProxies) > 0 {
			rawResList = append(rawResList, cfg.Proxies.ResidentialProxies...)
		}
	}
	if singleRes := os.Getenv("RESIDENTIAL_PROXY_URL"); singleRes != "" {
		rawResList = append(rawResList, singleRes)
	}
	if envResList := os.Getenv("RESIDENTIAL_PROXIES"); envResList != "" {
		rawResList = append(rawResList, strings.Split(envResList, ",")...)
	}
	if len(rawResList) == 0 {
		if fileBytes, err := os.ReadFile("residential_proxies.txt"); err == nil {
			lines := strings.Split(string(fileBytes), "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
					rawResList = append(rawResList, trimmed)
				}
			}
		}
	}
	for _, p := range rawResList {
		p = normalizeProxyURL(p)
		if p != "" {
			residentialProxies = append(residentialProxies, p)
		}
	}

	if len(proxies) > 0 {
		log.Printf("[Proxy] Loaded %d datacenter/rotating proxies in pool", len(proxies))
	}
	if len(residentialProxies) > 0 {
		log.Printf("[Proxy] Loaded %d residential proxies in pool", len(residentialProxies))
	}
}

func normalizeProxyURL(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "http://") && !strings.HasPrefix(p, "https://") && !strings.HasPrefix(p, "socks5://") {
		p = "http://" + p
	}
	return p
}

// HasProxies returns true if any rotating or residential proxy is configured.
func HasProxies() bool {
	return len(proxies) > 0 || len(residentialProxies) > 0
}

// HasResidentialProxies returns true if residential proxies are specifically available.
func HasResidentialProxies() bool {
	return len(residentialProxies) > 0
}

// GetNextProxy returns the next rotating proxy in round-robin fashion.
func GetNextProxy() (*url.URL, error) {
	if len(proxies) == 0 {
		if len(residentialProxies) > 0 {
			return GetNextResidentialProxy()
		}
		return nil, nil
	}
	idx := atomic.AddUint32(&proxyIndex, 1) - 1
	selected := proxies[idx%uint32(len(proxies))]
	return url.Parse(selected)
}

// GetNextResidentialProxy retrieves a dedicated residential proxy from the pool.
func GetNextResidentialProxy() (*url.URL, error) {
	if len(residentialProxies) == 0 {
		// Fallback to standard proxy pool if no dedicated residential proxy is present
		return GetNextProxy()
	}
	idx := atomic.AddUint32(&resProxyIndex, 1) - 1
	selected := residentialProxies[idx%uint32(len(residentialProxies))]
	return url.Parse(selected)
}

func getNextProxy(req *http.Request) (*url.URL, error) {
	return GetNextProxy()
}

// NewProxyTransport builds an HTTP transport routed explicitly through a proxy.
func NewProxyTransport(proxyURL *url.URL) *http.Transport {
	if proxyURL == nil {
		return newBrowserTransport(nil)
	}
	return newBrowserTransport(http.ProxyURL(proxyURL))
}

// NewDirectTransport builds an unproxied, direct HTTP transport with browser TLS emulation.
func NewDirectTransport() *http.Transport {
	return newBrowserTransport(nil)
}
