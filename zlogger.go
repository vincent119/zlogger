// Package zlogger provides a structured logging library based on zap
//
// For detailed usage, please refer to README.md
package zlogger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Re-export common zap types for convenience
type (
	// Logger is an alias for zap.Logger
	Logger = zap.Logger
	// SugaredLogger is an alias for zap.SugaredLogger
	SugaredLogger = zap.SugaredLogger
	// Level is an alias for zapcore.Level
	Level = zapcore.Level
	// EncoderConfig is an alias for zapcore.EncoderConfig
	EncoderConfig = zapcore.EncoderConfig
	// Core is an alias for zapcore.Core
	Core = zapcore.Core
)

// Common log level constants
const (
	DebugLevel  = zapcore.DebugLevel
	InfoLevel   = zapcore.InfoLevel
	WarnLevel   = zapcore.WarnLevel
	ErrorLevel  = zapcore.ErrorLevel
	DPanicLevel = zapcore.DPanicLevel
	PanicLevel  = zapcore.PanicLevel
	FatalLevel  = zapcore.FatalLevel
)

// Sugar returns a SugaredLogger with a more convenient API
func Sugar() *SugaredLogger {
	if globalLogger != nil {
		return globalLogger.Sugar()
	}
	return nil
}

// Named creates a named sub-logger
func Named(name string) *Logger {
	if globalLogger != nil {
		return globalLogger.Named(name)
	}
	return nil
}

// With creates a sub-logger with preset fields
func With(fields ...Field) *Logger {
	if globalLogger != nil {
		return globalLogger.With(fields...)
	}
	return nil
}

// WithOptions creates a logger with additional options
func WithOptions(opts ...zap.Option) *Logger {
	if globalLogger != nil {
		return globalLogger.WithOptions(opts...)
	}
	return nil
}

// NewNop creates a no-op logger (useful for testing)
func NewNop() *Logger {
	return zap.NewNop()
}

// NewDevelopment creates a development mode logger
func NewDevelopment() (*Logger, error) {
	return zap.NewDevelopment()
}

// NewProduction creates a production mode logger
func NewProduction() (*Logger, error) {
	return zap.NewProduction()
}
