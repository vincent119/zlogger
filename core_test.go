package zlogger

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
		{"unknown", zap.InfoLevel}, // default to info
		{"", zap.InfoLevel},        // empty string defaults to info
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

func TestBuildEncoderConfigColorContract(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		enabled   bool
		wantColor bool
	}{
		{name: "console 啟用顏色", format: "console", enabled: true, wantColor: true},
		{name: "console 停用顏色", format: "console", enabled: false, wantColor: false},
		{name: "JSON 啟用顏色", format: "json", enabled: true, wantColor: false},
		{name: "JSON 停用顏色", format: "json", enabled: false, wantColor: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Format: tt.format, ColorEnabled: tt.enabled}
			entry := zapcore.Entry{Level: zap.InfoLevel, Message: "測試訊息"}
			encoded, err := newEncoder(tt.format, buildEncoderConfig(cfg)).EncodeEntry(entry, nil)
			if err != nil {
				t.Fatalf("編碼日誌失敗：%v", err)
			}
			t.Cleanup(encoded.Free)

			level := encoded.String()
			if tt.format == "json" {
				var output struct {
					Level string `json:"level"`
				}
				if err := json.Unmarshal(encoded.Bytes(), &output); err != nil {
					t.Fatalf("解析 JSON 日誌失敗：%v", err)
				}
				level = output.Level
			}

			if gotColor := strings.Contains(level, "\x1b["); gotColor != tt.wantColor {
				t.Errorf("format=%s color_enabled=%t 的 level=%q，顏色狀態=%t，預期=%t",
					tt.format, tt.enabled, level, gotColor, tt.wantColor)
			}
		})
	}
}

// resetGlobalState 隔離會修改全域 logger 的測試，並回報資源清理錯誤。
func resetGlobalState(t testing.TB) {
	t.Helper()

	configureMu.Lock()
	cleanup := globalCleanup
	configureMu.Unlock()
	if cleanup != nil {
		if err := cleanup(); err != nil {
			t.Errorf("清理全域 logger 失敗：%v", err)
		}
	}

	configureMu.Lock()
	globalLogger = nil
	globalConfig = nil
	zapGlobalLevel = zap.NewAtomicLevel()
	configured = false
	globalCleanup = nil
	configureMu.Unlock()
	zap.ReplaceGlobals(zap.NewNop())
}

func registerGlobalCleanup(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		resetGlobalState(t)
	})
}

func registerFileCleanup(t *testing.T, file *os.File) {
	t.Helper()
	if file == nil {
		t.Fatal("測試日誌檔案不可為 nil")
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("關閉測試日誌檔案失敗：%v", err)
		}
	})
}

func TestInit_WithNilConfig(t *testing.T) {
	resetGlobalState(t)

	// Use custom buffer to capture output
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

	// Test log output
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
	resetGlobalState(t)

	// Should not panic when globalLogger is nil
	Debug("debug message")
	Info("info message")
	Warn("warn message")
	Error("error message")
	// Do not test Fatal as it calls os.Exit
}

func TestSetLevel(t *testing.T) {
	resetGlobalState(t)

	// Set initial level
	zapGlobalLevel.SetLevel(zap.InfoLevel)

	// Test setting to debug
	zapGlobalLevel.SetLevel(parseLevel("debug"))
	if zapGlobalLevel.Level() != zap.DebugLevel {
		t.Errorf("expected DebugLevel, got %v", zapGlobalLevel.Level())
	}

	// Test setting to error
	zapGlobalLevel.SetLevel(parseLevel("error"))
	if zapGlobalLevel.Level() != zap.ErrorLevel {
		t.Errorf("expected ErrorLevel, got %v", zapGlobalLevel.Level())
	}
}

func TestGetLogger_NilLogger(t *testing.T) {
	resetGlobalState(t)

	logger := GetLogger()
	if logger != nil {
		t.Error("expected nil logger before init")
	}
}

func TestSync_NilLogger(t *testing.T) {
	resetGlobalState(t)

	err := Sync()
	if err != nil {
		t.Errorf("Sync() with nil logger should return nil, got %v", err)
	}
}

