package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ─── Robots.txt Structures and Cache ─────────────────────────────────────────

type AgentRules struct {
	DisallowedPaths []string
	AllowedPaths    []string
	CrawlDelay      time.Duration
}

type RobotsData struct {
	Rules map[string]*AgentRules // key: lowercase user-agent name
}

var robotsCache = make(map[string]*RobotsData)
var robotsMu sync.RWMutex

var agentUserAgents = map[string]string{
	"googlebot":   "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
	"bingbot":     "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
	"slurp":       "Mozilla/5.0 (compatible; Yahoo! Slurp; http://help.yahoo.com/help/us/ysearch/slurp)",
	"duckduckbot": "Mozilla/5.0 (compatible; DuckDuckBot-Https/1.1; +http://duckduckgo.com/duckduckbot.html)",
	"baiduspider": "Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)",
	"yandexbot":   "Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)",
}

var rotatingUserAgents = []string{
	// Chrome Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	// Chrome macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	// Firefox Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:123.0) Gecko/20100101 Firefox/123.0",
	// Firefox macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:123.0) Gecko/20100101 Firefox/123.0",
	// Safari macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
	// Safari iOS
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Mobile/15E148 Safari/605.1.15",
	// Chrome Android
	"Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Mobile Safari/537.36",
}

func getRobotsData(baseURL *url.URL) *RobotsData {
	host := baseURL.Host
	robotsMu.RLock()
	data, exists := robotsCache[host]
	robotsMu.RUnlock()
	if exists {
		return data
	}

	robotsURL := fmt.Sprintf("%s://%s/robots.txt", baseURL.Scheme, host)
	data = &RobotsData{
		Rules: make(map[string]*AgentRules),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "SearqonBot/1.0 (+https://sooriya04.github.io/Searqon/)")
		resp, rerr := httpClient.Do(req)
		if rerr == nil {
			defer resp.Body.Close()
			if resp.StatusCode == 200 {
				body, berr := io.ReadAll(resp.Body)
				if berr == nil {
					lines := strings.Split(string(body), "\n")
					
					var activeAgents []string
					lastWasAgent := false

					for _, line := range lines {
						line = strings.TrimSpace(line)
						if line == "" || strings.HasPrefix(line, "#") {
							continue
						}
						parts := strings.SplitN(line, ":", 2)
						if len(parts) < 2 {
							continue
						}
						key := strings.ToLower(strings.TrimSpace(parts[0]))
						val := strings.TrimSpace(parts[1])

						if key == "user-agent" {
							if !lastWasAgent {
								activeAgents = []string{}
							}
							agent := strings.ToLower(val)
							activeAgents = append(activeAgents, agent)
							if _, exists := data.Rules[agent]; !exists {
								data.Rules[agent] = &AgentRules{}
							}
							lastWasAgent = true
						} else {
							lastWasAgent = false
							for _, agent := range activeAgents {
								rules := data.Rules[agent]
								if key == "disallow" {
									if val != "" {
										rules.DisallowedPaths = append(rules.DisallowedPaths, val)
									}
								} else if key == "allow" {
									if val != "" {
										rules.AllowedPaths = append(rules.AllowedPaths, val)
									}
								} else if key == "crawl-delay" {
									var delaySec float64
									if _, derr := fmt.Sscanf(val, "%f", &delaySec); derr == nil {
										rules.CrawlDelay = time.Duration(delaySec * float64(time.Second))
									}
								}
							}
						}
					}
				}
			}
		}
	}

	robotsMu.Lock()
	robotsCache[host] = data
	robotsMu.Unlock()
	return data
}

