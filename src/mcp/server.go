package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"

	"src/crawler"
	"src/models"
	"src/scraper"
	"src/search"
)

// JSON-RPC 2.0 primitives according to the Model Context Protocol (MCP) specification.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ToolContent struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

type CallToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ProgressNotification represents standard MCP progress notification format.
type ProgressNotification struct {
	JSONRPC string                     `json:"jsonrpc"`
	Method  string                     `json:"method"`
	Params  ProgressNotificationParams `json:"params"`
}

type ProgressNotificationParams struct {
	ProgressToken interface{} `json:"progressToken"`
	Progress      float64     `json:"progress"`
	Total         float64     `json:"total,omitempty"`
	Message       string      `json:"message,omitempty"`
}

// ProgressReporter is a callback func for emitting progress updates.
type ProgressReporter func(progress, total float64, message string)

type progressReporterContextKey struct{}

// WithProgressReporter attaches a ProgressReporter to context.
func WithProgressReporter(ctx context.Context, reporter ProgressReporter) context.Context {
	return context.WithValue(ctx, progressReporterContextKey{}, reporter)
}

// GetProgressReporter extracts ProgressReporter from context if present.
func GetProgressReporter(ctx context.Context) ProgressReporter {
	if rep, ok := ctx.Value(progressReporterContextKey{}).(ProgressReporter); ok {
		return rep
	}
	return nil
}

type ToolParamProperty struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Default     any      `json:"default,omitempty"`
}

type ToolInputSchema struct {
	Type       string                       `json:"type"`
	Properties map[string]ToolParamProperty `json:"properties"`
	Required   []string                     `json:"required,omitempty"`
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema ToolInputSchema `json:"inputSchema"`
}

// Server encapsulates the MCP protocol dispatcher.
type Server struct {
	tools       []ToolDefinition
	writeNotify func([]byte)
	mu          sync.Mutex
}

// NewServer initializes a new Searqon MCP server loaded with agent-optimized tools.
func NewServer() *Server {
	s := &Server{}
	s.registerTools()
	return s
}

// SetNotificationWriter sets the function called whenever an asynchronous notification (e.g. $/progress) is sent.
func (s *Server) SetNotificationWriter(fn func([]byte)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeNotify = fn
}

func (s *Server) registerTools() {
	s.tools = []ToolDefinition{
		{
			Name: "searqon_search",
			Description: "Search the web using Searqon meta-search engine (powered by SearXNG, DuckDuckGo, Arxiv, and Wikipedia with automatic reciprocal rank fusion). " +
				"Returns clean, agent-ready search snippets, relevance scores, and source URLs. Supports optional live scraping of top results.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]ToolParamProperty{
					"query": {
						Type:        "string",
						Description: "The search query to execute across meta-search providers.",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of search results to return (default: 5, max: 20).",
					},
					"scrape": {
						Type:        "boolean",
						Description: "If true, concurrently scrapes and returns the clean Markdown text of the top results.",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name: "searqon_scrape",
			Description: "Ultra-sophisticated 4-tier anti-bot web scraper designed specifically for AI agents. " +
				"Bypasses Cloudflare / bot shields via tiered escalation (Fast HTTP -> Spoofed Fingerprint -> Headless Browser -> Proxy). " +
				"Extracts main readable content into clean Markdown (stripping navs, ads, scripts, and clutter) and truncates to avoid context overflow.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]ToolParamProperty{
					"url": {
						Type:        "string",
						Description: "The full target URL to scrape (e.g. 'https://example.com/article').",
					},
					"max_tokens": {
						Type:        "integer",
						Description: "Maximum token budget (approximate words * 1.3). Prevents blowing out LLM context windows. Default: 4000.",
					},
					"format": {
						Type:        "string",
						Description: "Output format: 'markdown' (recommended), 'text', or 'structured_json'. Default: 'markdown'.",
						Enum:        []string{"markdown", "text", "structured_json"},
					},
					"bypass_cache": {
						Type:        "boolean",
						Description: "Force a live fetch bypassing Searqon's SQLite/Redis cache. Default: false.",
					},
					"stealth_level": {
						Type:        "string",
						Description: "Enforce anti-bot evasion: 'auto', 'fast_http', 'spoofed_headers', 'headless', 'proxy'. Default: 'auto'.",
						Enum:        []string{"auto", "fast_http", "spoofed_headers", "headless", "proxy"},
					},
				},
				Required: []string{"url"},
			},
		},
		{
			Name: "searqon_research",
			Description: "One-shot deep research tool for complex AI agent queries. " +
				"Executes a multi-engine search via SearXNG, selects the top authoritative pages, scrapes them concurrently with anti-bot evasion, " +
				"and synthesizes an executive dossier with citations ready for immediate LLM consumption.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]ToolParamProperty{
					"query": {
						Type:        "string",
						Description: "The research topic or question.",
					},
					"max_sources": {
						Type:        "integer",
						Description: "Number of top sources to scrape and compile (default: 3, max: 8).",
					},
					"max_tokens_per_source": {
						Type:        "integer",
						Description: "Maximum tokens to include per source document (default: 2000).",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name: "searqon_map",
			Description: "Discover and map internal URLs across a target website or domain. " +
				"Acts as a sitemap crawler to reveal page hierarchy, link structures, and page paths for autonomous agent exploration.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]ToolParamProperty{
					"url": {
						Type:        "string",
						Description: "The website or domain root URL to map (e.g. 'https://docs.example.com').",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of internal links to discover (default: 50, max: 150).",
					},
				},
				Required: []string{"url"},
			},
		},
		{
			Name: "searqon_extract",
			Description: "Extract targeted data from a webpage using CSS selectors or structured schema instructions. " +
				"Ideal for extracting structured tables, pricing, article metadata, or specific DOM elements.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]ToolParamProperty{
					"url": {
						Type:        "string",
						Description: "The webpage URL to scrape and extract from.",
					},
					"selectors": {
						Type:        "object",
						Description: "Map of field names to CSS selectors (e.g. {\"title\": \"h1\", \"price\": \".price\"}).",
					},
					"schema": {
						Type:        "string",
						Description: "Optional prompt or JSON schema description for structured extraction.",
					},
					"bypass_cache": {
						Type:        "boolean",
						Description: "Force live network fetch, ignoring cached records (default: false).",
					},
				},
				Required: []string{"url"},
			},
		},
	}
}

