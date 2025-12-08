package zlogger

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected zapcore.Level
	}{
		{"debug", zap.DebugLevel},
		{"DEBUG", zap.DebugLevel},
		{"Debug", zap.DebugLevel},
		{"info", zap.InfoLevel},
		{"INFO", zap.InfoLevel},
		{"warn", zap.WarnLevel},
		{"WARN", zap.WarnLevel},
		{"error", zap.ErrorLevel},
		{"ERROR", zap.ErrorLevel},
		{"fatal", zap.FatalLevel},
		{"FATAL", zap.FatalLevel},
		{"unknown", zap.InfoLevel}, // 預設為 info
		{"", zap.InfoLevel},        // 空字串預設為 info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestProcessSQLString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`SELECT * FROM users`, `SELECT * FROM users`},
		{`SELECT * FROM users WHERE name = \"John\"`, `SELECT * FROM users WHERE name = "John"`},
		{`SELECT * FROM users WHERE name = \'John\'`, `SELECT * FROM users WHERE name = 'John'`},
		{`path\\to\\file`, `path\to\file`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := processSQLString(tt.input)
			if result != tt.expected {
				t.Errorf("processSQLString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// 重置全局狀態以便測試
func resetGlobalState() {
	globalLogger = nil
	globalConfig = nil
	once = sync.Once{}
}

func TestInit_WithNilConfig(t *testing.T) {
	resetGlobalState()

	// 使用自定義 buffer 捕獲輸出
	var buf bytes.Buffer
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:      "ts",
		LevelKey:     "level",
		MessageKey:   "msg",
		EncodeTime:   zapcore.ISO8601TimeEncoder,
		EncodeLevel:  zapcore.CapitalLevelEncoder,
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zap.DebugLevel,
	)
	globalLogger = zap.New(core)
	globalConfig = DefaultConfig()

	// 測試日誌輸出
	Info("test message", String("key", "value"))

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got: %s", output)
	}
	if !strings.Contains(output, `"key":"value"`) {
		t.Errorf("expected output to contain key:value, got: %s", output)
	}
}

func TestLogFunctions_NilLogger(t *testing.T) {
	resetGlobalState()

	// 當 globalLogger 為 nil 時，不應該 panic
	Debug("debug message")
	Info("info message")
	Warn("warn message")
	Error("error message")
	// 不測試 Fatal，因為它會調用 os.Exit
}

func TestSetLevel(t *testing.T) {
	resetGlobalState()

	// 設置初始級別
	zapGlobalLevel.SetLevel(zap.InfoLevel)

	// 測試設置為 debug
	zapGlobalLevel.SetLevel(parseLevel("debug"))
	if zapGlobalLevel.Level() != zap.DebugLevel {
		t.Errorf("expected DebugLevel, got %v", zapGlobalLevel.Level())
	}

	// 測試設置為 error
	zapGlobalLevel.SetLevel(parseLevel("error"))
	if zapGlobalLevel.Level() != zap.ErrorLevel {
		t.Errorf("expected ErrorLevel, got %v", zapGlobalLevel.Level())
	}
}

func TestGetLogger_NilLogger(t *testing.T) {
	resetGlobalState()

	logger := GetLogger()
	if logger != nil {
		t.Error("expected nil logger before init")
	}
}

func TestSync_NilLogger(t *testing.T) {
	resetGlobalState()

	err := Sync()
	if err != nil {
		t.Errorf("Sync() with nil logger should return nil, got %v", err)
	}
}

func TestLogWithFields(t *testing.T) {
	resetGlobalState()

	var buf bytes.Buffer
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:      "ts",
		LevelKey:     "level",
		MessageKey:   "msg",
		EncodeTime:   zapcore.ISO8601TimeEncoder,
		EncodeLevel:  zapcore.CapitalLevelEncoder,
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zap.DebugLevel,
	)
	globalLogger = zap.New(core)
	globalConfig = DefaultConfig()

	// 測試各種 Field 類型
	Info("test",
		String("str", "hello"),
		Int("int", 42),
		Bool("bool", true),
	)

	output := buf.String()
	if !strings.Contains(output, `"str":"hello"`) {
		t.Errorf("expected string field, got: %s", output)
	}
	if !strings.Contains(output, `"int":42`) {
		t.Errorf("expected int field, got: %s", output)
	}
	if !strings.Contains(output, `"bool":true`) {
		t.Errorf("expected bool field, got: %s", output)
	}
}