func isAgentAllowed(targetURL string, agent string, robotsData *RobotsData) (bool, time.Duration) {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return false, 0
	}
	path := parsed.Path
	if path == "" {
		path = "/"
	}

	rules, exists := robotsData.Rules[agent]
	if !exists {
		return true, 0
	}

	longestAllow := -1
	for _, allow := range rules.AllowedPaths {
		if strings.HasPrefix(path, allow) {
			if len(allow) > longestAllow {
				longestAllow = len(allow)
			}
		}
	}

	longestDisallow := -1
	for _, disallow := range rules.DisallowedPaths {
		if strings.HasPrefix(path, disallow) {
			if len(disallow) > longestDisallow {
				longestDisallow = len(disallow)
			}
		}
	}

	if longestAllow > longestDisallow {
		return true, rules.CrawlDelay
	}
	if longestDisallow > longestAllow {
		return false, rules.CrawlDelay
	}

	return true, rules.CrawlDelay
}

func findAllowedAgent(targetURL string, robotsData *RobotsData) (string, time.Duration, bool) {
	// 1. If wildcard "*" is allowed, rotate realistic browser User-Agents!
	if allowed, delay := isAgentAllowed(targetURL, "*", robotsData); allowed {
		ua := rotatingUserAgents[rand.Intn(len(rotatingUserAgents))]
		return ua, delay, true
	}

	// 2. If wildcard is blocked but 'searqonbot' or 'searqon' is explicitly whitelisted, use it
	if allowed, delay := isAgentAllowed(targetURL, "searqonbot", robotsData); allowed {
		return "SearqonBot/1.0 (+https://sooriya04.github.io/Searqon/)", delay, true
	}
	if allowed, delay := isAgentAllowed(targetURL, "searqon", robotsData); allowed {
		return "SearqonBot/1.0 (+https://sooriya04.github.io/Searqon/)", delay, true
	}

	searchAgents := []string{"googlebot", "bingbot", "slurp", "duckduckbot", "baiduspider", "yandexbot"}
	for _, agent := range searchAgents {
		if _, exists := robotsData.Rules[agent]; exists {
			if allowed, delay := isAgentAllowed(targetURL, agent, robotsData); allowed {
				ua := agentUserAgents[agent]
				return ua, delay, true
			}
		}
	}

	for agent, ua := range agentUserAgents {
		if allowed, delay := isAgentAllowed(targetURL, agent, robotsData); allowed {
			return ua, delay, true
		}
	}

	return "", 0, false
}

func isAllowed(targetURL string, robotsData *RobotsData) bool {
	_, _, allowed := findAllowedAgent(targetURL, robotsData)
	return allowed
}

// ─── Site Mapper ─────────────────────────────────────────────────────────────

func mapSiteURLs(targetURL string, limit int) MapResult {
	startTime := time.Now()
	result := MapResult{SourceURL: targetURL}

	if limit <= 0 {
		limit = 100
	}

	parsedBase, err := url.Parse(targetURL)
	if err != nil {
		return result
	}
	baseDomain := parsedBase.Hostname()

	// Get robots.txt
	robotsData := getRobotsData(parsedBase)
	userAgent, delay, allowed := findAllowedAgent(targetURL, robotsData)
	if !allowed {
		result.Error = "disallowed by robots.txt"
		return result
	}
	if delay > 0 {
		time.Sleep(delay)
	}
	if userAgent == "" {
		userAgent = defaultHeaders["User-Agent"]
	}

	htmlContent, _, _, err := fetchHTML(targetURL, userAgent)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		result.Error = err.Error()
		return result
	}

	var links []MapLink
	seen := map[string]bool{targetURL: true}

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		if len(links) >= limit {
			return
		}
		href, ok := s.Attr("href")
		if !ok || href == "" || strings.HasPrefix(href, "#") {
			return
		}

		resolved, rerr := parsedBase.Parse(href)
		if rerr != nil || resolved.Hostname() != baseDomain {
			return
		}

		resolved.Fragment = ""
		cleanURL := resolved.String()
		if seen[cleanURL] {
			return
		}
		seen[cleanURL] = true

		title := strings.TrimSpace(s.Text())
		if title == "" {
			title, _ = s.Attr("title")
		}

		links = append(links, MapLink{URL: cleanURL, Title: title})
	})

	result.Links = links
	result.Count = len(links)
	result.Duration = time.Since(startTime).Milliseconds()
	log.Printf("[Go Scraper] Map: %s → %d links (%dms)", targetURL, result.Count, result.Duration)
	return result
}

