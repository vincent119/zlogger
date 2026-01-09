package zlogger

import (
	"bytes"
	"os"
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

// 測試 sqlProcessingCore
func TestSqlProcessingCore_With(t *testing.T) {
	var buf bytes.Buffer
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}
	baseCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zap.DebugLevel,
	)

	sqlCore := &sqlProcessingCore{Core: baseCore}

	// 測試 With 方法
	fields := []zapcore.Field{
		zap.String("sql", `SELECT * FROM users WHERE name = \"John\"`),
	}
	newCore := sqlCore.With(fields)
	if newCore == nil {
		t.Error("expected non-nil core from With")
	}
}

func TestSqlProcessingCore_Check(t *testing.T) {
	var buf bytes.Buffer
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}
	baseCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zap.DebugLevel,
	)

	sqlCore := &sqlProcessingCore{Core: baseCore}

	// 測試 Check 方法
	entry := zapcore.Entry{
		Level:   zap.InfoLevel,
		Message: "test",
	}
	ce := &zapcore.CheckedEntry{}
	result := sqlCore.Check(entry, ce)
	if result == nil {
		t.Error("expected non-nil CheckedEntry")
	}
}

func TestSqlProcessingCore_Write(t *testing.T) {
	var buf bytes.Buffer
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}
	baseCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zap.DebugLevel,
	)

	sqlCore := &sqlProcessingCore{Core: baseCore}

	// 測試 Write 方法
	entry := zapcore.Entry{
		Level:   zap.InfoLevel,
		Message: "test with \\backslash",
	}
	fields := []zapcore.Field{
		zap.String("sql", `SELECT * FROM users WHERE name = \"John\"`),
	}

	err := sqlCore.Write(entry, fields)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
}

func TestSetLevel_WithLogger(t *testing.T) {
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
		zapGlobalLevel,
	)
	globalLogger = zap.New(core)
	globalConfig = DefaultConfig()

	// 測試 SetLevel 函式
	SetLevel("debug")
	if zapGlobalLevel.Level() != zap.DebugLevel {
		t.Errorf("expected DebugLevel, got %v", zapGlobalLevel.Level())
	}

	SetLevel("error")
	if zapGlobalLevel.Level() != zap.ErrorLevel {
		t.Errorf("expected ErrorLevel, got %v", zapGlobalLevel.Level())
	}
}

func TestLogAllLevels(t *testing.T) {
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
	globalConfig = DefaultConfig()

	// 測試所有日誌級別
	Debug("debug message", String("level", "debug"))
	Info("info message", String("level", "info"))
	Warn("warn message", String("level", "warn"))
	Error("error message", String("level", "error"))

	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Error("expected debug message in output")
	}
	if !strings.Contains(output, "info message") {
		t.Error("expected info message in output")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("expected warn message in output")
	}
	if !strings.Contains(output, "error message") {
		t.Error("expected error message in output")
	}
}

// 測試 Init 和 initLogger
func TestInitLogger_WithConsoleOutput(t *testing.T) {
	resetGlobalState()

	cfg := &Config{
		Level:        "debug",
		Format:       "console",
		Outputs:      []string{"console"},
		ColorEnabled: false,
	}

	// 直接呼叫 initLogger（不使用 Init 以避免 sync.Once）
	initLogger(cfg)

	if globalLogger == nil {
		t.Error("expected globalLogger to be initialized")
	}
	if globalConfig == nil {
		t.Error("expected globalConfig to be set")
	}
}

func TestInitLogger_WithJSONFormat(t *testing.T) {
	resetGlobalState()

	cfg := &Config{
		Level:        "info",
		Format:       "json",
		Outputs:      []string{"console"},
		ColorEnabled: false,
	}

	initLogger(cfg)

	if globalLogger == nil {
		t.Error("expected globalLogger to be initialized")
	}
}

func TestInitLogger_WithColorEnabled(t *testing.T) {
	resetGlobalState()

	cfg := &Config{
		Level:        "info",
		Format:       "console",
		Outputs:      []string{"console"},
		ColorEnabled: true,
	}

	initLogger(cfg)

	if globalLogger == nil {
		t.Error("expected globalLogger to be initialized")
	}
}