func TestLogWithFields(t *testing.T) {
	resetGlobalState(t)

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

	// Test various Field types
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

func TestSetLevel_WithLogger(t *testing.T) {
	resetGlobalState(t)

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

	// Test SetLevel function
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
	resetGlobalState(t)

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

	// Test all log levels
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

// Test Init and initLogger
func TestInitLogger_WithConsoleOutput(t *testing.T) {
	resetGlobalState(t)

	cfg := &Config{
		Level:        "debug",
		Format:       "console",
		Outputs:      []string{"console"},
		ColorEnabled: false,
	}

	// Call initLogger directly (not using Init to avoid sync.Once)
	initLogger(cfg)

	if globalLogger == nil {
		t.Error("expected globalLogger to be initialized")
	}
	if globalConfig == nil {
		t.Error("expected globalConfig to be set")
	}
}

func TestInitLogger_WithJSONFormat(t *testing.T) {
	resetGlobalState(t)

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
	resetGlobalState(t)

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
	resetGlobalState(t)

	tmpDir := t.TempDir()
	registerGlobalCleanup(t)

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

	// Verify log file creation
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read log dir: %v", err)
	}
	if len(files) == 0 {
		t.Error("expected log file to be created")
	}
}

func TestInitLogger_WithFileAndConsoleOutput(t *testing.T) {
	resetGlobalState(t)

	tmpDir := t.TempDir()
	registerGlobalCleanup(t)

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
	resetGlobalState(t)

	tmpDir := t.TempDir()
	registerGlobalCleanup(t)

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
	resetGlobalState(t)

	cfg := &Config{
		Level:   "info",
		Format:  "console",
		Outputs: []string{}, // empty outputs, should use default console
	}

	initLogger(cfg)

	if globalLogger == nil {
		t.Error("expected globalLogger with default console output")
	}
}

func TestInitLogger_NilConfig(t *testing.T) {
	resetGlobalState(t)

	// Pass nil, should use default config
	initLogger(nil)

	if globalLogger == nil {
		t.Error("expected globalLogger with default config")
	}
}

func TestBuildConsoleCore_JSONFormat(t *testing.T) {
	resetGlobalState(t)

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
	resetGlobalState(t)

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
	resetGlobalState(t)

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

	core, file := buildFileCore(encoderConfig)
	registerFileCleanup(t, file)
	if core == nil {
		t.Error("expected non-nil core")
	}
}

func TestBuildFileCore_ConsoleFormat(t *testing.T) {
	resetGlobalState(t)

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

	core, file := buildFileCore(encoderConfig)
	registerFileCleanup(t, file)
	if core == nil {
		t.Error("expected non-nil core")
	}
}

func TestBuildFileCore_WithFileName(t *testing.T) {
	resetGlobalState(t)

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

	core, file := buildFileCore(encoderConfig)
	registerFileCleanup(t, file)
	if core == nil {
		t.Error("expected non-nil core")
	}

	// Verify custom filename
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
	resetGlobalState(t)

	// Use temp directory since default would use ./logs
	originalWd, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(originalWd) }()

	globalConfig = &Config{
		Format:  "json",
		LogPath: "", // empty path, should use default ./logs
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:    "ts",
		LevelKey:   "level",
		MessageKey: "msg",
	}

	core, file := buildFileCore(encoderConfig)
	registerFileCleanup(t, file)
	if core == nil {
		t.Error("expected non-nil core with default log path")
	}
}

func TestNewReturnsFileOpenError(t *testing.T) {
	tmpDir := t.TempDir()
	notDirectory := filepath.Join(tmpDir, "not-directory")
	if err := os.WriteFile(notDirectory, []byte("content"), 0o600); err != nil {
		t.Fatalf("建立測試檔案失敗：%v", err)
	}

	cfg := &Config{
		Level:   "info",
		Format:  "json",
		Outputs: []string{"file"},
		LogPath: filepath.Join(notDirectory, "logs"),
	}

	instance, err := New(cfg)
	if err == nil {
		t.Fatal("預期建立 file logger 失敗")
	}
	if instance != nil {
		t.Fatal("建構失敗時不應回傳部分 Instance")
	}
}

