package utils

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"src/config"
)

// LogRecord represents a structured log event stored in SQLite.
type LogRecord struct {
	ID         int64                  `json:"id"`
	Timestamp  string                 `json:"timestamp"`
	Level      string                 `json:"level"`
	Source     string                 `json:"source"`
	Message    string                 `json:"message"`
	Attributes map[string]interface{} `json:"attributes"`
}

var (
	logDB       *sql.DB
	logQueue    chan LogRecord
	logWorkerWg sync.WaitGroup
	logCloseCh  chan struct{}
	logOnce     sync.Once
	logMu       sync.RWMutex
	sqliteReady bool
)

// SQLiteLogHandler is an slog.Handler that writes to stdout and queues to SQLite.
type SQLiteLogHandler struct {
	stdoutHandler slog.Handler
	minLevel      slog.Level
}

func (h *SQLiteLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.minLevel
}

func (h *SQLiteLogHandler) Handle(ctx context.Context, r slog.Record) error {
	// 1. Output to standard stdout handler
	if h.stdoutHandler != nil && h.stdoutHandler.Enabled(ctx, r.Level) {
		_ = h.stdoutHandler.Handle(ctx, r)
	}

	// 2. Queue into SQLite if available
	if !IsSQLiteLogReady() {
		return nil
	}

	attrs := make(map[string]interface{})
	source := "system"

	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "source" {
			source = a.Value.String()
		} else {
			attrs[a.Key] = a.Value.Any()
		}
		return true
	})

	levelStr := strings.ToUpper(r.Level.String())
	if strings.Contains(r.Message, "HTTP Request") {
		source = "http"
	}

	QueueLog(LogRecord{
		Timestamp:  r.Time.UTC().Format(time.RFC3339),
		Level:      levelStr,
		Source:     source,
		Message:    r.Message,
		Attributes: attrs,
	})

	return nil
}

func (h *SQLiteLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SQLiteLogHandler{
		stdoutHandler: h.stdoutHandler.WithAttrs(attrs),
		minLevel:      h.minLevel,
	}
}

func (h *SQLiteLogHandler) WithGroup(name string) slog.Handler {
	return &SQLiteLogHandler{
		stdoutHandler: h.stdoutHandler.WithGroup(name),
		minLevel:      h.minLevel,
	}
}

// logWriter intercepts standard Go log.Printf outputs and routes them to SQLite & stdout.
type logWriter struct {
	originalWriter io.Writer
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	n, err = w.originalWriter.Write(p)
	raw := strings.TrimSpace(string(p))
	if raw == "" || !IsSQLiteLogReady() {
		return n, err
	}

	// Guess source and level from standard log prefix
	source := "system"
	level := "INFO"

	if strings.Contains(raw, "[Escalation]") {
		source = "escalation"
	} else if strings.Contains(raw, "[Database]") {
		source = "database"
	} else if strings.Contains(raw, "[Proxy]") {
		source = "proxy"
	} else if strings.Contains(raw, "[Scraper]") {
		source = "scraper"
	} else if strings.Contains(raw, "[Search]") {
		source = "search"
	} else if strings.Contains(raw, "[Config]") {
		source = "config"
	}

	if strings.Contains(raw, "Error") || strings.Contains(raw, "failed") || strings.Contains(raw, "blocked") {
		level = "WARN"
	}

	QueueLog(LogRecord{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Level:      level,
		Source:     source,
		Message:    raw,
		Attributes: map[string]interface{}{},
	})

	return n, err
}

// InitLogger initializes the dual logging system (console + SQLite database).
func InitLogger() {
	cfg := config.Get()

	// Parse configured log level
	var minLevel slog.Level
	switch strings.ToLower(cfg.Logs.Level) {
	case "debug":
		minLevel = slog.LevelDebug
	case "warn", "warning":
		minLevel = slog.LevelWarn
	case "error":
		minLevel = slog.LevelError
	default:
		minLevel = slog.LevelInfo
	}

	// 1. Configure stdout handler
	stdoutHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: minLevel,
	})

	// 2. Initialize SQLite logs storage if enabled
	if cfg.Logs.Enabled && (cfg.Logs.Storage == "sqlite" || cfg.Logs.Storage == "both" || cfg.Logs.Storage == "") {
		initSQLiteLogger(cfg.Logs.SQLitePath, cfg.Logs.RetentionDays)
	}

	// 3. Register composite handler
	compositeHandler := &SQLiteLogHandler{
		stdoutHandler: stdoutHandler,
		minLevel:      minLevel,
	}

	slog.SetDefault(slog.New(compositeHandler))

	// 4. Redirect standard log package output to tee into SQLite
	log.SetOutput(&logWriter{originalWriter: os.Stdout})
}

