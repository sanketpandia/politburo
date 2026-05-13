package runtime

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// NotifyShutdown returns a context that is cancelled on SIGINT or SIGTERM.
func NotifyShutdown(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}
