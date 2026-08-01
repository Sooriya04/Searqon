package scraper

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
)

var (
	proxies    []string
	proxyIndex uint32
)

// InitProxyPool configures the rotating proxy list.
func InitProxyPool() {
	var rawList []string

	if envList := os.Getenv("ROTATING_PROXIES"); envList != "" {
		rawList = strings.Split(envList, ",")
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
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, "http://") && !strings.HasPrefix(p, "https://") && !strings.HasPrefix(p, "socks5://") {
			p = "http://" + p
		}
		proxies = append(proxies, p)
	}

	if len(proxies) > 0 {
		log.Printf("[Proxy] Loaded %d proxies in rotating pool", len(proxies))
		httpClient.Transport = newBrowserTransport(getNextProxy)
	}
}

func getNextProxy(req *http.Request) (*url.URL, error) {
	if len(proxies) == 0 {
		return nil, nil
	}

	idx := atomic.AddUint32(&proxyIndex, 1) - 1
	selected := proxies[idx%uint32(len(proxies))]

	parsed, err := url.Parse(selected)
	if err != nil {
		log.Printf("[Proxy] Invalid proxy URL %s: %v", selected, err)
		return nil, err
	}

	return parsed, nil
}
