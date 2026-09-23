package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"src/config"
	"src/db"
	"src/handlers"
	"src/utils"
)

var testTmpDir string

func TestMain(m *testing.M) {
	var err error
	testTmpDir, err = os.MkdirTemp("", "searqon_tests_*")
	if err != nil {
		os.Exit(1)
	}

	cfg := config.DefaultConfig()
	cfg.Database.Enabled = true
	cfg.Database.Type = "sqlite"
	cfg.Database.SQLite.Path = filepath.Join(testTmpDir, "test_searqon.db")
	cfg.Logs.Enabled = true
	cfg.Logs.Storage = "sqlite"
	cfg.Logs.SQLitePath = filepath.Join(testTmpDir, "test_logs.db")
	cfg.Redis.Enabled = false
	cfg.SearXNG.Enabled = false

	utils.InitLogger()
	db.InitDB()

	code := m.Run()

	db.CloseDB()
	utils.CloseLogger()
	_ = os.RemoveAll(testTmpDir)

	os.Exit(code)
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handlers.HealthHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse health response: %v", err)
	}

	if resp["success"] != true {
		t.Fatalf("expected success true in health response")
	}
}

func TestOpenAPIHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rr := httptest.NewRecorder()

	handlers.OpenAPIHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", rr.Code)
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &spec); err != nil {
		t.Fatalf("failed to parse openapi.json: %v", err)
	}

	paths, ok := spec["paths"].(map[string]interface{})
	if !ok || paths["/logs"] == nil {
		t.Fatalf("expected /logs path defined in openapi.json")
	}
}

func TestLogsHandler(t *testing.T) {
	// Seed test logs
	utils.QueueLog(utils.LogRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     "INFO",
		Source:    "http",
		Message:   "HTTP Request test",
		Attributes: map[string]interface{}{
			"status": 200,
			"path":   "/health",
		},
	})

	utils.QueueLog(utils.LogRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     "ERROR",
		Source:    "scraper",
		Message:   "Simulated error in scraper pipeline",
		Attributes: map[string]interface{}{
			"error": "timeout",
		},
	})

	time.Sleep(350 * time.Millisecond) // Worker flush

	// 1. Query all logs
	req := httptest.NewRequest(http.MethodGet, "/logs?limit=10", nil)
	rr := httptest.NewRecorder()
	handlers.LogsHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool              `json:"success"`
		Total   int               `json:"total"`
		Logs    []utils.LogRecord `json:"logs"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse logs response: %v", err)
	}

	if !resp.Success || resp.Total < 2 {
		t.Fatalf("expected success and at least 2 logs, got %d", resp.Total)
	}

	// 2. Filter by ERROR level
	errReq := httptest.NewRequest(http.MethodGet, "/logs?level=ERROR", nil)
	errRR := httptest.NewRecorder()
	handlers.LogsHandler(errRR, errReq)

	var errResp struct {
		Logs []utils.LogRecord `json:"logs"`
	}
	_ = json.Unmarshal(errRR.Body.Bytes(), &errResp)
	if len(errResp.Logs) == 0 || errResp.Logs[0].Level != "ERROR" {
		t.Fatalf("expected ERROR logs, got: %+v", errResp.Logs)
	}

	// 3. Search query filter
	searchReq := httptest.NewRequest(http.MethodGet, "/logs?search=scraper", nil)
	searchRR := httptest.NewRecorder()
	handlers.LogsHandler(searchRR, searchReq)

	var searchResp struct {
		Logs []utils.LogRecord `json:"logs"`
	}
	_ = json.Unmarshal(searchRR.Body.Bytes(), &searchResp)
	if len(searchResp.Logs) == 0 {
		t.Fatalf("expected search match for 'scraper'")
	}
}

func TestScrapeHandlerDirect(t *testing.T) {
	// Spin up local mock HTTP target server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html><html><head><title>Test Extraction Page</title></head><body><article><h1>Headline Article</h1><p>This is substantive article content designed to test the fast HTTP scraping and markdown parsing pipeline in Go.</p></article></body></html>`))
	}))
	defer ts.Close()

	payload := map[string]interface{}{
		"url":          ts.URL,
		"bypass_cache": true,
	}
	bodyJSON, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/scrape", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handlers.ScrapeHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var res map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res["success"] != true {
		t.Fatalf("expected success true, got %v", res)
	}
	data, ok := res["data"].(map[string]interface{})
	if !ok || data["title"] != "Test Extraction Page" {
		t.Fatalf("expected extracted title 'Test Extraction Page', got %v", data["title"])
	}
}