// HandleMessage parses an incoming JSON-RPC payload and dispatches the MCP handler.
func (s *Server) HandleMessage(reqBytes []byte) ([]byte, error) {
	return s.HandleMessageContext(context.Background(), reqBytes)
}

func (s *Server) HandleMessageContext(ctx context.Context, reqBytes []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error:   &RPCError{Code: -32000, Message: "Context canceled or timeout: " + err.Error()},
		}
		return json.Marshal(resp)
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error:   &RPCError{Code: -32700, Message: "Parse error"},
		}
		return json.Marshal(resp)
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "initialize":
		resp.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{
					"listChanged": false,
				},
			},
			"serverInfo": map[string]interface{}{
				"name":    "searqon",
				"version": "1.2.0",
			},
		}

	case "notifications/initialized":
		return nil, nil

	case "tools/list":
		resp.Result = map[string]interface{}{
			"tools": s.tools,
		}

	case "tools/call":
		var callParams struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
			Meta      *struct {
				ProgressToken interface{} `json:"progressToken"`
			} `json:"_meta,omitempty"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			resp.Error = &RPCError{Code: -32602, Message: "Invalid tool call params"}
			break
		}

		toolCtx := ctx
		if callParams.Meta != nil && callParams.Meta.ProgressToken != nil {
			token := callParams.Meta.ProgressToken
			toolCtx = WithProgressReporter(toolCtx, func(progress, total float64, message string) {
				s.sendProgress(token, progress, total, message)
			})
		}

		toolResult, err := s.executeToolContext(toolCtx, callParams.Name, callParams.Arguments)
		if err != nil {
			se := models.ClassifyError(500, "", err)
			errJSON, _ := json.MarshalIndent(se, "", "  ")
			resp.Result = CallToolResult{
				Content: []ToolContent{
					{Type: "text", Text: string(errJSON)},
				},
				IsError: true,
			}
			resp.Error = nil
		} else {
			resp.Result = toolResult
			resp.Error = nil
		}

	case "ping":
		resp.Result = map[string]interface{}{}

	default:
		resp.Error = &RPCError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)}
	}

	return json.Marshal(resp)
}

func (s *Server) sendProgress(token interface{}, progress, total float64, message string) {
	s.mu.Lock()
	fn := s.writeNotify
	s.mu.Unlock()
	if fn == nil || token == nil {
		return
	}

	notif := ProgressNotification{
		JSONRPC: "2.0",
		Method:  "notifications/progress",
		Params: ProgressNotificationParams{
			ProgressToken: token,
			Progress:      progress,
			Total:         total,
			Message:       message,
		},
	}

	if b, err := json.Marshal(notif); err == nil {
		fn(b)
	}
}

func (s *Server) executeTool(name string, args map[string]interface{}) (CallToolResult, error) {
	return s.executeToolContext(context.Background(), name, args)
}

func (s *Server) executeToolContext(ctx context.Context, name string, args map[string]interface{}) (CallToolResult, error) {
	switch name {
	case "searqon_search":
		return s.handleSearchContext(ctx, args)
	case "searqon_scrape":
		return s.handleScrapeContext(ctx, args)
	case "searqon_research":
		return s.handleResearchContext(ctx, args)
	case "searqon_map":
		return s.handleMapContext(ctx, args)
	case "searqon_extract":
		return s.handleExtractContext(ctx, args)
	default:
		return CallToolResult{}, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) handleSearch(args map[string]interface{}) (CallToolResult, error) {
	return s.handleSearchContext(context.Background(), args)
}

func (s *Server) handleSearchContext(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	query, _ := args["query"].(string)
	if strings.TrimSpace(query) == "" {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: "Query parameter is required"}}}, nil
	}

	limit := 5
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
		if limit > 20 {
			limit = 20
		}
	}

	scrape := false
	if sc, ok := args["scrape"].(bool); ok {
		scrape = sc
	}

	reporter := GetProgressReporter(ctx)
	if reporter != nil {
		reporter(0, 3, fmt.Sprintf("Initiating search for query: %s", query))
	}

	var searchResp models.SearchResponse
	if reporter != nil {
		searchResp = search.RunSearchPipelineStream(ctx, query, limit, scrape, false, 0, false, "", func(evt search.SearchEvent) {
			switch evt.Type {
			case "discovery":
				reporter(1, 3, fmt.Sprintf("Discovered %d candidate results via %s", evt.Total, evt.Provider))
			case "scrape_progress":
				reporter(2, 3, "Scraping page contents")
			case "done":
				reporter(3, 3, "Search pipeline complete")
			}
		})
	} else {
		searchResp = search.RunSearchPipelineWithContext(ctx, query, limit, scrape, false, 0, false, "")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Searqon Search Results for: %q\n", query))
	sb.WriteString(fmt.Sprintf("**Provider:** %s | **Total Found:** %d | **Latency:** %dms\n\n",
		searchResp.Provider, searchResp.Total, searchResp.Duration))

	if len(searchResp.Results) == 0 {
		sb.WriteString("No results found for this query.")
	} else {
		for i, r := range searchResp.Results {
			sb.WriteString(fmt.Sprintf("### %d. [%s](%s)\n", i+1, r.Title, r.URL))
			sb.WriteString(fmt.Sprintf("- **Source:** %s | **Score:** %.2f | **Fetched At:** %s | **SHA-256:** `%s`\n", r.Source, r.Score, r.FetchedAt, r.ContentHash))
			if r.Snippet != "" {
				sb.WriteString(fmt.Sprintf("- **Snippet:** %s\n", strings.TrimSpace(r.Snippet)))
			}
			if scrape && r.Markdown != "" {
				preview := r.Markdown
				if len(preview) > 1500 {
					preview = truncateUTF8Safe(preview, 1500) + "\n...[truncated for search summary]..."
				}
				sb.WriteString(fmt.Sprintf("\n<scraped_content>\n%s\n</scraped_content>\n", preview))
			}
			sb.WriteString("\n")
		}
	}

	return CallToolResult{
		Content: []ToolContent{
			{Type: "text", Text: sb.String()},
		},
	}, nil
}

func (s *Server) handleScrape(args map[string]interface{}) (CallToolResult, error) {
	return s.handleScrapeContext(context.Background(), args)
}

func (s *Server) handleScrapeContext(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	targetURL, _ := args["url"].(string)
	if strings.TrimSpace(targetURL) == "" {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: "url parameter is required"}}}, nil
	}

	format := "markdown"
	if f, ok := args["format"].(string); ok && f != "" {
		format = f
	}

	bypassCache := false
	if bc, ok := args["bypass_cache"].(bool); ok {
		bypassCache = bc
	}

	maxTokens := 4000
	if mt, ok := args["max_tokens"].(float64); ok && mt > 0 {
		maxTokens = int(mt)
	} else if tl, ok := args["token_limit"].(float64); ok && tl > 0 {
		maxTokens = int(tl)
	}

	stealthLevel := "auto"
	if sl, ok := args["stealth_level"].(string); ok && sl != "" {
		stealthLevel = sl
	}

	opts := scraper.ScrapeOptions{
		Format:       format,
		BypassCache:  bypassCache,
		StealthLevel: stealthLevel,
		MaxTokens:    maxTokens,
	}

	result, _ := scraper.ScrapeSingleURLWithOptionsContext(ctx, targetURL, opts)

	if !result.Scraped && (result.Error != "" || result.StructuredError != nil) {
		se := result.StructuredError
		if se == nil {
			se = models.ClassifyError(result.StatusCode, result.Content, fmt.Errorf("%s", result.Error))
		}
		if se == nil {
			se = &models.SearqonError{
				ErrorType:         models.ErrUpstreamUnavailable,
				StatusCode:        502,
				Message:           result.Error,
				Retryable:         false,
				SuggestedFallback: models.FallbackLightpanda,
			}
		}
		errJSON, _ := json.MarshalIndent(se, "", "  ")
		return CallToolResult{
			IsError: true,
			Content: []ToolContent{
				{Type: "text", Text: string(errJSON)},
			},
		}, nil
	}

	// Format text representation
	var textOutput string
	if format == "structured_json" {
		jb, _ := json.MarshalIndent(result, "", "  ")
		textOutput = string(jb)
	} else {
		content := result.Markdown
		if content == "" {
			content = result.Content
		}

		// Apply Token Budget Truncation (1 token ≈ 4 chars or 0.75 words)
		charLimit := maxTokens * 4
		truncatedNote := ""
		if len(content) > charLimit {
			content = truncateUTF8Safe(content, charLimit)
			truncatedNote = fmt.Sprintf("\n\n> [!NOTE]\n> Content truncated at %d tokens (%d characters) to protect your context window. To fetch deeper, use targeted CSS selectors or increase max_tokens.", maxTokens, charLimit)
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n", result.Title))
		sb.WriteString(fmt.Sprintf("- **URL:** %s | **Fetched At:** %s | **SHA-256:** `%s`\n", result.URL, result.ScrapedAt, result.ContentHash))
		if result.Author != "" {
			sb.WriteString(fmt.Sprintf("- **Author:** %s\n", result.Author))
		}
		if result.PublishedAt != nil {
			sb.WriteString(fmt.Sprintf("- **Published:** %s\n", result.PublishedAt.Format("2006-01-02")))
		}
		sb.WriteString(fmt.Sprintf("- **Evasion Tier:** %s | **Render Engine:** %s | **Cached:** %v | **Latency:** %dms\n\n",
			result.EscalationTier, result.RenderMethod, result.Cached, result.Duration))
		sb.WriteString("---\n\n")
		sb.WriteString(content)
		sb.WriteString(truncatedNote)

		textOutput = sb.String()
	}

	return CallToolResult{
		Content: []ToolContent{
			{Type: "text", Text: textOutput},
		},
	}, nil
}

func (s *Server) handleResearch(args map[string]interface{}) (CallToolResult, error) {
	return s.handleResearchContext(context.Background(), args)
}

func (s *Server) handleResearchContext(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	query, _ := args["query"].(string)
	if strings.TrimSpace(query) == "" {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: "query parameter is required"}}}, nil
	}

	maxSources := 3
	if ms, ok := args["max_sources"].(float64); ok && ms > 0 {
		maxSources = int(ms)
		if maxSources > 8 {
			maxSources = 8
		}
	}

	maxTokensPerSource := 2000
	if mt, ok := args["max_tokens_per_source"].(float64); ok && mt > 0 {
		maxTokensPerSource = int(mt)
	}

	reporter := GetProgressReporter(ctx)
	if reporter != nil {
		reporter(0, float64(maxSources+2), fmt.Sprintf("Initiating deep research for: %s", query))
	}

	// 1. Search across meta-engines
	searchResp := search.RunSearchPipelineWithContext(ctx, query, maxSources+2, false, false, 0, false, "")
	if len(searchResp.Results) == 0 {
		return CallToolResult{
			Content: []ToolContent{
				{Type: "text", Text: fmt.Sprintf("No research material found for query: %q", query)},
			},
		}, nil
	}

	if reporter != nil {
		reporter(1, float64(maxSources+2), fmt.Sprintf("Discovered %d candidate sources via %s", len(searchResp.Results), searchResp.Provider))
	}

	// 2. Select top unique URLs to scrape
	var targets []models.SearchResult
	seenDomains := make(map[string]bool)
	for _, r := range searchResp.Results {
		d := r.URL
		if u, err := url.Parse(r.URL); err == nil && u.Hostname() != "" {
			d = u.Hostname()
		}
		if !seenDomains[d] {
			seenDomains[d] = true
			targets = append(targets, r)
		}
		if len(targets) >= maxSources {
			break
		}
	}

	// Fallback to non-unique domain if not enough
	if len(targets) < maxSources {
		for _, r := range searchResp.Results {
			if len(targets) >= maxSources {
				break
			}
			found := false
			for _, t := range targets {
				if t.URL == r.URL {
					found = true
					break
				}
			}
			if !found {
				targets = append(targets, r)
			}
		}
	}

	// 3. Concurrently scrape targets with anti-bot evasion
	type researchDoc struct {
		title     string
		url       string
		content   string
		source    string
		tier      string
		fetchedAt string
		sha256    string
	}

	docsChan := make(chan researchDoc, len(targets))
	var wg sync.WaitGroup
	var scrapedCount int64

	for _, target := range targets {
		wg.Add(1)
		go func(t models.SearchResult) {
			defer wg.Done()
			res, _ := scraper.ScrapeSingleURLWithOptionsContext(ctx, t.URL, scraper.ScrapeOptions{
				Format:       "markdown",
				StealthLevel: "auto",
			})

			body := res.Markdown
			if body == "" {
				body = res.Content
			}
			if body == "" {
				body = t.Snippet
			}

			// Token budget clamp
			charLimit := maxTokensPerSource * 4
			if len(body) > charLimit {
				body = truncateUTF8Safe(body, charLimit) + "\n...[truncated for token budget]..."
			}

			docsChan <- researchDoc{
				title:     t.Title,
				url:       t.URL,
				content:   body,
				source:    t.Source,
				tier:      res.EscalationTier,
				fetchedAt: res.ScrapedAt,
				sha256:    res.ContentHash,
			}

			if reporter != nil {
				sc := atomic.AddInt64(&scrapedCount, 1)
				reporter(1+float64(sc), float64(len(targets)+2), fmt.Sprintf("Scraped page %d of %d: %s", sc, len(targets), t.Title))
			}
		}(target)
	}

	wg.Wait()
	close(docsChan)

	if reporter != nil {
		reporter(float64(len(targets)+2), float64(len(targets)+2), "Research dossier synthesis complete")
	}

	// 4. Synthesize dossier
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Research Dossier: %s\n\n", query))
	sb.WriteString(fmt.Sprintf("Searqon Deep Research synthesized from %d sources across %s.\n\n",
		len(targets), searchResp.Provider))

	docIdx := 1
	for doc := range docsChan {
		sb.WriteString(fmt.Sprintf("## Source %d: [%s](%s)\n", docIdx, doc.title, doc.url))
		sb.WriteString(fmt.Sprintf("- **Search Source:** %s | **Evasion Tier:** %s | **Fetched At:** %s | **SHA-256:** `%s`\n\n", doc.source, doc.tier, doc.fetchedAt, doc.sha256))
		sb.WriteString(doc.content)
		sb.WriteString("\n\n---\n\n")
		docIdx++
	}

	return CallToolResult{
		Content: []ToolContent{
			{Type: "text", Text: sb.String()},
		},
	}, nil
}

func (s *Server) handleMap(args map[string]interface{}) (CallToolResult, error) {
	return s.handleMapContext(context.Background(), args)
}

func (s *Server) handleMapContext(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	targetURL, _ := args["url"].(string)
	if strings.TrimSpace(targetURL) == "" {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: "url parameter is required"}}}, nil
	}

	limit := 50
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
		if limit > 150 {
			limit = 150
		}
	}

	mapRes := crawler.MapSiteURLsWithContext(ctx, targetURL, limit)
	if mapRes.Error != "" {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("Failed to map site: %s", mapRes.Error)}}}, nil
	}

	resJSON, err := json.MarshalIndent(mapRes, "", "  ")
	if err != nil {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("Serialization error: %v", err)}}}, nil
	}

	return CallToolResult{
		Content: []ToolContent{
			{Type: "text", Text: string(resJSON)},
		},
	}, nil
}

func (s *Server) handleExtract(args map[string]interface{}) (CallToolResult, error) {
	return s.handleExtractContext(context.Background(), args)
}

func (s *Server) handleExtractContext(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	targetURL, _ := args["url"].(string)
	if strings.TrimSpace(targetURL) == "" {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: "url parameter is required"}}}, nil
	}

	bypassCache, _ := args["bypass_cache"].(bool)
	schemaPrompt, _ := args["schema"].(string)

	maxTokens := 4000
	if mt, ok := args["max_tokens"].(float64); ok && mt > 0 {
		maxTokens = int(mt)
	} else if tl, ok := args["token_limit"].(float64); ok && tl > 0 {
		maxTokens = int(tl)
	}

	var selectors map[string]string
	if rawSel, ok := args["selectors"].(map[string]interface{}); ok {
		selectors = make(map[string]string)
		for k, v := range rawSel {
			if strVal, ok := v.(string); ok {
				selectors[k] = strVal
			}
		}
	}

	opts := scraper.ScrapeOptions{
		Format:        "markdown",
		BypassCache:   bypassCache,
		ExtractSchema: schemaPrompt,
		StealthLevel:  "auto",
		MaxTokens:     maxTokens,
	}

	res, rawHTML := scraper.ScrapeSingleURLWithOptionsContext(ctx, targetURL, opts)
	if !res.Scraped && res.Error != "" {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("Scrape error: %s", res.Error)}}}, nil
	}

	extracted := make(map[string]interface{})
	extracted["title"] = res.Title
	extracted["url"] = targetURL
	extracted["markdown"] = res.Markdown
	extracted["token_estimate"] = res.TokenEstimate
	extracted["headings"] = res.Headings
	extracted["outline"] = res.Outline
	extracted["is_truncated"] = res.IsTruncated
	extracted["content_hash"] = res.ContentHash
	extracted["fetched_at"] = res.ScrapedAt

	if len(selectors) > 0 {
		htmlToParse := rawHTML
		if strings.TrimSpace(htmlToParse) == "" {
			if res.Content != "" {
				htmlToParse = res.Content
			} else if res.Markdown != "" {
				htmlToParse = res.Markdown
			}
		}

		if strings.TrimSpace(htmlToParse) != "" {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlToParse))
			if err == nil && doc != nil {
				selData := make(map[string]string)
				for k, sel := range selectors {
					selData[k] = strings.TrimSpace(doc.Find(sel).First().Text())
				}
				extracted["selectors_data"] = selData
			}
		}
	}

	if res.StructuredData != nil {
		extracted["structured_data"] = res.StructuredData
	}

	outJSON, err := json.MarshalIndent(extracted, "", "  ")
	if err != nil {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("Serialization error: %v", err)}}}, nil
	}

	return CallToolResult{
		Content: []ToolContent{
			{Type: "text", Text: string(outJSON)},
		},
	}, nil
}

// StartStdioLoop runs the MCP server reading JSON-RPC from Stdin and writing to Stdout.
// Crucial: All operational logs must go to Stderr to prevent corruption of the stdio JSON-RPC stream.
func (s *Server) StartStdioLoop() error {
	// Re-direct default logging to Stderr
	os.Stdout.Sync()
	reader := os.Stdin
	writer := os.Stdout
	var writeMu sync.Mutex

	s.SetNotificationWriter(func(b []byte) {
		writeMu.Lock()
		defer writeMu.Unlock()
		writer.Write(b)
		writer.Write([]byte("\n"))
	})

	dec := json.NewDecoder(reader)
	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("stdin decode error: %w", err)
		}

		respBytes, err := s.HandleMessage(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[MCP] Error handling message: %v\n", err)
			continue
		}

		if len(respBytes) > 0 {
			writeMu.Lock()
			writer.Write(respBytes)
			writer.Write([]byte("\n"))
			writeMu.Unlock()
		}
	}
}

// SSEHandler provides an HTTP handler for remote agents connecting over Server-Sent Events.
func (s *Server) SSEHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusOK)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", "*")

		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}
			resp, _ := s.HandleMessageContext(r.Context(), body)
			w.Header().Set("Content-Type", "application/json")
			w.Write(resp)
			return
		}

		// GET -> SSE stream endpoint
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Disable global HTTP server write deadlines for long-lived MCP SSE streams
		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Time{})

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", "/mcp")
		flusher.Flush()

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
		defer cancel()

		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fmt.Fprintf(w, ": keep-alive\n\n")
				flusher.Flush()
			}
		}
	}
}

// truncateUTF8Safe trims s to at most maxBytes, stepping backwards to avoid splitting multi-byte UTF-8 runes.
func truncateUTF8Safe(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	if maxBytes <= 0 {
		return ""
	}
	for maxBytes > 0 && !utf8.RuneStart(s[maxBytes]) {
		maxBytes--
	}
	return s[:maxBytes]
}
