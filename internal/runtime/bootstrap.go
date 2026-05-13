package runtime

import (
	"fmt"
	"log"
	"os"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/app"
)

// Bootstrap loads config, initializes logging, and wires the application.
// The returned cleanup func must be deferred by the caller.
func Bootstrap() (*app.App, app.Config, func(), error) {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg := app.LoadConfig()

	if err := logging.Init(cfg.AppEnv); err != nil {
		return nil, app.Config{}, nil, fmt.Errorf("logging init: %w", err)
	}

	logging.Info("Starting up", "environment", cfg.AppEnv, "debug", cfg.Debug)

	application, err := app.New(cfg)
	if err != nil {
		logging.Error("Failed to initialize application", "error", err)
		logging.Close()
		return nil, app.Config{}, nil, fmt.Errorf("app init: %w", err)
	}

	return application, cfg, func() { _ = logging.Close() }, nil
}
