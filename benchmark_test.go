package zlogger

import (
	"context"
	"io"
	"strconv"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var benchmarkContextResult context.Context

type benchmarkWriteSyncCloser struct{}

func (benchmarkWriteSyncCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (benchmarkWriteSyncCloser) Sync() error {
	return nil
}

func (benchmarkWriteSyncCloser) Close() error {
	return nil
}

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

func BenchmarkLoggerInfoContext(b *testing.B) {
	logger := newBenchmarkLogger(zapcore.InfoLevel)
	fields := benchmarkFields()
	ctx := WithContext(context.Background(), fields...)

	originalLogger := globalLogger
	globalLogger = logger
	b.Cleanup(func() {
		globalLogger = originalLogger
	})

	b.Run("direct", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			logger.Info("request completed", fields...)
		}
	})

	b.Run("context_only", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			InfoContext(ctx, "request completed")
		}
	})

	b.Run("context_with_fields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			InfoContext(ctx, "request completed", fields...)
		}
	})
}

func BenchmarkLoggerWithContext(b *testing.B) {
	for _, fieldCount := range []int{1, 5, 20} {
		fields := benchmarkContextFields(fieldCount)
		base := context.Background()

		b.Run(strconv.Itoa(fieldCount)+"_fields/batch", func(b *testing.B) {
			var result context.Context
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				result = WithContext(base, fields...)
			}
			benchmarkContextResult = result
		})

		b.Run(strconv.Itoa(fieldCount)+"_fields/incremental", func(b *testing.B) {
			var result context.Context
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				ctx := base
				for _, field := range fields {
					ctx = WithContext(ctx, field)
				}
				result = ctx
			}
			benchmarkContextResult = result
		})
	}
}

func BenchmarkLoggerSplitOutputWrite(b *testing.B) {
	payload := []byte("{\"level\":\"info\",\"message\":\"request completed\",\"request_id\":\"req-1234567890\"}\n")

	b.Run("serial", func(b *testing.B) {
		output := newBenchmarkSplitOutput()
		b.ReportAllocs()
		b.SetBytes(int64(len(payload)))
		b.ResetTimer()
		for b.Loop() {
			written, err := output.Write(zapcore.InfoLevel, payload)
			if err != nil || written != len(payload) {
				b.Fatalf("SplitOutput.Write 寫入結果 = (%d, %v)，預期 (%d, nil)", written, err, len(payload))
			}
		}
	})

	b.Run("parallel_same_level", func(b *testing.B) {
		output := newBenchmarkSplitOutput()
		b.ReportAllocs()
		b.SetBytes(int64(len(payload)))
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				written, err := output.Write(zapcore.InfoLevel, payload)
				if err != nil || written != len(payload) {
					b.Errorf("SplitOutput.Write 寫入結果 = (%d, %v)，預期 (%d, nil)", written, err, len(payload))
					return
				}
			}
		})
	})

	b.Run("parallel_mixed_level", func(b *testing.B) {
		output := newBenchmarkSplitOutput()
		levels := [...]zapcore.Level{
			zapcore.InfoLevel,
			zapcore.WarnLevel,
			zapcore.ErrorLevel,
		}

		b.ReportAllocs()
		b.SetBytes(int64(len(payload)))
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			index := 0
			for pb.Next() {
				written, err := output.Write(levels[index%len(levels)], payload)
				if err != nil || written != len(payload) {
					b.Errorf("SplitOutput.Write 寫入結果 = (%d, %v)，預期 (%d, nil)", written, err, len(payload))
					return
				}
				index++
			}
		})
	})
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

func benchmarkContextFields(count int) []zap.Field {
	fields := make([]zap.Field, count)
	for index := range fields {
		fields[index] = zap.String("field_"+strconv.Itoa(index), "value")
	}
	return fields
}

func newBenchmarkSplitOutput() *SplitOutput {
	return &SplitOutput{
		infoOut:  benchmarkWriteSyncCloser{},
		warnOut:  benchmarkWriteSyncCloser{},
		errorOut: benchmarkWriteSyncCloser{},
	}
}
