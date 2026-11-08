package utcp

import (
	"strings"
)

// BuildManual constructs the authoritative UtcpManual for Searqon with endpoints rooted at baseURL.
func BuildManual(baseURL string) UtcpManual {
	cleanBase := strings.TrimRight(baseURL, "/")

	tools := []Tool{
		{
			Name: "searqon_search",
			Description: "Search the web using Searqon meta-search engine (SearXNG, DuckDuckGo, Arxiv, and Wikipedia) " +
				"with automatic reciprocal rank fusion and optional concurrent scraping of top results.",
			Inputs: ToolInputSchema{
				Type: "object",
				Properties: map[string]ToolParamProperty{
					"query": {
						Type:        "string",
						Description: "Search query string to execute across search providers.",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of search results to return (default: 5, max: 20).",
						Default:     5,
					},
					"scrape": {
						Type:        "boolean",
						Description: "If true, concurrently scrapes and returns clean Markdown text of the top results.",
						Default:     false,
					},
					"category": {
						Type:        "string",
						Description: "Search category or intent: 'general', 'science', 'it', 'news'.",
						Enum:        []string{"general", "science", "it", "news"},
						Default:     "general",
					},
					"max_tokens": {
						Type:        "integer",
						Description: "Maximum token budget for scraped previews per result (default: 4000).",
						Default:     4000,
					},
				},
				Required: []string{"query"},
			},
			ToolCallTemplate: CallTemplate{
				CallTemplateType: "http",
				URL:              cleanBase + "/search",
				HTTPMethod:       "POST",
				ContentType:      "application/json",
			},
		},
		{
			Name: "searqon_scrape",
			Description: "Ultra-sophisticated 4-tier anti-bot web scraper designed specifically for AI agents. " +
				"Extracts main readable content into clean Markdown (stripping navs, ads, scripts, and clutter).",
			Inputs: ToolInputSchema{
				Type: "object",
				Properties: map[string]ToolParamProperty{
					"url": {
						Type:        "string",
						Description: "The full target URL to scrape (e.g. 'https://example.com/article').",
					},
					"format": {
						Type:        "string",
						Description: "Output format: 'markdown' (recommended), 'text', or 'html'.",
						Enum:        []string{"markdown", "text", "html"},
						Default:     "markdown",
					},
					"max_tokens": {
						Type:        "integer",
						Description: "Maximum token budget for content truncation. Default: 8000.",
						Default:     8000,
					},
					"bypass_cache": {
						Type:        "boolean",
						Description: "Force live network retrieval, ignoring cached records. Default: false.",
						Default:     false,
					},
				},
				Required: []string{"url"},
			},
			ToolCallTemplate: CallTemplate{
				CallTemplateType: "http",
				URL:              cleanBase + "/scrape",
				HTTPMethod:       "POST",
				ContentType:      "application/json",
			},
		},
		{
			Name: "searqon_research",
			Description: "One-shot deep research tool for complex AI agent queries. " +
				"Searches across engines, selects top authoritative pages, scrapes them concurrently with anti-bot evasion, and synthesizes chunked sources.",
			Inputs: ToolInputSchema{
				Type: "object",
				Properties: map[string]ToolParamProperty{
					"query": {
						Type:        "string",
						Description: "The research query or topic.",
					},
					"max_sources": {
						Type:        "integer",
						Description: "Maximum number of source documents to fetch and chunk (default: 5, max: 10).",
						Default:     5,
					},
					"bypass_cache": {
						Type:        "boolean",
						Description: "Bypass cached search and scrape results. Default: false.",
						Default:     false,
					},
				},
				Required: []string{"query"},
			},
			ToolCallTemplate: CallTemplate{
				CallTemplateType: "http",
				URL:              cleanBase + "/pipeline",
				HTTPMethod:       "POST",
				ContentType:      "application/json",
			},
		},
		{
			Name: "searqon_map",
			Description: "Discover and map internal URLs across a target website or domain. " +
				"Acts as a sitemap crawler to reveal page hierarchy and links for autonomous agent exploration.",
			Inputs: ToolInputSchema{
				Type: "object",
				Properties: map[string]ToolParamProperty{
					"url": {
						Type:        "string",
						Description: "The website or domain root URL to map (e.g. 'https://docs.example.com').",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of internal links to discover (default: 50, max: 150).",
						Default:     50,
					},
				},
				Required: []string{"url"},
			},
			ToolCallTemplate: CallTemplate{
				CallTemplateType: "http",
				URL:              cleanBase + "/map",
				HTTPMethod:       "POST",
				ContentType:      "application/json",
			},
		},
		{
			Name: "searqon_extract",
			Description: "Extract targeted data from a webpage using CSS selectors or structured schema instructions. " +
				"Ideal for extracting structured tables, pricing, article metadata, or specific DOM elements.",
			Inputs: ToolInputSchema{
				Type: "object",
				Properties: map[string]ToolParamProperty{
					"url": {
						Type:        "string",
						Description: "The webpage URL to scrape and extract from.",
					},
					"schema": {
						Type:        "string",
						Description: "Extraction instructions, CSS selector, or expected JSON schema format.",
					},
					"bypass_cache": {
						Type:        "boolean",
						Description: "Force live network fetch, ignoring cached records (default: false).",
						Default:     false,
					},
				},
				Required: []string{"url", "schema"},
			},
			ToolCallTemplate: CallTemplate{
				CallTemplateType: "http",
				URL:              cleanBase + "/extract",
				HTTPMethod:       "POST",
				ContentType:      "application/json",
			},
		},
		{
			Name: "searqon_crawl",
			Description: "Recursively discover and scrape multiple linked pages under a target domain hierarchy up to a max depth and page limit.",
			Inputs: ToolInputSchema{
				Type: "object",
				Properties: map[string]ToolParamProperty{
					"url": {
						Type:        "string",
						Description: "Root URL to begin crawling from.",
					},
					"depth": {
						Type:        "integer",
						Description: "Maximum recursive link traversal depth (default: 2, max: 3).",
						Default:     2,
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum total pages to scrape (default: 20, max: 50).",
						Default:     20,
					},
				},
				Required: []string{"url"},
			},
			ToolCallTemplate: CallTemplate{
				CallTemplateType: "http",
				URL:              cleanBase + "/crawl",
				HTTPMethod:       "POST",
				ContentType:      "application/json",
			},
		},
		{
			Name: "searqon_search_stream",
			Description: "Real-time streaming web search over Server-Sent Events (SSE). Emits discovery, scraping progress, and final synthesized answers incrementally.",
			Inputs: ToolInputSchema{
				Type: "object",
				Properties: map[string]ToolParamProperty{
					"query": {
						Type:        "string",
						Description: "Search query string to execute and stream.",
					},
				},
				Required: []string{"query"},
			},
			ToolCallTemplate: CallTemplate{
				CallTemplateType: "sse",
				URL:              cleanBase + "/search/stream?query={query}",
			},
		},
	}

	return UtcpManual{
		UtcpVersion:   "1.0.1",
		ManualVersion: "1.0.0",
		Tools:         tools,
	}
}
