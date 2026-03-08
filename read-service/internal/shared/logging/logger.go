package logging

import (
	"log/slog"
	"os"
)

// Setup initializes the global logger.
// In a real application, this would take configuration (level, format).
func Setup() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// ATG Update: Added shared logging package
