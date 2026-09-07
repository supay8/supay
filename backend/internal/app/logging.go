package app

import (
	"log/slog"
	"os"
	"strings"

	"github.com/brandsrx/supay/internal/observability"
)

// SetupLogging configura slog como logger por defecto. LOG_LEVEL ajusta la
// verbosidad (debug|info|warn|error) y LOG_FORMAT elige json o texto.
func SetupLogging() {
	var level slog.Level
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if strings.EqualFold(strings.TrimSpace(os.Getenv("LOG_FORMAT")), "text") {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(observability.NewRedactingHandler(handler)))
}