func TestNewReturnsInvalidConfigBeforeIO(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "不應建立")
	cfg := &Config{
		Level:   "trace",
		Format:  "json",
		Outputs: []string{"file"},
		LogPath: logPath,
	}

	instance, err := New(cfg)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("錯誤 = %v，預期 ErrInvalidConfig", err)
	}
	if instance != nil {
		t.Fatal("設定無效時不應回傳部分 Instance")
	}
	if _, statErr := os.Stat(logPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("驗證失敗前不應建立目錄，Stat 錯誤 = %v", statErr)
	}
}

func TestNewDoesNotMutateConfig(t *testing.T) {
	cfg := &Config{
		Level:        "DEBUG",
		Format:       "JSON",
		Outputs:      []string{"FILE"},
		LogPath:      t.TempDir(),
		ColorEnabled: false,
	}

	instance, err := New(cfg)
	if err != nil {
		t.Fatalf("建立 Instance 失敗：%v", err)
	}
	t.Cleanup(func() {
		if err := instance.Close(); err != nil {
			t.Errorf("關閉 Instance 失敗：%v", err)
		}
	})

	if cfg.Level != "DEBUG" || cfg.Format != "JSON" || cfg.Outputs[0] != "FILE" {
		t.Fatalf("New 不應修改輸入 Config：%+v", cfg)
	}
}

func TestNewWithOptionsUsesConfiguredPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不提供相同 POSIX mode 語意")
	}

	parent := t.TempDir()
	referenceDir := filepath.Join(parent, "reference")
	if err := os.Mkdir(referenceDir, 0o750); err != nil {
		t.Fatalf("建立參考目錄失敗：%v", err)
	}
	referenceFile := filepath.Join(referenceDir, "reference.log")
	//nolint:gosec // 測試刻意建立 0640 參考檔，驗證自訂 group-read 權限。
	if err := os.WriteFile(referenceFile, nil, 0o640); err != nil {
		t.Fatalf("建立參考檔案失敗：%v", err)
	}

	base := filepath.Join(parent, "logs")
	instance, err := NewWithOptions(
		fileOutputTestConfig(base, "app.log"),
		WithDirPerm(0o750),
		WithFilePerm(0o640),
	)
	if err != nil {
		t.Fatalf("以自訂 permissions 建立 Instance 失敗：%v", err)
	}
	t.Cleanup(func() {
		if err := instance.Close(); err != nil {
			t.Errorf("關閉 Instance 失敗：%v", err)
		}
	})

	assertSamePermission(t, base, referenceDir)
	assertSamePermission(t, filepath.Join(base, "app.log"), referenceFile)
}

func TestConfigureWithOptionsUsesConfiguredPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不提供相同 POSIX mode 語意")
	}
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })

	parent := t.TempDir()
	referenceDir := filepath.Join(parent, "reference")
	if err := os.Mkdir(referenceDir, 0o750); err != nil {
		t.Fatalf("建立參考目錄失敗：%v", err)
	}
	referenceFile := filepath.Join(referenceDir, "reference.log")
	//nolint:gosec // 測試刻意建立 0640 參考檔，驗證自訂 group-read 權限。
	if err := os.WriteFile(referenceFile, nil, 0o640); err != nil {
		t.Fatalf("建立參考檔案失敗：%v", err)
	}

	base := filepath.Join(parent, "logs")
	outputs := []string{"file"}
	fileName := "app.log"
	cleanup, err := ConfigureWithOptions(&ConfigPatch{
		Outputs:  &outputs,
		LogPath:  &base,
		FileName: &fileName,
	}, WithDirPerm(0o750), WithFilePerm(0o640))
	if err != nil {
		t.Fatalf("以自訂 permissions 設定全域 logger 失敗：%v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Errorf("清理全域 logger 失敗：%v", err)
		}
	})

	assertSamePermission(t, base, referenceDir)
	assertSamePermission(t, filepath.Join(base, "app.log"), referenceFile)
}

func TestNewRollsBackResourcesInReverseOrder(t *testing.T) {
	firstErr := errors.New("第一個關閉錯誤")
	secondErr := errors.New("第二個關閉錯誤")
	order := make([]string, 0, 2)
	closers := []io.Closer{
		&recordingCloser{name: "first", order: &order, err: firstErr},
		&recordingCloser{name: "second", order: &order, err: secondErr},
	}

	err := closeOwnedResources(closers, "回收 logger 資源")
	if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("回滾錯誤未保留完整錯誤鏈：%v", err)
	}
	if len(order) != 2 || order[0] != "second" || order[1] != "first" {
		t.Fatalf("資源關閉順序 = %v，預期為 [second first]", order)
	}
}

