package utils

import (
	"log"
	"log/slog"
	"os"
)

// InitLogger initializes a JSON structured logger for slog.
func InitLogger() {
	// 1. Override default log package to write to stdout
	log.SetOutput(os.Stdout)

	// 2. Override slog output to stdout
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

