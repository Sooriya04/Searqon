package mcp

import (
	"encoding/json"
	"strings"
	"testing"
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

	resMap, ok := resp.Result.(map[string]interface{})
	if !ok || resMap["isError"] != true {
		t.Errorf("Expected isError=true for empty URL in searqon_extract, got %+v", resp.Result)
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
