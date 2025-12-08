// Package zlogger 提供基於 zap 的結構化日誌庫
//
// 詳細使用方式請參考 README.md
package zlogger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 重新導出 zap 的常用類型，方便使用者直接使用
type (
	// Logger 是 zap.Logger 的別名
	Logger = zap.Logger
	// SugaredLogger 是 zap.SugaredLogger 的別名
	SugaredLogger = zap.SugaredLogger
	// Level 是 zapcore.Level 的別名
	Level = zapcore.Level
	// EncoderConfig 是 zapcore.EncoderConfig 的別名
	EncoderConfig = zapcore.EncoderConfig
	// Core 是 zapcore.Core 的別名
	Core = zapcore.Core
)

// 常用的日誌級別常量
const (
	DebugLevel  = zapcore.DebugLevel
	InfoLevel   = zapcore.InfoLevel
	WarnLevel   = zapcore.WarnLevel
	ErrorLevel  = zapcore.ErrorLevel
	DPanicLevel = zapcore.DPanicLevel
	PanicLevel  = zapcore.PanicLevel
	FatalLevel  = zapcore.FatalLevel
)

// Sugar 返回 SugaredLogger，提供更方便的 API
func Sugar() *SugaredLogger {
	if globalLogger != nil {
		return globalLogger.Sugar()
	}
	return nil
}

// Named 創建具名的子 logger
func Named(name string) *Logger {
	if globalLogger != nil {
		return globalLogger.Named(name)
	}
	return nil
}

// With 創建帶有預設字段的子 logger
func With(fields ...Field) *Logger {
	if globalLogger != nil {
		return globalLogger.With(fields...)
	}
	return nil
}

// WithOptions 創建帶有額外選項的 logger
func WithOptions(opts ...zap.Option) *Logger {
	if globalLogger != nil {
		return globalLogger.WithOptions(opts...)
	}
	return nil
}

// NewNop 創建一個不輸出任何內容的 logger（用於測試）
func NewNop() *Logger {
	return zap.NewNop()
}

// NewDevelopment 創建開發模式的 logger
func NewDevelopment() (*Logger, error) {
	return zap.NewDevelopment()
}

// NewProduction 創建生產模式的 logger
func NewProduction() (*Logger, error) {
	return zap.NewProduction()
}

