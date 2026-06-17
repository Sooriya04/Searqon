package models

import "time"

// SearchResult represents a single discovered search result.
type SearchResult struct {
	Title    string  `json:"title"`
	URL      string  `json:"url"`
	Snippet  string  `json:"snippet"`
	Source   string  `json:"source"`             // "searxng" | "duckduckgo" | "local_index"
	Content  string  `json:"content,omitempty"`  // scraped plain text
	Markdown string  `json:"markdown,omitempty"` // scraped markdown
	Scraped  bool    `json:"scraped"`
	Score    float64 `json:"score"`              // relevance score from ranker
}

// SearchResponse is the final search aggregation response.
type SearchResponse struct {
	Query    string         `json:"query"`
	Results  []SearchResult `json:"results"`
	Total    int            `json:"total"`
	Duration int64          `json:"duration"`
	Provider string         `json:"provider"`
}

// ScrapeResult is the structure of a single page's full extraction.
type ScrapeResult struct {
	Title            string         `json:"title"`
	Content          string         `json:"content"`            // plain text (always present)
	Markdown         string         `json:"markdown,omitempty"` // markdown formatted version
	URL              string         `json:"url"`
	WordCount        int            `json:"wordCount"`
	StartTime        string         `json:"startTime"`
	EndTime          string         `json:"endTime"`
	Duration         int64          `json:"duration"` // ms
	Error            string         `json:"error,omitempty"`
	CanonicalURL     string         `json:"canonicalUrl,omitempty"`
	Domain           string         `json:"domain"`
	Description      string         `json:"description,omitempty"`
	Author           string         `json:"author,omitempty"`
	PublishedAt      *time.Time     `json:"publishedAt,omitempty"`
	Language         string         `json:"language,omitempty"`
	OutboundLinks    []string       `json:"outboundLinks,omitempty"`
	StatusCode       int            `json:"statusCode,omitempty"`
	ContentType      string         `json:"contentType,omitempty"`
	Scraped          bool           `json:"scraped"`
	ExtractionMethod string         `json:"extractionMethod,omitempty"`
	FetchDurationMS  int            `json:"fetchDurationMs,omitempty"`
	Images           []ScrapedImage `json:"images,omitempty"`
}

// ScrapedImage holds details about an extracted image.
type ScrapedImage struct {
	URL string `json:"url"`
	Alt string `json:"alt,omitempty"`
}

// MapLink is a discovered link URL and its anchor text.
type MapLink struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

// MapResult represents the link mapping of a URL.
type MapResult struct {
	SourceURL string    `json:"sourceUrl"`
	Links     []MapLink `json:"links"`
	Count     int       `json:"count"`
	Duration  int64     `json:"duration"`
	Error     string    `json:"error,omitempty"`
}

// CrawlResult holds all crawled pages.
type CrawlResult struct {
	SourceURL string         `json:"sourceUrl"`
	Pages     []ScrapeResult `json:"pages"`
	Total     int            `json:"total"`
	Duration  int64          `json:"duration"`
	Error     string         `json:"error,omitempty"`
}

// API Requests
type SearchRequest struct {
	Query       string `json:"query"`
	Limit       int    `json:"limit"`
	Scrape      *bool  `json:"scrape"`
	BypassCache bool   `json:"bypass_cache"`
	MaxWords    int    `json:"max_words"`
	Summarize   bool   `json:"summarize"`
}

type ScrapeRequest struct {
	URL         string `json:"url"`
	Format      string `json:"format"`
	BypassCache bool   `json:"bypass_cache"`
	MaxWords    int    `json:"max_words"`
}

type BatchScrapeRequest struct {
	URLs        []string `json:"urls"`
	Format      string   `json:"format"`
	BypassCache bool     `json:"bypass_cache"`
	MaxWords    int      `json:"max_words"`
}

type MapRequest struct {
	URL   string `json:"url"`
	Limit int    `json:"limit"`
}

type CrawlRequest struct {
	URL      string `json:"url"`
	Limit    int    `json:"limit"`
	Depth    int    `json:"depth"`
	Format   string `json:"format"`
	Stream   bool   `json:"stream"`
	MaxWords int    `json:"max_words"`
}

type HTMLScrapeRequest struct {
	HTML     string `json:"html"`
	URL      string `json:"url"`
	Format   string `json:"format"`
	MaxWords int    `json:"max_words"`
}