func TestNewCleanupIsConcurrentAndIdempotent(t *testing.T) {
	cfg := &Config{
		Level:    "debug",
		Format:   "json",
		Outputs:  []string{"file"},
		LogPath:  t.TempDir(),
		FileName: "instance.log",
	}

	instance, err := New(cfg)
	if err != nil {
		t.Fatalf("建立 Instance 失敗：%v", err)
	}
	instance.Logger().Info("cleanup 測試")
	if err := instance.Sync(); err != nil {
		t.Fatalf("同步 Instance 失敗：%v", err)
	}

	const callers = 8
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- instance.Close()
		}()
	}
	wg.Wait()
	close(errs)

	for closeErr := range errs {
		if closeErr != nil {
			t.Errorf("Close 回傳非預期錯誤：%v", closeErr)
		}
	}
	if err := instance.Sync(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("關閉後 Sync 錯誤 = %v，預期包裝 os.ErrClosed", err)
	}
}

func TestConfigureCanRetryAfterFailure(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })

	tmpDir := t.TempDir()
	notDirectory := filepath.Join(tmpDir, "not-directory")
	if err := os.WriteFile(notDirectory, []byte("content"), 0o600); err != nil {
		t.Fatalf("建立測試檔案失敗：%v", err)
	}
	outputs := []string{"file"}
	badPath := filepath.Join(notDirectory, "logs")

	cleanup, err := Configure(&ConfigPatch{Outputs: &outputs, LogPath: &badPath})
	if err == nil {
		t.Fatal("第一次 Configure 預期失敗")
	}
	if cleanup != nil {
		t.Fatal("Configure 失敗時不應回傳 cleanup")
	}
	if GetLogger() != nil {
		t.Fatal("Configure 失敗時不應發布半初始化 logger")
	}

	cleanup, err = Configure(nil)
	if err != nil {
		t.Fatalf("修正設定後 Configure 應可重試：%v", err)
	}
	if GetLogger() == nil {
		t.Fatal("Configure 成功後應發布全域 logger")
	}
	if err := cleanup(); err != nil {
		t.Fatalf("清理全域 logger 失敗：%v", err)
	}
}

func TestConfigureRejectsSecondSuccess(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })

	cleanup, err := Configure(nil)
	if err != nil {
		t.Fatalf("第一次 Configure 失敗：%v", err)
	}
	first := GetLogger()

	secondCleanup, err := Configure(nil)
	if !errors.Is(err, ErrAlreadyConfigured) {
		t.Fatalf("第二次 Configure 錯誤 = %v，預期 ErrAlreadyConfigured", err)
	}
	if secondCleanup != nil {
		t.Fatal("第二次 Configure 不應回傳 cleanup")
	}
	if GetLogger() != first {
		t.Fatal("第二次 Configure 不應替換已發布 logger")
	}
	if err := cleanup(); err != nil {
		t.Fatalf("清理全域 logger 失敗：%v", err)
	}
}

func TestConfigureConcurrent(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })

	const callers = 8
	type result struct {
		cleanup func() error
		err     error
	}
	results := make(chan result, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cleanup, err := Configure(nil)
			results <- result{cleanup: cleanup, err: err}
		}()
	}
	wg.Wait()
	close(results)

	var success int
	var cleanup func() error
	for result := range results {
		if result.err == nil {
			success++
			cleanup = result.cleanup
			continue
		}
		if !errors.Is(result.err, ErrAlreadyConfigured) {
			t.Errorf("競爭 Configure 回傳非預期錯誤：%v", result.err)
		}
	}
	if success != 1 {
		t.Fatalf("成功次數 = %d，預期為 1", success)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("清理全域 logger 失敗：%v", err)
	}
}

func TestLegacyInitCompatibility(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })

	Init(nil)
	if GetLogger() == nil {
		t.Fatal("Init(nil) 應維持既有初始化行為")
	}

	Init(nil)
}

type recordingCloser struct {
	name  string
	order *[]string
	err   error
}

func (c *recordingCloser) Close() error {
	*c.order = append(*c.order, c.name)
	return c.err
}
