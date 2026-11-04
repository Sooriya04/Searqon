package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestContextCancellationAbortsScrape(t *testing.T) {
	server := NewServer()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	callReq := `{"jsonrpc":"2.0","id":99,"method":"tools/call","params":{"name":"searqon_scrape","arguments":{"url":"https://example.com"}}}`
	
	respBytes, err := server.HandleMessageContext(ctx, []byte(callReq))
	if err != nil {
		t.Fatalf("HandleMessageContext failed: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	resStr := fmt.Sprintf("%v", resp.Result)
	if resp.Error == nil && !strings.Contains(resStr, "canceled") && !strings.Contains(resStr, "context") {
		t.Errorf("Expected context cancellation error, got: %+v", resp)
	}
}
