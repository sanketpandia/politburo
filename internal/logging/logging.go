package logging

import (
	"log/slog"
	"os"
)

func Init(environment string) {
	level := slog.LevelInfo
	if environment == "local" || environment == "development" {
		level = slog.LevelDebug
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}
