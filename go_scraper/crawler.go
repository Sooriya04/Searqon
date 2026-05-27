package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ─── Robots.txt Structures and Cache ─────────────────────────────────────────

type RobotsData struct {
	DisallowedPaths []string
	CrawlDelay      time.Duration
}

var robotsCache = make(map[string]*RobotsData)
var robotsMu sync.RWMutex

func getRobotsData(baseURL *url.URL) *RobotsData {
	host := baseURL.Host
	robotsMu.RLock()
	data, exists := robotsCache[host]
	robotsMu.RUnlock()
	if exists {
		return data
	}

	robotsURL := fmt.Sprintf("%s://%s/robots.txt", baseURL.Scheme, host)
	data = &RobotsData{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err == nil {
		for key, value := range defaultHeaders {
			req.Header.Set(key, value)
		}
		resp, rerr := httpClient.Do(req)
		if rerr == nil {
			defer resp.Body.Close()
			if resp.StatusCode == 200 {
				body, berr := io.ReadAll(resp.Body)
				if berr == nil {
					lines := strings.Split(string(body), "\n")
					isApplicable := false
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
							if val == "*" || strings.Contains(strings.ToLower(val), "searqon") {
								isApplicable = true
							} else {
								isApplicable = false
							}
						}

						if isApplicable {
							if key == "disallow" {
								if val != "" {
									data.DisallowedPaths = append(data.DisallowedPaths, val)
								}
							} else if key == "crawl-delay" {
								var delaySec float64
								if _, derr := fmt.Sscanf(val, "%f", &delaySec); derr == nil {
									data.CrawlDelay = time.Duration(delaySec * float64(time.Second))
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

func isAllowed(targetURL string, robotsData *RobotsData) bool {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return false
	}
	path := parsed.Path
	if path == "" {
		path = "/"
	}
	for _, dis := range robotsData.DisallowedPaths {
		if dis == "/" {
			return false
		}
		if dis != "" && strings.HasPrefix(path, dis) {
			return false
		}
	}
	return true
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
	if !isAllowed(targetURL, robotsData) {
		result.Error = "disallowed by robots.txt"
		return result
	}

	htmlContent, _, _, err := fetchHTML(targetURL)
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

				// Respect Crawl Delay if specified in robots.txt
				if robotsData.CrawlDelay > 0 {
					time.Sleep(robotsData.CrawlDelay)
				}

				var scraped ScrapeResult
				var htmlContent string

				// Check robots.txt allowance
				if !isAllowed(task.url, robotsData) {
					scraped = ScrapeResult{
						URL:       task.url,
						Error:     "disallowed by robots.txt",
						StartTime: time.Now().UTC().Format(time.RFC3339),
						EndTime:   time.Now().UTC().Format(time.RFC3339),
					}
				} else {
					scraped, htmlContent = scrapeSingleURL(task.url, format)
				}

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
