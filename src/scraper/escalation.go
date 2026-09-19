package scraper

import (
	"compress/flate"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"src/models"
	"src/utils"
)

// EscalationTier represents the current stealth and evasion level in the scraper escalation ladder.
type EscalationTier string

const (
	TierFastHTTP         EscalationTier = "fast_http"
	TierSpoofedHeaders   EscalationTier = "spoofed_headers"
	TierHeadlessBrowser  EscalationTier = "headless_browser"
	TierResidentialProxy EscalationTier = "residential_proxy"
)

// fetchHTTPRaw performs a single HTTP request with custom transport, headers, and decompression.
func fetchHTTPRaw(targetURL string, headers map[string]string, transport *http.Transport, timeout time.Duration) (string, int, string, *url.URL, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil || parsedURL.Scheme == "" {
		return "", 0, "", nil, fmt.Errorf("invalid URL: %s", targetURL)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_ = WaitDomainRateLimit(ctx, parsedURL.Hostname())

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return "", 0, "", nil, fmt.Errorf("request build failed: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, "", nil, err
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL
	statusCode := resp.StatusCode
	contentType := resp.Header.Get("Content-Type")

	var reader io.Reader = resp.Body
	switch strings.ToLower(resp.Header.Get("Content-Encoding")) {
	case "gzip":
		if gz, err := gzip.NewReader(resp.Body); err == nil {
			defer gz.Close()
			reader = gz
		}
	case "br":
		reader = brotli.NewReader(resp.Body)
	case "deflate":
		reader = flate.NewReader(resp.Body)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(reader, 8*1024*1024))
	if err != nil && len(bodyBytes) == 0 {
		return "", statusCode, contentType, finalURL, fmt.Errorf("read body failed: %w", err)
	}

	return string(bodyBytes), statusCode, contentType, finalURL, nil
}

// ExecuteSmartEscalationScrape executes the multi-tier anti-bot escalation ladder:
// Tier 1: Fast HTTP (direct, lightweight, low-latency)
// Tier 2: Anti-bot Spoofed Headers (coherent browser persona, client hints, modern TLS)
// Tier 3: Headless Browser (Lightpanda or Camoufox for client-side JS / dynamic challenge execution)
// Tier 4: Residential / Rotating Proxy Fallback (route via proxy pool with spoofed headers)
func ExecuteSmartEscalationScrape(targetURL string, userAgent string, opts ScrapeOptions, startTime time.Time) (models.ScrapeResult, string, error) {
	level := strings.ToLower(opts.StealthLevel)
	if level == "" {
		level = "auto"
	}

	var lastErrSummary []string
	botEncountered := false

	// =========================================================================
	// TIER 1: Fast HTTP (Direct, lightweight, ~50-150ms)
	// =========================================================================
	if level == "auto" || level == "fast_http" {
		t1Start := time.Now()
		headers := GetFastHTTPHeaders(targetURL)
		if userAgent != "" {
			headers["User-Agent"] = userAgent
		}

		directTransport := NewDirectTransport()
		body, statusCode, contentType, finalURL, err := fetchHTTPRaw(targetURL, headers, directTransport, 2500*time.Millisecond)

		if err == nil {
			block := DetectBotChallenge(statusCode, body, contentType)
			if !block.IsBlocked && statusCode < 400 && utils.CountWords(body) >= 20 {
				log.Printf("[Escalation] Tier 1 (Fast HTTP) succeeded in %dms for %s", time.Since(t1Start).Milliseconds(), targetURL)

				finalURLStr := targetURL
				if finalURL != nil {
					finalURLStr = finalURL.String()
				}

				parsed := ScrapeHTMLContentWithSchema(body, targetURL, finalURLStr, opts.Format, startTime, opts.ExtractSchema)
				parsed.StatusCode = statusCode
				parsed.ContentType = contentType
				parsed.RenderMethod = "go"
				parsed.EscalationTier = string(TierFastHTTP)
				parsed.BotDetected = false
				return parsed, body, nil
			}

			botEncountered = true
			diag := fmt.Sprintf("status=%d, block=%s (%s)", statusCode, block.Category, block.Details)
			lastErrSummary = append(lastErrSummary, fmt.Sprintf("Tier 1 (Fast HTTP): %s", diag))
			log.Printf("[Escalation] Tier 1 blocked/challenged [%s]. Escalating to Tier 2 (Spoofed Headers)...", diag)
		} else {
			lastErrSummary = append(lastErrSummary, fmt.Sprintf("Tier 1 network error: %v", err))
			log.Printf("[Escalation] Tier 1 network failed (%v). Escalating to Tier 2 (Spoofed Headers)...", err)
		}

		if level == "fast_http" {
			return models.ScrapeResult{}, "", fmt.Errorf("tier 1 fast_http failed: %s", strings.Join(lastErrSummary, "; "))
		}
	}

	// =========================================================================
	// TIER 2: Anti-Bot Spoofed Headers (Browser Persona, Client Hints, TLS 1.3)
	// =========================================================================
	if level == "auto" || level == "spoofed_headers" {
		t2Start := time.Now()
		persona := PickRandomPersona()
		if userAgent != "" {
			persona.UserAgent = userAgent
		}
		headers := GetStealthSpoofedHeaders(targetURL, &persona)

		directTransport := NewDirectTransport()
		body, statusCode, contentType, finalURL, err := fetchHTTPRaw(targetURL, headers, directTransport, 4000*time.Millisecond)

		if err == nil {
			block := DetectBotChallenge(statusCode, body, contentType)
			if !block.IsBlocked && statusCode < 400 && utils.CountWords(body) >= 20 {
				log.Printf("[Escalation] Tier 2 (Spoofed Headers - %s) bypassed challenge in %dms for %s", persona.Name, time.Since(t2Start).Milliseconds(), targetURL)

				finalURLStr := targetURL
				if finalURL != nil {
					finalURLStr = finalURL.String()
				}

				parsed := ScrapeHTMLContentWithSchema(body, targetURL, finalURLStr, opts.Format, startTime, opts.ExtractSchema)
				parsed.StatusCode = statusCode
				parsed.ContentType = contentType
				parsed.RenderMethod = "go"
				parsed.EscalationTier = string(TierSpoofedHeaders)
				parsed.BotDetected = true
				return parsed, body, nil
			}

			botEncountered = true
			diag := fmt.Sprintf("status=%d, block=%s (%s)", statusCode, block.Category, block.Details)
			lastErrSummary = append(lastErrSummary, fmt.Sprintf("Tier 2 (Spoofed Headers): %s", diag))
			log.Printf("[Escalation] Tier 2 blocked or requires JS [%s]. Escalating to Tier 3 (Headless Browser)...", diag)
		} else {
			lastErrSummary = append(lastErrSummary, fmt.Sprintf("Tier 2 network error: %v", err))
			log.Printf("[Escalation] Tier 2 network failed (%v). Escalating to Tier 3 (Headless Browser)...", err)
		}

		if level == "spoofed_headers" {
			return models.ScrapeResult{}, "", fmt.Errorf("tier 2 spoofed_headers failed: %s", strings.Join(lastErrSummary, "; "))
		}
	}

	// =========================================================================
	// TIER 3: Headless Browser (Camoufox / Lightpanda for JS execution)
	// =========================================================================
	if (!opts.ForceNative && level == "auto") || level == "headless" || level == "browser" {
		t3Start := time.Now()

		// 1. Try Camoufox (Stealth Firefox Anti-Detect Browser)
		if camoufoxEnabled, _ := LoadCamoufoxConfig(); camoufoxEnabled {
			res, raw, cErr := ScrapeWithCamoufox(targetURL, "", opts.Format, startTime, opts.ExtractSchema)
			if cErr == nil {
				block := DetectBotChallenge(res.StatusCode, raw, res.ContentType)
				if !block.IsBlocked && res.WordCount >= 20 {
					log.Printf("[Escalation] Tier 3 (Camoufox) rendered page in %dms for %s", time.Since(t3Start).Milliseconds(), targetURL)
					res.EscalationTier = string(TierHeadlessBrowser)
					res.BotDetected = true
					return res, raw, nil
				}
				lastErrSummary = append(lastErrSummary, fmt.Sprintf("Tier 3 (Camoufox) challenge detected: %s", block.Details))
			} else {
				lastErrSummary = append(lastErrSummary, fmt.Sprintf("Tier 3 (Camoufox) error: %v", cErr))
			}
		}

		// 2. Try Lightpanda Headless Browser
		if lightpandaEnabled, lpPath := utils.LoadLightpandaConfig(); lightpandaEnabled && lpPath != "" {
			res, raw, lpErr := ScrapeWithLightpanda(targetURL, userAgent, lpPath, opts.Format, startTime, opts.ExtractSchema)
			if lpErr == nil {
				block := DetectBotChallenge(res.StatusCode, raw, res.ContentType)
				if !block.IsBlocked && res.WordCount >= 20 {
					log.Printf("[Escalation] Tier 3 (Lightpanda) rendered page in %dms for %s", time.Since(t3Start).Milliseconds(), targetURL)
					res.EscalationTier = string(TierHeadlessBrowser)
					res.BotDetected = true
					return res, raw, nil
				}
				lastErrSummary = append(lastErrSummary, fmt.Sprintf("Tier 3 (Lightpanda) challenge detected: %s", block.Details))
			} else {
				lastErrSummary = append(lastErrSummary, fmt.Sprintf("Tier 3 (Lightpanda) error: %v", lpErr))
			}
		}

		log.Printf("[Escalation] Tier 3 headless browser failed or unavailable. Escalating to Tier 4 (Residential Proxy)...")

		if level == "headless" || level == "browser" {
			return models.ScrapeResult{}, "", fmt.Errorf("tier 3 headless browser failed: %s", strings.Join(lastErrSummary, "; "))
		}
	}

	// =========================================================================
	// TIER 4: Residential / Rotating Proxy Fallback
	// =========================================================================
	if level == "auto" || level == "proxy" {
		t4Start := time.Now()

		if HasProxies() {
			// Prefer dedicated residential proxy over standard datacenter proxy
			proxyURL, pErr := GetNextResidentialProxy()
			if pErr == nil && proxyURL != nil {
				proxyStr := proxyURL.String()

				// If Camoufox or Lightpanda is available, try running headless browser through the proxy
				if !opts.ForceNative {
					if camoufoxEnabled, _ := LoadCamoufoxConfig(); camoufoxEnabled {
						if res, raw, cErr := ScrapeWithCamoufox(targetURL, proxyStr, opts.Format, startTime, opts.ExtractSchema); cErr == nil {
							block := DetectBotChallenge(res.StatusCode, raw, res.ContentType)
							if !block.IsBlocked && res.WordCount >= 20 {
								log.Printf("[Escalation] Tier 4 (Camoufox + Residential Proxy) succeeded in %dms for %s", time.Since(t4Start).Milliseconds(), targetURL)
								res.EscalationTier = string(TierResidentialProxy)
								res.BotDetected = true
								return res, raw, nil
							}
						}
					}

					if lightpandaEnabled, lpPath := utils.LoadLightpandaConfig(); lightpandaEnabled && lpPath != "" {
						if res, raw, lpErr := ScrapeWithLightpandaProxy(targetURL, userAgent, proxyStr, lpPath, opts.Format, startTime, opts.ExtractSchema); lpErr == nil {
							block := DetectBotChallenge(res.StatusCode, raw, res.ContentType)
							if !block.IsBlocked && res.WordCount >= 20 {
								log.Printf("[Escalation] Tier 4 (Lightpanda + Residential Proxy) succeeded in %dms for %s", time.Since(t4Start).Milliseconds(), targetURL)
								res.EscalationTier = string(TierResidentialProxy)
								res.BotDetected = true
								return res, raw, nil
							}
						}
					}
				}

				// HTTP fetch over residential proxy with spoofed headers
				persona := PickRandomPersona()
				if userAgent != "" {
					persona.UserAgent = userAgent
				}
				headers := GetStealthSpoofedHeaders(targetURL, &persona)
				proxyTransport := NewProxyTransport(proxyURL)

				body, statusCode, contentType, finalURL, err := fetchHTTPRaw(targetURL, headers, proxyTransport, 6000*time.Millisecond)
				if err == nil {
					block := DetectBotChallenge(statusCode, body, contentType)
					if !block.IsBlocked && statusCode < 400 && utils.CountWords(body) >= 20 {
						log.Printf("[Escalation] Tier 4 (Residential Proxy HTTP) succeeded in %dms for %s", time.Since(t4Start).Milliseconds(), targetURL)

						finalURLStr := targetURL
						if finalURL != nil {
							finalURLStr = finalURL.String()
						}

						parsed := ScrapeHTMLContentWithSchema(body, targetURL, finalURLStr, opts.Format, startTime, opts.ExtractSchema)
						parsed.StatusCode = statusCode
						parsed.ContentType = contentType
						parsed.RenderMethod = "go"
						parsed.EscalationTier = string(TierResidentialProxy)
						parsed.BotDetected = true
						return parsed, body, nil
					}
					lastErrSummary = append(lastErrSummary, fmt.Sprintf("Tier 4 (Proxy HTTP) blocked: %s", block.Details))
				} else {
					lastErrSummary = append(lastErrSummary, fmt.Sprintf("Tier 4 (Proxy HTTP) error: %v", err))
				}
			}
		} else {
			lastErrSummary = append(lastErrSummary, "Tier 4 proxy skipped: no residential/rotating proxies configured")
		}
	}

	failMsg := fmt.Sprintf("anti-bot escalation failed: %s", strings.Join(lastErrSummary, " | "))
	log.Printf("[Escalation] All escalation tiers exhausted for %s: %s", targetURL, failMsg)

	fallbackRes := models.ScrapeResult{
		URL:             targetURL,
		StartTime:       startTime.UTC().Format(time.RFC3339),
		EndTime:         time.Now().UTC().Format(time.RFC3339),
		Duration:        time.Since(startTime).Milliseconds(),
		FetchDurationMS: int(time.Since(startTime).Milliseconds()),
		Error:           failMsg,
		Scraped:         false,
		BotDetected:     botEncountered,
	}

	return fallbackRes, "", fmt.Errorf("%s", failMsg)
}
