package zlogger

import (
	"io"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func BenchmarkLoggerInfoDisabled(b *testing.B) {
	logger := newBenchmarkLogger(zapcore.ErrorLevel)
	fields := benchmarkFields()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		logger.Info("request completed", fields...)
	}
}

func BenchmarkLoggerInfoFields(b *testing.B) {
	logger := newBenchmarkLogger(zapcore.InfoLevel)
	fields := benchmarkFields()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		logger.Info("request completed", fields...)
	}
}

func newBenchmarkLogger(level zapcore.Level) *zap.Logger {
	cfg := DefaultConfig()
	cfg.Format = "json"
	cfg.ColorEnabled = false
	encoder := newEncoder(cfg.Format, buildEncoderConfig(cfg))
	sink := zapcore.Lock(zapcore.AddSync(io.Discard))
	return zap.New(zapcore.NewCore(encoder, sink, level))
}

func benchmarkFields() []zap.Field {
	return []zap.Field{
		zap.String("request_id", "req-1234567890"),
		zap.String("method", "GET"),
		zap.String("path", "/v1/orders/123"),
		zap.Int("status", 200),
		zap.Duration("duration", 25*time.Millisecond),
	}
}
