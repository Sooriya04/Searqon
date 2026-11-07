package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"src/models"
)

func TestMCPInitialize(t *testing.T) {
	server := NewServer()

	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`
	respBytes, err := server.HandleMessage([]byte(initReq))
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("Unmarshal response failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Expected success, got error: %v", resp.Error)
	}

	resultMap, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got: %T", resp.Result)
	}

	serverInfo, ok := resultMap["serverInfo"].(map[string]interface{})
	if !ok || serverInfo["name"] != "searqon" {
		t.Errorf("Unexpected serverInfo: %+v", serverInfo)
	}
}

func TestMCPToolsList(t *testing.T) {
	server := NewServer()

	listReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	respBytes, err := server.HandleMessage([]byte(listReq))
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("Unmarshal response failed: %v", err)
	}

	var toolsListResp struct {
		Result struct {
			Tools []ToolDefinition `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBytes, &toolsListResp); err != nil {
		t.Fatalf("Unmarshal tools failed: %v", err)
	}

	tools := toolsListResp.Result.Tools
	if len(tools) < 5 {
		t.Fatalf("Expected at least 5 tools, got %d", len(tools))
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	for _, expected := range []string{"searqon_search", "searqon_scrape", "searqon_research", "searqon_map", "searqon_extract"} {
		if !toolNames[expected] {
			t.Errorf("Expected tool %s to be registered", expected)
		}
	}
}

func TestMCPCallMapTool(t *testing.T) {
	server := NewServer()

	// Missing url should return error in CallToolResult
	callReq := `{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"searqon_map","arguments":{"url":""}}}`
	respBytes, err := server.HandleMessage([]byte(callReq))
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Expected resp.Error to be nil on tool error per JSON-RPC 2.0 / MCP spec, got: %+v", resp.Error)
	}

	resMap, ok := resp.Result.(map[string]interface{})
	if !ok || resMap["isError"] != true {
		t.Errorf("Expected isError=true for empty URL in searqon_map, got %+v", resp.Result)
	}
}

func TestMCPCallExtractTool(t *testing.T) {
	server := NewServer()

	// Missing url should return error in CallToolResult
	callReq := `{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"searqon_extract","arguments":{"url":""}}}`
	respBytes, err := server.HandleMessage([]byte(callReq))
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Expected resp.Error to be nil on tool error per JSON-RPC 2.0 / MCP spec, got: %+v", resp.Error)
	}

	resMap, ok := resp.Result.(map[string]interface{})
	if !ok || resMap["isError"] != true {
		t.Errorf("Expected isError=true for empty URL in searqon_extract, got %+v", resp.Result)
	}

	// Live extraction and cached extraction test
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>MCP Extract Test</title></head><body>
			<div class="content">
				<h2 class="title">MCP Extract Item</h2>
				<span class="badge">Active</span>
				<p>This is test content with sufficient length for extraction checks in Searqon MCP tool testing suite.</p>
			</div>
		</body></html>`)
	}))
	defer ts.Close()

	// First call: live scrape & extract
	callReqLive := fmt.Sprintf(`{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"searqon_extract","arguments":{"url":%q,"selectors":{"heading":".title","status":".badge"},"bypass_cache":true}}}`, ts.URL)
	respBytesLive, err := server.HandleMessage([]byte(callReqLive))
	if err != nil {
		t.Fatalf("Live HandleMessage failed: %v", err)
	}
	var respLive JSONRPCResponse
	if err := json.Unmarshal(respBytesLive, &respLive); err != nil {
		t.Fatalf("Live unmarshal failed: %v", err)
	}
	resMapLive := respLive.Result.(map[string]interface{})
	contentLive := resMapLive["content"].([]interface{})[0].(map[string]interface{})["text"].(string)

	var extractedLive map[string]interface{}
	if err := json.Unmarshal([]byte(contentLive), &extractedLive); err != nil {
		t.Fatalf("Failed to parse extracted JSON: %v", err)
	}
	selDataLive := extractedLive["selectors_data"].(map[string]interface{})
	if selDataLive["heading"] != "MCP Extract Item" || selDataLive["status"] != "Active" {
		t.Fatalf("Live selector extraction failed: %+v", selDataLive)
	}

	// Second call: cached extraction (bypass_cache=false)
	callReqCached := fmt.Sprintf(`{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"searqon_extract","arguments":{"url":%q,"selectors":{"heading":".title","status":".badge"},"bypass_cache":false}}}`, ts.URL)
	respBytesCached, err := server.HandleMessage([]byte(callReqCached))
	if err != nil {
		t.Fatalf("Cached HandleMessage failed: %v", err)
	}
	var respCached JSONRPCResponse
	if err := json.Unmarshal(respBytesCached, &respCached); err != nil {
		t.Fatalf("Cached unmarshal failed: %v", err)
	}
	resMapCached := respCached.Result.(map[string]interface{})
	contentCached := resMapCached["content"].([]interface{})[0].(map[string]interface{})["text"].(string)

	var extractedCached map[string]interface{}
	if err := json.Unmarshal([]byte(contentCached), &extractedCached); err != nil {
		t.Fatalf("Failed to parse cached extracted JSON: %v", err)
	}
	selDataCached := extractedCached["selectors_data"].(map[string]interface{})
	if selDataCached["heading"] != "MCP Extract Item" || selDataCached["status"] != "Active" {
		t.Fatalf("Cached selector extraction failed: %+v", selDataCached)
	}
}

func TestMCPToolStructuredErrorOutput(t *testing.T) {
	server := NewServer()

	// Calling searqon_scrape on an unreachable / blocked mock or invalid url
	callReq := `{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"searqon_scrape","arguments":{"url":"http://127.0.0.1:1/blocked"}}}`
	respBytes, err := server.HandleMessage([]byte(callReq))
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Expected resp.Error to be nil on tool error per JSON-RPC 2.0 / MCP spec, got: %+v", resp.Error)
	}

	resMap, ok := resp.Result.(map[string]interface{})
	if !ok || resMap["isError"] != true {
		t.Fatalf("Expected isError=true for scraping unreachable URL, got %+v", resp.Result)
	}

	contentSlice, ok := resMap["content"].([]interface{})
	if !ok || len(contentSlice) == 0 {
		t.Fatalf("Expected error content in tool result")
	}

	contentMap := contentSlice[0].(map[string]interface{})
	text := contentMap["text"].(string)

	var se models.SearqonError
	if err := json.Unmarshal([]byte(text), &se); err != nil {
		t.Fatalf("Expected tool error text to be valid SearqonError JSON, got: %s", text)
	}

	if se.ErrorType == "" {
		t.Errorf("Expected non-empty error type in structured error")
	}
}

func TestMCPUnknownMethod(t *testing.T) {
	server := NewServer()

	unknownReq := `{"jsonrpc":"2.0","id":3,"method":"invalid/method"}`
	respBytes, err := server.HandleMessage([]byte(unknownReq))
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("Unmarshal response failed: %v", err)
	}

	if resp.Error == nil || !strings.Contains(resp.Error.Message, "Method not found") {
		t.Errorf("Expected method not found error, got: %+v", resp.Error)
	}
}