func initSQLiteLogger(dbPath string, retentionDays int) {
	logOnce.Do(func() {
		if dbPath == "" {
			dbPath = "./data/logs.db"
		}

		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			log.Printf("[Logger] Failed to open SQLite logs database %s: %v", dbPath, err)
			return
		}

		// Enable WAL mode & performance optimizations
		_, _ = db.Exec("PRAGMA journal_mode=WAL;")
		_, _ = db.Exec("PRAGMA synchronous=NORMAL;")
		_, _ = db.Exec("PRAGMA busy_timeout=5000;")

		schema := `
		CREATE TABLE IF NOT EXISTS system_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			level TEXT NOT NULL,
			source TEXT NOT NULL,
			message TEXT NOT NULL,
			attributes TEXT DEFAULT '{}'
		);
		CREATE INDEX IF NOT EXISTS idx_system_logs_timestamp ON system_logs(timestamp);
		CREATE INDEX IF NOT EXISTS idx_system_logs_level ON system_logs(level);
		CREATE INDEX IF NOT EXISTS idx_system_logs_source ON system_logs(source);
		`
		if _, err := db.Exec(schema); err != nil {
			log.Printf("[Logger] Failed to initialize system_logs schema: %v", err)
			db.Close()
			return
		}

		logDB = db
		logQueue = make(chan LogRecord, 4096)
		logCloseCh = make(chan struct{})

		logMu.Lock()
		sqliteReady = true
		logMu.Unlock()

		logWorkerWg.Add(1)
		go logBatchWorker(retentionDays)

		log.Printf("[Logger] SQLite log storage initialized successfully at %s", dbPath)
	})
}

// QueueLog pushes a log event to the asynchronous batch channel.
func QueueLog(rec LogRecord) {
	if !IsSQLiteLogReady() || logQueue == nil {
		return
	}
	select {
	case logQueue <- rec:
	default:
		// Queue full under extreme pressure, drop rather than blocking
	}
}

// IsSQLiteLogReady returns true if SQLite logger is initialized.
func IsSQLiteLogReady() bool {
	logMu.RLock()
	defer logMu.RUnlock()
	return sqliteReady && logDB != nil
}

func logBatchWorker(retentionDays int) {
	defer logWorkerWg.Done()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	// Daily cleanup ticker for log retention
	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer cleanupTicker.Stop()

	batch := make([]LogRecord, 0, 100)

	flush := func() {
		if len(batch) == 0 || logDB == nil {
			return
		}

		tx, err := logDB.Begin()
		if err != nil {
			batch = batch[:0]
			return
		}

		stmt, err := tx.Prepare(`
			INSERT INTO system_logs (timestamp, level, source, message, attributes)
			VALUES (?, ?, ?, ?, ?)
		`)
		if err != nil {
			_ = tx.Rollback()
			batch = batch[:0]
			return
		}
		defer stmt.Close()

		for _, rec := range batch {
			attrsJSON, _ := json.Marshal(rec.Attributes)
			if len(attrsJSON) == 0 {
				attrsJSON = []byte("{}")
			}
			_, _ = stmt.Exec(rec.Timestamp, rec.Level, rec.Source, rec.Message, string(attrsJSON))
		}

		_ = tx.Commit()
		batch = batch[:0]
	}

	for {
		select {
		case rec := <-logQueue:
			batch = append(batch, rec)
			if len(batch) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-cleanupTicker.C:
			if retentionDays > 0 && logDB != nil {
				cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
				_, _ = logDB.Exec("DELETE FROM system_logs WHERE timestamp < ?", cutoff)
			}
		case <-logCloseCh:
			// Drain remaining logs
			for {
				select {
				case rec := <-logQueue:
					batch = append(batch, rec)
				default:
					flush()
					return
				}
			}
		}
	}
}

// QueryLogs retrieves system logs filtered by query parameters.
func QueryLogs(limit int, offset int, level string, source string, search string) ([]LogRecord, int, error) {
	if !IsSQLiteLogReady() {
		return nil, 0, fmt.Errorf("SQLite log storage is not active")
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	var conditions []string
	var args []interface{}

	if level != "" {
		conditions = append(conditions, "level = ?")
		args = append(args, strings.ToUpper(level))
	}
	if source != "" {
		conditions = append(conditions, "source = ?")
		args = append(args, strings.ToLower(source))
	}
	if search != "" {
		conditions = append(conditions, "(message LIKE ? OR attributes LIKE ?)")
		args = append(args, "%"+search+"%", "%"+search+"%")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total matching logs
	countQuery := "SELECT COUNT(*) FROM system_logs " + whereClause
	var total int
	err := logDB.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Retrieve paginated records
	selectQuery := fmt.Sprintf(`
		SELECT id, timestamp, level, source, message, attributes
		FROM system_logs
		%s
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	queryArgs := append(args, limit, offset)
	rows, err := logDB.Query(selectQuery, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []LogRecord
	for rows.Next() {
		var rec LogRecord
		var attrsJSON string
		if err := rows.Scan(&rec.ID, &rec.Timestamp, &rec.Level, &rec.Source, &rec.Message, &attrsJSON); err != nil {
			continue
		}
		if attrsJSON != "" {
			_ = json.Unmarshal([]byte(attrsJSON), &rec.Attributes)
		}
		if rec.Attributes == nil {
			rec.Attributes = make(map[string]interface{})
		}
		records = append(records, rec)
	}

	return records, total, nil
}

// CloseLogger flushes any queued logs and closes the SQLite database.
func CloseLogger() {
	logMu.Lock()
	if !sqliteReady {
		logMu.Unlock()
		return
	}
	sqliteReady = false
	logMu.Unlock()

	if logCloseCh != nil {
		close(logCloseCh)
		logWorkerWg.Wait()
	}

	if logDB != nil {
		_ = logDB.Close()
	}
}
