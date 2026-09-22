package utils

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteLogger(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "searqon_log_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_logs.db")
	initSQLiteLogger(dbPath, 7)

	if !IsSQLiteLogReady() {
		t.Fatalf("expected SQLite log storage to be ready")
	}

	// Send test logs
	QueueLog(LogRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     "INFO",
		Source:    "test",
		Message:   "Test log message 1",
		Attributes: map[string]interface{}{
			"foo": "bar",
			"code": 200,
		},
	})

	QueueLog(LogRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     "ERROR",
		Source:    "scraper",
		Message:   "Simulated scraper failure",
		Attributes: map[string]interface{}{
			"url": "https://example.com/fail",
		},
	})

	// Allow worker flush
	time.Sleep(400 * time.Millisecond)

	// Query all logs
	records, total, err := QueryLogs(50, 0, "", "", "")
	if err != nil {
		t.Fatalf("failed to query logs: %v", err)
	}
	if total < 2 {
		t.Fatalf("expected at least 2 logs, got %d", total)
	}

	// Query filtered by ERROR
	errRecords, errTotal, err := QueryLogs(50, 0, "ERROR", "", "")
	if err != nil {
		t.Fatalf("failed to query error logs: %v", err)
	}
	if errTotal < 1 {
		t.Fatalf("expected at least 1 error log, got %d", errTotal)
	}
	if errRecords[0].Level != "ERROR" {
		t.Fatalf("expected ERROR level, got %s", errRecords[0].Level)
	}

	// Query search term
	searchRecords, _, err := QueryLogs(50, 0, "", "", "scraper")
	if err != nil {
		t.Fatalf("failed to query with search term: %v", err)
	}
	if len(searchRecords) == 0 {
		t.Fatalf("expected search match for 'scraper'")
	}

	_ = records
}
