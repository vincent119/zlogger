package zlogger

import (
	"bytes"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestSugar_NilLogger(t *testing.T) {
	resetGlobalState()

	sugar := Sugar()
	if sugar != nil {
		t.Error("expected nil SugaredLogger when globalLogger is nil")
	}
}

func TestSugar_WithLogger(t *testing.T) {
	resetGlobalState()

	// 建立測試用 logger
	var buf bytes.Buffer
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zap.DebugLevel,
	)
	globalLogger = zap.New(core)

	sugar := Sugar()
	if sugar == nil {
		t.Error("expected non-nil SugaredLogger")
	}
}

func TestNamed_NilLogger(t *testing.T) {
	resetGlobalState()

	named := Named("test")
	if named != nil {
		t.Error("expected nil Logger when globalLogger is nil")
	}
}

func TestNamed_WithLogger(t *testing.T) {
	resetGlobalState()

	var buf bytes.Buffer
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		NameKey:     "logger",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zap.DebugLevel,
	)
	globalLogger = zap.New(core)

	named := Named("test")
	if named == nil {
		t.Error("expected non-nil named Logger")
	}
}

func TestWith_NilLogger(t *testing.T) {
	resetGlobalState()

	withLogger := With(String("key", "value"))
	if withLogger != nil {
		t.Error("expected nil Logger when globalLogger is nil")
	}
}

func TestWith_WithLogger(t *testing.T) {
	resetGlobalState()

	var buf bytes.Buffer
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zap.DebugLevel,
	)
	globalLogger = zap.New(core)

	withLogger := With(String("key", "value"))
	if withLogger == nil {
		t.Error("expected non-nil Logger with fields")
	}

	// 使用 withLogger 記錄日誌
	withLogger.Info("test message")

	output := buf.String()
	if output == "" {
		t.Error("expected log output")
	}
}

func TestWithOptions_NilLogger(t *testing.T) {
	resetGlobalState()

	optLogger := WithOptions(zap.AddCaller())
	if optLogger != nil {
		t.Error("expected nil Logger when globalLogger is nil")
	}
}

func TestWithOptions_WithLogger(t *testing.T) {
	resetGlobalState()

	var buf bytes.Buffer
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		CallerKey:   "caller",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zap.DebugLevel,
	)
	globalLogger = zap.New(core)

	optLogger := WithOptions(zap.AddCaller())
	if optLogger == nil {
		t.Error("expected non-nil Logger with options")
	}
}

func TestNewNop(t *testing.T) {
	logger := NewNop()
	if logger == nil {
		t.Error("expected non-nil nop logger")
	}

	// nop logger 不應該輸出任何內容
	logger.Info("this should not appear anywhere")
}

func TestNewDevelopment(t *testing.T) {
	logger, err := NewDevelopment()
	if err != nil {
		t.Errorf("unexpected error creating development logger: %v", err)
	}
	if logger == nil {
		t.Error("expected non-nil development logger")
	}
}

func TestNewProduction(t *testing.T) {
	logger, err := NewProduction()
	if err != nil {
		t.Errorf("unexpected error creating production logger: %v", err)
	}
	if logger == nil {
		t.Error("expected non-nil production logger")
	}
}

func TestGetLogger_WithLogger(t *testing.T) {
	resetGlobalState()

	var buf bytes.Buffer
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zap.DebugLevel,
	)
	globalLogger = zap.New(core)

	logger := GetLogger()
	if logger == nil {
		t.Error("expected non-nil logger from GetLogger")
	}
	if logger != globalLogger {
		t.Error("expected GetLogger to return globalLogger")
	}
}

func TestSync_WithLogger(t *testing.T) {
	resetGlobalState()

	var buf bytes.Buffer
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zap.DebugLevel,
	)
	globalLogger = zap.New(core)

	err := Sync()
	// Sync 可能會返回錯誤（stdout/stderr 的 sync 問題），但不應該 panic
	_ = err
}

// 重置全局狀態以便測試（如果 core_test.go 未導出）
func init() {
	// 確保 once 可以被重置
	_ = sync.Once{}
}
