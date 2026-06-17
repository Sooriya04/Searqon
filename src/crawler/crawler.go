package crawler

import (
	"log"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"

	"src/models"
	"src/scraper"
)

// MapSiteURLs maps all internal links on a given website domain.
func MapSiteURLs(targetURL string, limit int) models.MapResult {
	startTime := time.Now()
	result := models.MapResult{SourceURL: targetURL}

	if limit <= 0 {
		limit = 100
	}

	parsedBase, err := url.Parse(targetURL)
	if err != nil {
		return result
	}
	baseDomain := parsedBase.Hostname()

	userAgent, delay, allowed := scraper.GetRobotsDataAndAgent(targetURL)
	if !allowed {
		result.Error = "disallowed by robots.txt"
		return result
	}
	if delay > 0 {
		time.Sleep(delay)
	}

	htmlContent, _, _, _, err := scraper.FetchHTML(targetURL, userAgent)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		result.Error = err.Error()
		return result
	}

	var links []models.MapLink
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

		links = append(links, models.MapLink{URL: cleanURL, Title: title})
	})

	result.Links = links
	result.Count = len(links)
	result.Duration = time.Since(startTime).Milliseconds()
	log.Printf("[Crawler] Map complete: %s → %d links (%dms)", targetURL, result.Count, result.Duration)
	return result
}

// CrawlSite crawls a target URL recursively.
func CrawlSite(targetURL string, limit, depth int, format string, onPageScraped func(models.ScrapeResult)) models.CrawlResult {
	startTime := time.Now()

	if limit <= 0 {
		limit = 30
	}
	if depth <= 0 {
		depth = 2
	}

	result := models.CrawlResult{SourceURL: targetURL}

	parsedBase, err := url.Parse(targetURL)
	if err != nil {
		result.Error = "invalid URL"
		return result
	}
	baseDomain := parsedBase.Hostname()

	if !scraper.IsAllowed(targetURL) {
		result.Error = "root URL disallowed by robots.txt"
		return result
	}

	type crawlTask struct {
		url   string
		depth int
	}

	jobs := make(chan crawlTask, limit*10)
	discoveredLinksChan := make(chan []crawlTask, limit*10)

	var mu sync.Mutex
	var pages []models.ScrapeResult
	visited := map[string]bool{targetURL: true}

	var activeWorkers int32
	var pendingTasks int32 = 1

	numWorkers := 5
	for i := 0; i < numWorkers; i++ {
		go func() {
			for task := range jobs {
				atomic.AddInt32(&activeWorkers, 1)

				scraped, htmlContent := scraper.ScrapeSingleURLNative(task.url, format, false)

				mu.Lock()
				pages = append(pages, scraped)
				mu.Unlock()

				if onPageScraped != nil {
					onPageScraped(scraped)
				}

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
				atomic.AddInt32(&pendingTasks, -1)
			}
		}()
	}

	jobs <- crawlTask{url: targetURL, depth: 0}

	for atomic.LoadInt32(&pendingTasks) > 0 {
		select {
		case newLinks := <-discoveredLinksChan:
			var addedCount int32
			mu.Lock()
			for _, link := range newLinks {
				if !visited[link.url] && len(visited) < limit {
					visited[link.url] = true
					addedCount++
					jobs <- link
				}
			}
			mu.Unlock()
			if addedCount > 0 {
				atomic.AddInt32(&pendingTasks, addedCount)
			}
		case <-time.After(20 * time.Millisecond):
		}
	}

	close(jobs)

	result.Pages = pages
	result.Total = len(pages)
	result.Duration = time.Since(startTime).Milliseconds()
	log.Printf("[Crawler] Crawl complete: %s → %d pages (%dms)", targetURL, result.Total, result.Duration)
	return result
}