// ─── Recursive Crawler ───────────────────────────────────────────────────────

func crawlSite(targetURL string, limit, depth int, format string, onPageScraped func(ScrapeResult)) CrawlResult {
	startTime := time.Now()

	if limit <= 0 {
		limit = 30
	}
	if depth <= 0 {
		depth = 2
	}

	result := CrawlResult{SourceURL: targetURL}

	parsedBase, err := url.Parse(targetURL)
	if err != nil {
		result.Error = "invalid URL"
		return result
	}
	baseDomain := parsedBase.Hostname()

	// Get robots.txt
	robotsData := getRobotsData(parsedBase)
	if !isAllowed(targetURL, robotsData) {
		result.Error = "root URL disallowed by robots.txt"
		return result
	}

	type crawlTask struct {
		url   string
		depth int
	}

	// Channels
	jobs := make(chan crawlTask, limit*10)
	discoveredLinksChan := make(chan []crawlTask, limit*10)

	var mu sync.Mutex
	var pages []ScrapeResult
	visited := map[string]bool{targetURL: true}

	// Counters for tracking progress
	var activeWorkers int32
	var pendingTasks int32 = 1 // Starts with the root URL

	// Limit concurrency to 5 workers (matches Spider worker concurrency setup)
	numWorkers := 5
	for i := 0; i < numWorkers; i++ {
		go func() {
			for task := range jobs {
				atomic.AddInt32(&activeWorkers, 1)

				var scraped ScrapeResult
				var htmlContent string

				scraped, htmlContent = scrapeSingleURL(task.url, format, false)

				mu.Lock()
				pages = append(pages, scraped)
				mu.Unlock()

				// Callback to stream result to Node/SSE in real-time
				if onPageScraped != nil {
					onPageScraped(scraped)
				}

				// Discover links
				var newLinks []crawlTask
				if task.depth < depth && scraped.Error == "" && htmlContent != "" {
					doc, derr := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
					if derr == nil {
						doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
							href, ok := s.Attr("href")
							if !ok || href == "" || strings.HasPrefix(href, "#") {
								return
							}
							resolved, rerr := parsedBase.Parse(href)
							if rerr != nil || resolved.Hostname() != baseDomain {
								return
							}
							resolved.Fragment = ""
							cleanURL := resolved.String()
							newLinks = append(newLinks, crawlTask{url: cleanURL, depth: task.depth + 1})
						})
					}
				}

				if len(newLinks) > 0 {
					discoveredLinksChan <- newLinks
				}

				atomic.AddInt32(&activeWorkers, -1)
				// Decrement pending tasks
				atomic.AddInt32(&pendingTasks, -1)
			}
		}()
	}

	// Coordinator: Push initial URL
	jobs <- crawlTask{url: targetURL, depth: 0}

	// Loop to coordinate tasks
	for atomic.LoadInt32(&pendingTasks) > 0 {
		select {
		case newLinks := <-discoveredLinksChan:
			var addedCount int32
			mu.Lock()
			for _, link := range newLinks {
				if !visited[link.url] && len(visited) < limit {
					visited[link.url] = true
					addedCount++
					// Push to jobs channel
					jobs <- link
				}
			}
			mu.Unlock()
			if addedCount > 0 {
				atomic.AddInt32(&pendingTasks, addedCount)
			}
		case <-time.After(20 * time.Millisecond):
			// Keep loop checking
		}
	}

	// Close jobs to terminate workers
	close(jobs)

	result.Pages = pages
	result.Total = len(pages)
	result.Duration = time.Since(startTime).Milliseconds()
	log.Printf("[Go Scraper] Crawl: %s → %d pages (%dms)", targetURL, result.Total, result.Duration)
	return result
}
