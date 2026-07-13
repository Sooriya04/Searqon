package utils

import (
	"log"
	"log/slog"
	"os"
	"src/logs"
)

// InitLogger initializes a JSON structured logger for slog and intercepts outputs for Minibase.
func InitLogger() {
	// 1. Create our multi-writer that forwards to Minibase REST API
	minibaseWriter := logs.NewMinibaseWriter(os.Stdout)

	// 2. Override default log package to write to our hook
	log.SetOutput(minibaseWriter)

	// 3. Override slog output to also go to our hook
	handler := slog.NewJSONHandler(minibaseWriter, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

