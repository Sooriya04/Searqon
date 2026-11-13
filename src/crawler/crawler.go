package crawler

import (
	"context"
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
	return MapSiteURLsWithContext(context.Background(), targetURL, limit)
}

func MapSiteURLsWithContext(ctx context.Context, targetURL string, limit int) models.MapResult {
	startTime := time.Now()
	result := models.MapResult{SourceURL: targetURL}

	if err := ctx.Err(); err != nil {
		result.Error = err.Error()
		return result
	}

	if limit <= 0 {
		limit = 100
	}

	parsedBase, err := url.Parse(targetURL)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	baseDomain := parsedBase.Hostname()

	userAgent, delay, allowed := scraper.GetRobotsDataAndAgent(targetURL)
	if !allowed {
		result.Error = "disallowed by robots.txt"
		return result
	}
	if delay > 0 {
		select {
		case <-ctx.Done():
			result.Error = ctx.Err().Error()
			return result
		case <-time.After(delay):
		}
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
	return CrawlSiteWithContext(context.Background(), targetURL, limit, depth, format, onPageScraped)
}

func CrawlSiteWithContext(ctx context.Context, targetURL string, limit, depth int, format string, onPageScraped func(models.ScrapeResult)) models.CrawlResult {
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

	var mu sync.Mutex
	var pages []models.ScrapeResult
	visited := map[string]bool{targetURL: true}

	var activeWorkers int32
	var pendingTasks int32 = 1
	var wg sync.WaitGroup

	crawlCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	numWorkers := 5
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				var task crawlTask
				select {
				case <-crawlCtx.Done():
					return
				case t, ok := <-jobs:
					if !ok {
						return
					}
					task = t
				}

				atomic.AddInt32(&activeWorkers, 1)

				scraped, htmlContent := scraper.ScrapeSingleURLNative(task.url, format, false)

				mu.Lock()
				pages = append(pages, scraped)
				mu.Unlock()

				if onPageScraped != nil {
					onPageScraped(scraped)
				}

				var newLinks []crawlTask
				if task.depth < depth && scraped.Error == "" {
					addLink := func(href string) {
						if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
							return
						}
						resolved, rerr := parsedBase.Parse(href)
						if rerr != nil || resolved.Hostname() != baseDomain {
							return
						}
						resolved.Fragment = ""
						cleanURL := resolved.String()
						newLinks = append(newLinks, crawlTask{url: cleanURL, depth: task.depth + 1})
					}

					if htmlContent != "" {
						doc, derr := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
						if derr == nil {
							doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
								if href, ok := s.Attr("href"); ok {
									addLink(href)
								}
							})
						}
					} else {
						// Cache hit fallback: use OutboundLinks or extract from markdown/content
						if len(scraped.OutboundLinks) > 0 {
							for _, link := range scraped.OutboundLinks {
								addLink(link)
							}
						}
						textToScan := scraped.Markdown
						if textToScan == "" {
							textToScan = scraped.Content
						}
						for _, token := range strings.Fields(textToScan) {
							if strings.HasPrefix(token, "http://") || strings.HasPrefix(token, "https://") || strings.HasPrefix(token, "/") {
								trimmed := strings.Trim(token, "()[]<>\"'.,;*")
								addLink(trimmed)
							}
						}
					}
				}

				if len(newLinks) > 0 {
					var toSchedule []crawlTask
					mu.Lock()
					for _, link := range newLinks {
						if !visited[link.url] && len(visited) < limit {
							visited[link.url] = true
							toSchedule = append(toSchedule, link)
						}
					}
					mu.Unlock()

					if len(toSchedule) > 0 {
						atomic.AddInt32(&pendingTasks, int32(len(toSchedule)))
						for _, item := range toSchedule {
							select {
							case jobs <- item:
							case <-crawlCtx.Done():
								atomic.AddInt32(&pendingTasks, -1)
							}
						}
					}
				}

				atomic.AddInt32(&activeWorkers, -1)
				if atomic.AddInt32(&pendingTasks, -1) == 0 {
					// All pending tasks finished and no more tasks remain
					close(jobs)
				}
			}
		}()
	}

	jobs <- crawlTask{url: targetURL, depth: 0}

	wg.Wait()

	if err := crawlCtx.Err(); err != nil && ctx.Err() != nil {
		result.Error = ctx.Err().Error()
	} else if err := crawlCtx.Err(); err != nil && len(pages) == 0 {
		result.Error = "crawl timeout exceeded"
	}

	result.Pages = pages
	result.Total = len(pages)
	result.Duration = time.Since(startTime).Milliseconds()
	log.Printf("[Crawler] Crawl complete: %s → %d pages (%dms)", targetURL, result.Total, result.Duration)
	return result
}
