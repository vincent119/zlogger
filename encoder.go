package zlogger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewNoEscapeJSONEncoder creates a JSON encoder that does not escape HTML
func NewNoEscapeJSONEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return zapcore.NewJSONEncoder(cfg)
}

// DisableHTMLEscaping adds a hook to the logger (reserved for extension)
func DisableHTMLEscaping(log *zap.Logger) *zap.Logger {
	return log.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
		return nil
	}))
}
