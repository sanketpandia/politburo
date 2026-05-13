package messaging

import (
	"github.com/ThreeDotsLabs/watermill"
	"go.uber.org/zap"
)

// zapLogger adapts a *zap.Logger to watermill.LoggerAdapter.
type zapLogger struct {
	log *zap.Logger
}

// NewZapLogger returns a watermill.LoggerAdapter backed by the provided *zap.Logger.
func NewZapLogger(l *zap.Logger) watermill.LoggerAdapter {
	return &zapLogger{log: l}
}

func (z *zapLogger) Error(msg string, err error, fields watermill.LogFields) {
	args := fieldsToZap(fields)
	args = append(args, zap.Error(err))
	z.log.Error(msg, args...)
}

func (z *zapLogger) Info(msg string, fields watermill.LogFields) {
	z.log.Info(msg, fieldsToZap(fields)...)
}

func (z *zapLogger) Debug(msg string, fields watermill.LogFields) {
	z.log.Debug(msg, fieldsToZap(fields)...)
}

func (z *zapLogger) Trace(msg string, fields watermill.LogFields) {
	// Zap has no Trace level; map to Debug to avoid losing diagnostic output.
	z.log.Debug(msg, fieldsToZap(fields)...)
}

func (z *zapLogger) With(fields watermill.LogFields) watermill.LoggerAdapter {
	return &zapLogger{log: z.log.With(fieldsToZap(fields)...)}
}

// fieldsToZap converts watermill.LogFields (map[string]interface{}) to
// a flat slice of zap.Field values suitable for structured logging.
func fieldsToZap(fields watermill.LogFields) []zap.Field {
	out := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		out = append(out, zap.Any(k, v))
	}
	return out
}
