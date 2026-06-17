package utils

import (
	"log/slog"
	"os"
)

// InitLogger initializes a JSON structured logger for slog.
func InitLogger() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}
