package zlogger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewNoEscapeJSONEncoder returns zap's JSON encoder, which does not perform browser HTML escaping.
//
// Deprecated: Use zapcore.NewJSONEncoder directly.
func NewNoEscapeJSONEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return zapcore.NewJSONEncoder(cfg)
}

// DisableHTMLEscaping returns the logger unchanged for v1 source compatibility.
// It cannot replace the encoder of an existing logger.
//
// Deprecated: Select an appropriate encoder when constructing the core.
func DisableHTMLEscaping(log *zap.Logger) *zap.Logger {
	return log
}