func TestInitLogger_WithFileOutput(t *testing.T) {
	resetGlobalState()

	tmpDir := t.TempDir()

	cfg := &Config{
		Level:   "info",
		Format:  "json",
		Outputs: []string{"file"},
		LogPath: tmpDir,
	}

	initLogger(cfg)

	if globalLogger == nil {
		t.Error("expected globalLogger to be initialized")
	}

	// 驗證日誌檔案建立
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read log dir: %v", err)
	}
	if len(files) == 0 {
		t.Error("expected log file to be created")
	}
}

func TestInitLogger_WithFileAndConsoleOutput(t *testing.T) {
	resetGlobalState()

	tmpDir := t.TempDir()

	cfg := &Config{
		Level:    "debug",
		Format:   "json",
		Outputs:  []string{"console", "file"},
		LogPath:  tmpDir,
		FileName: "test.log",
	}

	initLogger(cfg)

	if globalLogger == nil {
		t.Error("expected globalLogger to be initialized")
	}
}

func TestInitLogger_WithAllOptions(t *testing.T) {
	resetGlobalState()

	tmpDir := t.TempDir()

	cfg := &Config{
		Level:         "debug",
		Format:        "console",
		Outputs:       []string{"console", "file"},
		LogPath:       tmpDir,
		FileName:      "full-test.log",
		AddCaller:     true,
		AddStacktrace: true,
		Development:   true,
		ColorEnabled:  false,
	}

	initLogger(cfg)

	if globalLogger == nil {
		t.Error("expected globalLogger to be initialized")
	}
}

func TestInitLogger_EmptyOutputs(t *testing.T) {
	resetGlobalState()

	cfg := &Config{
		Level:   "info",
		Format:  "console",
		Outputs: []string{}, // 空輸出，應該使用預設控制台
	}

	initLogger(cfg)

	if globalLogger == nil {
		t.Error("expected globalLogger with default console output")
	}
}

func TestInitLogger_NilConfig(t *testing.T) {
	resetGlobalState()

	// 傳入 nil，應使用預設配置
	initLogger(nil)

	if globalLogger == nil {
		t.Error("expected globalLogger with default config")
	}
}

func TestBuildConsoleCore_JSONFormat(t *testing.T) {
	resetGlobalState()

	globalConfig = &Config{
		Format: "json",
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}

	core := buildConsoleCore(encoderConfig)
	if core == nil {
		t.Error("expected non-nil core")
	}
}

func TestBuildConsoleCore_ConsoleFormat(t *testing.T) {
	resetGlobalState()

	globalConfig = &Config{
		Format: "console",
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}

	core := buildConsoleCore(encoderConfig)
	if core == nil {
		t.Error("expected non-nil core")
	}
}

func TestBuildFileCore_JSONFormat(t *testing.T) {
	resetGlobalState()

	tmpDir := t.TempDir()

	globalConfig = &Config{
		Format:  "json",
		LogPath: tmpDir,
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}

	core := buildFileCore(encoderConfig)
	if core == nil {
		t.Error("expected non-nil core")
	}
}

func TestBuildFileCore_ConsoleFormat(t *testing.T) {
	resetGlobalState()

	tmpDir := t.TempDir()

	globalConfig = &Config{
		Format:  "console",
		LogPath: tmpDir,
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}

	core := buildFileCore(encoderConfig)
	if core == nil {
		t.Error("expected non-nil core")
	}
}

func TestBuildFileCore_WithFileName(t *testing.T) {
	resetGlobalState()

	tmpDir := t.TempDir()

	globalConfig = &Config{
		Format:   "json",
		LogPath:  tmpDir,
		FileName: "custom.log",
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}

	core := buildFileCore(encoderConfig)
	if core == nil {
		t.Error("expected non-nil core")
	}

	// 驗證自訂檔名
	files, _ := os.ReadDir(tmpDir)
	found := false
	for _, f := range files {
		if f.Name() == "custom.log" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected custom.log file to be created")
	}
}

func TestBuildFileCore_EmptyLogPath(t *testing.T) {
	resetGlobalState()

	// 使用臨時目錄，因為預設會使用 ./logs
	originalWd, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	globalConfig = &Config{
		Format:  "json",
		LogPath: "", // 空路徑，應使用預設 ./logs
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:    "ts",
		LevelKey:   "level",
		MessageKey: "msg",
	}

	core := buildFileCore(encoderConfig)
	if core == nil {
		t.Error("expected non-nil core with default log path")
	}

	// 清理
	os.RemoveAll("./logs")
}
