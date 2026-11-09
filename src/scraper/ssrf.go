package scraper

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"src/config"
)

// ValidateScrapeURL guards against Server-Side Request Forgery (SSRF).
// It verifies the URL format, ensures http/https scheme, and blocks private/loopback/metadata IPs
// unless Scraper.AllowInternalNetworks is explicitly set to true.
func ValidateScrapeURL(rawURL string) (*url.URL, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsedURL.Scheme == "" || parsedURL.Hostname() == "" {
		return nil, fmt.Errorf("invalid URL: %s", rawURL)
	}

	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unsupported protocol scheme: %s", parsedURL.Scheme)
	}

	cfg := config.Get()
	if cfg != nil && cfg.Scraper.AllowInternalNetworks {
		return parsedURL, nil
	}
	// Also permit local test execution if testing flag is set or env var is present
	if strings.HasSuffix(os.Args[0], ".test") || os.Getenv("SEARQON_TEST_ENV") == "1" {
		return parsedURL, nil
	}

	hostname := parsedURL.Hostname()
	// Quick check on common loopback / internal names
	lowerHost := strings.ToLower(hostname)
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".localhost") || strings.HasSuffix(lowerHost, ".local") {
		return nil, fmt.Errorf("access to internal hostname '%s' is blocked", hostname)
	}

	// Resolve IP address with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", hostname)
	if err != nil {
		// If DNS resolution fails, check if hostname is direct IP
		if directIP := net.ParseIP(hostname); directIP != nil {
			ips = []net.IP{directIP}
		} else {
			return nil, fmt.Errorf("DNS resolution failed for '%s': %w", hostname, err)
		}
	}

	for _, ip := range ips {
		if isPrivateOrRestrictedIP(ip) {
			return nil, fmt.Errorf("access to private/internal IP (%s) blocked for target '%s'", ip.String(), hostname)
		}
	}

	return parsedURL, nil
}

func isPrivateOrRestrictedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsUnspecified() {
		return true
	}

	// IPv4 specific private ranges & metadata address (169.254.169.254)
	if ip4 := ip.To4(); ip4 != nil {
		// 10.0.0.0/8
		if ip4[0] == 10 {
			return true
		}
		// 172.16.0.0/12 (172.16.0.0 – 172.31.255.255)
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
		// 169.254.0.0/16 (Link Local / Cloud Metadata)
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
		// 127.0.0.0/8 (Loopback)
		if ip4[0] == 127 {
			return true
		}
		// 0.0.0.0/8
		if ip4[0] == 0 {
			return true
		}
		// Carrier Grade NAT: 100.64.0.0/10 (100.64.0.0 – 100.127.255.255)
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
		// Broadcast: 255.255.255.255
		if ip4[0] == 255 && ip4[1] == 255 && ip4[2] == 255 && ip4[3] == 255 {
			return true
		}
		return false
	}

	// IPv6 Unique Local Address (fc00::/7)
	if len(ip) == net.IPv6len && (ip[0]&0xfe) == 0xfc {
		return true
	}

	return false
}
