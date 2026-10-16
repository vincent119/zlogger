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
	"time"

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

func TestSplitInstancesAreIndependent(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })
	firstCfg := DefaultConfig()
	firstCfg.LogPath = t.TempDir()
	firstCfg.FileName = "first"
	first, err := NewSplit(firstCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := first.Close(); err != nil {
			t.Error(err)
		}
	})
	secondCfg := DefaultConfig()
	secondCfg.LogPath = t.TempDir()
	second, err := NewSplit(secondCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := second.Close(); err != nil {
			t.Error(err)
		}
	})
	if GetLogger() != nil || configured {
		t.Fatal("Instance must not modify globals")
	}
	if err := first.SetLevel("debug"); err != nil {
		t.Fatal(err)
	}
	if !first.Logger().Named("test").With(zap.String("key", "value")).Core().Enabled(zap.DebugLevel) {
		t.Fatal("derived logger must use the updated level")
	}
	if second.Logger().Core().Enabled(zap.DebugLevel) {
		t.Fatal("the second Instance must remain unchanged")
	}
	first.Logger().Debug("independent message")
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.SetLevel("info"); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("SetLevel after close = %v", err)
	}
	if err := first.Sync(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("Sync after close = %v", err)
	}
	second.Logger().Warn("still writable")
	if err := second.Sync(); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(firstCfg.LogPath, "first-info-*.log"))
	if err != nil || len(files) != 1 {
		t.Fatalf("files = %v, error = %v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil || !strings.Contains(string(data), "independent message") {
		t.Fatalf("content = %s, error = %v", data, err)
	}
}

func TestInstanceSetLevel(t *testing.T) {
	var nilInstance *Instance
	if err := nilInstance.SetLevel("info"); !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
	i, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i.Close() })
	for _, input := range []string{"", "invalid"} {
		if err := i.SetLevel(input); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("%q: %v", input, err)
		}
		if i.level.Level() != zap.InfoLevel {
			t.Fatal("invalid input must not change the level")
		}
	}
}

func TestConfigureCorePublishesGlobals(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })
	var buffer bytes.Buffer
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&buffer), zap.DebugLevel)
	cleanup, err := ConfigureCore(nil, core)
	if err != nil {
		t.Fatal(err)
	}
	Info("global message")
	zap.L().Info("zap message")
	child := GetLogger().Named("child").With(zap.String("key", "value"))
	child.Debug("must not be written")
	SetLevel("debug")
	child.Debug("lowered-level message")
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if GetLogger() != nil {
		t.Fatal("previous globals must be restored")
	}
	if _, err := ConfigureCore(nil, core); !errors.Is(err, ErrAlreadyConfigured) {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), "global message") || !strings.Contains(buffer.String(), "zap message") || !strings.Contains(buffer.String(), "lowered-level message") || strings.Contains(buffer.String(), "must not be written") {
		t.Fatal(buffer.String())
	}
}

func TestConfigureCoreCleanupOrderAndErrors(t *testing.T) {
	resetGlobalState(t)
	syncErr := errors.New("sync failed")
	closeErr := errors.New("close failed")
	sink := &recordingWriteSyncCloser{syncErr: syncErr}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), sink, zap.DebugLevel)
	calls := 0
	cleanup, err := ConfigureCore(nil, core, WithCoreCleanup(func() error {
		calls++
		syncCalls, _ := sink.counts()
		if syncCalls != 1 {
			t.Errorf("sync calls = %d", syncCalls)
		}
		return closeErr
	}))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			err := cleanup()
			if !errors.Is(err, syncErr) || !errors.Is(err, closeErr) {
				t.Errorf("cleanup = %v", err)
			}
		})
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("cleanup calls = %d", calls)
	}
	configureMu.Lock()
	globalCleanup = nil // The error was already checked. Avoid reporting the expected error again during isolation cleanup.
	configureMu.Unlock()
	resetGlobalState(t)
}

func TestConfigureSplitRoutesLevels(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })
	directory := t.TempDir()
	level, format := "debug", "json"
	cleanup, err := ConfigureSplit(&ConfigPatch{LogPath: &directory, Level: &level, Format: &format})
	if err != nil {
		t.Fatal(err)
	}
	Debug("debug message")
	Warn("warn message")
	Error("error message")
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	for name, message := range map[string]string{"info": "debug message", "warn": "warn message", "error": "error message"} {
		files, err := filepath.Glob(filepath.Join(directory, "-"+name+"-*.log"))
		if err != nil || len(files) != 1 {
			t.Fatalf("%s: %v %v", name, files, err)
		}
		data, err := os.ReadFile(files[0])
		if err != nil || !strings.Contains(string(data), message) {
			t.Fatalf("%s: %s %v", name, data, err)
		}
	}
}

func TestConfigureCoreOwnership(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })
	core := zapcore.NewNopCore()
	calls := 0
	callback := func() error { calls++; return nil }
	badLevel := "invalid"
	tests := []struct {
		name  string
		patch *ConfigPatch
		core  zapcore.Core
		opts  []CoreOption
	}{
		{name: "nil core"},
		{name: "nil option", core: core, opts: []CoreOption{nil}},
		{name: "nil callback", core: core, opts: []CoreOption{WithCoreCleanup(nil)}},
		{name: "duplicate callback", core: core, opts: []CoreOption{WithCoreCleanup(callback), WithCoreCleanup(callback)}},
		{name: "invalid configuration", core: core, patch: &ConfigPatch{Level: &badLevel}, opts: []CoreOption{WithCoreCleanup(callback)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup, err := ConfigureCore(tt.patch, tt.core, tt.opts...)
			if cleanup != nil || !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("cleanup=%v, error=%v", cleanup != nil, err)
			}
			if calls != 0 || configured {
				t.Fatal("failure must not transfer ownership or publish globals")
			}
		})
	}
	cleanup, err := ConfigureCore(nil, core, WithCoreCleanup(callback))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ConfigureCore(nil, core, WithCoreCleanup(callback)); !errors.Is(err, ErrAlreadyConfigured) {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("duplicate configuration must not invoke the callback")
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("callback must run once")
	}
}

func TestNewSplitRollback(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		opts   []FileOutputOption
	}{
		{name: "invalid level", mutate: func(c *Config) { c.Level = "invalid" }},
		{name: "empty directory", mutate: func(c *Config) { c.LogPath = "" }},
		{name: "unsafe prefix", mutate: func(c *Config) { c.FileName = "../outside" }},
		{name: "invalid permissions", opts: []FileOutputOption{WithFilePerm(0)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.LogPath = t.TempDir()
			if tt.mutate != nil {
				tt.mutate(cfg)
			}
			i, err := NewSplit(cfg, tt.opts...)
			if err == nil || i != nil {
				t.Fatalf("instance=%v, error=%v", i, err)
			}
		})
	}
}

func TestSplitInstanceCloseOrderAndErrors(t *testing.T) {
	syncErr, closeErr := errors.New("sync failed"), errors.New("close failed")
	sink := &recordingWriteSyncCloser{syncErr: syncErr, closeErr: closeErr}
	output := &SplitOutput{infoOut: sink}
	i := &Instance{closers: []io.Closer{&splitInstanceResource{output: output}}}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			err := i.Close()
			if !errors.Is(err, syncErr) || !errors.Is(err, closeErr) {
				t.Errorf("Close = %v", err)
			}
		})
	}
	wg.Wait()
	syncCalls, closeCalls := sink.counts()
	if syncCalls != 1 || closeCalls != 1 {
		t.Fatalf("Sync=%d Close=%d", syncCalls, closeCalls)
	}
}

func TestGlobalConfigurationEntrypointsShareGuard(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })
	directory := t.TempDir()
	entries := []func() (func() error, error){
		func() (func() error, error) { return Configure(nil) },
		func() (func() error, error) { return ConfigureCore(nil, zapcore.NewNopCore()) },
		func() (func() error, error) { return ConfigureSplit(&ConfigPatch{LogPath: &directory}) },
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for _, entry := range entries {
		wg.Go(func() {
			_, err := entry()
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				success++
			} else if !errors.Is(err, ErrAlreadyConfigured) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("successful calls=%d", success)
	}
	for _, entry := range entries {
		if _, err := entry(); !errors.Is(err, ErrAlreadyConfigured) {
			t.Fatal(err)
		}
	}
}

func TestConfigureSplitFailureCanRetry(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })
	directory := t.TempDir()
	badLevel := "invalid"
	if cleanup, err := ConfigureSplit(&ConfigPatch{Level: &badLevel}); err == nil || cleanup != nil {
		t.Fatal("invalid configuration must fail")
	}
	file := filepath.Join(directory, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(file, "child")
	if cleanup, err := ConfigureSplit(&ConfigPatch{LogPath: &badPath}); err == nil || cleanup != nil {
		t.Fatal("invalid path must fail")
	}
	if configured || GetLogger() != nil {
		t.Fatal("failure must not publish globals")
	}
	cleanup, err := ConfigureSplit(&ConfigPatch{LogPath: &directory})
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
}

func TestConfigureSplitCleanupStopsWorker(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })
	cfg := DefaultConfig()
	cfg.LogPath = t.TempDir()
	i, err := NewSplit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	output := i.closers[0].(*splitInstanceResource).output
	configureMu.Lock()
	cleanup := publishGlobal(i, cfg, i.Close)
	configureMu.Unlock()
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-output.done:
	default:
		t.Fatal("worker must stop before cleanup returns")
	}
	if err := output.openFiles(); !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
}

func TestSplitInstanceCaller(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LogPath = t.TempDir()
	cfg.Format = "json"
	i, err := NewSplit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	i.Logger().Info("caller message")
	if err := i.Close(); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(cfg.LogPath, "-info-*.log"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "core_test.go:") {
		t.Fatalf("caller = %s", data)
	}
}

func TestConfigureCoreSetLevel(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })
	var buffer bytes.Buffer
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&buffer), zap.WarnLevel)
	cleanup, err := ConfigureCore(nil, core)
	if err != nil {
		t.Fatal(err)
	}
	SetLevel("debug")
	if GetLogger().Core().Enabled(zap.InfoLevel) {
		t.Fatal("must not enable levels rejected by the original core")
	}
	entry := zapcore.Entry{Level: zap.DebugLevel, Message: "must not be written"}
	if GetLogger().Core().Check(entry, nil) != nil {
		t.Fatal("Check must respect the original core filter")
	}
	SetLevel("error")
	if GetLogger().Core().Check(zapcore.Entry{Level: zap.WarnLevel}, nil) != nil {
		t.Fatal("Check must respect the dynamic gate")
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
}

func TestInstanceSetLevelConcurrentWithClose(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LogPath = t.TempDir()
	i, err := NewSplit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			err := i.SetLevel("DEBUG")
			if err != nil && !errors.Is(err, os.ErrClosed) {
				t.Error(err)
			}
		})
	}
	wg.Go(func() {
		if err := i.Close(); err != nil {
			t.Error(err)
		}
	})
	wg.Wait()
}

func TestConfigureSplitCreationRollback(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })
	directory := t.TempDir()
	date := time.Now().Format("2006-01-02")
	blocked := filepath.Join(directory, "-error-"+date+".log")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	cleanup, err := ConfigureSplit(&ConfigPatch{LogPath: &directory})
	if err == nil || cleanup != nil || configured || GetLogger() != nil {
		t.Fatalf("partial initialization must not publish globals, error=%v", err)
	}
	// On Windows, renaming the created info file fails if its handle remains open.
	info := filepath.Join(directory, "-info-"+date+".log")
	if err := os.Rename(info, info+".moved"); err != nil {
		t.Fatalf("handle must be closed after rollback: %v", err)
	}
}

func TestNewSplitDefaultsAndOptions(t *testing.T) {
	t.Chdir(t.TempDir())
	i, err := NewSplit(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := i.Close(); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.LogPath = t.TempDir()
	cfg.AddStacktrace, cfg.Development, cfg.AddCaller = true, true, false
	i, err = NewSplit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	i.Logger().Error("message with stack trace")
	if err := i.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewSplitUsesConfiguredPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not provide equivalent POSIX mode semantics")
	}
	parent := t.TempDir()
	referenceDir, referenceFile := createPermissionReference(t, parent)
	cfg := DefaultConfig()
	cfg.LogPath = filepath.Join(parent, "logs")
	cfg.FileName = "app"
	i, err := NewSplit(cfg, WithDirPerm(0o750), WithFilePerm(0o640))
	if err != nil {
		t.Fatal(err)
	}
	if err := i.Close(); err != nil {
		t.Fatal(err)
	}
	assertSamePermission(t, cfg.LogPath, referenceDir)
	assertCreatedSplitFilePermissions(t, cfg.LogPath, "app", referenceFile)
}

func TestBuildEncoderConfigColorContract(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		enabled   bool
		wantColor bool
	}{
		{name: "console with color", format: "console", enabled: true, wantColor: true},
		{name: "console without color", format: "console", enabled: false, wantColor: false},
		{name: "JSON with color", format: "json", enabled: true, wantColor: false},
		{name: "JSON without color", format: "json", enabled: false, wantColor: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Format: tt.format, ColorEnabled: tt.enabled}
			entry := zapcore.Entry{Level: zap.InfoLevel, Message: "test message"}
			encoded, err := newEncoder(tt.format, buildEncoderConfig(cfg)).EncodeEntry(entry, nil)
			if err != nil {
				t.Fatalf("encode log entry: %v", err)
			}
			t.Cleanup(encoded.Free)

			level := encoded.String()
			if tt.format == "json" {
				var output struct {
					Level string `json:"level"`
				}
				if err := json.Unmarshal(encoded.Bytes(), &output); err != nil {
					t.Fatalf("decode JSON log entry: %v", err)
				}
				level = output.Level
			}

			if gotColor := strings.Contains(level, "\x1b["); gotColor != tt.wantColor {
				t.Errorf("format=%s color_enabled=%t level=%q, colored=%t, want=%t",
					tt.format, tt.enabled, level, gotColor, tt.wantColor)
			}
		})
	}
}

// resetGlobalState isolates tests that modify globals and reports resource cleanup errors.
func resetGlobalState(t testing.TB) {
	t.Helper()

	configureMu.Lock()
	cleanup := globalCleanup
	configureMu.Unlock()
	if cleanup != nil {
		if err := cleanup(); err != nil {
			t.Errorf("clean up global logger: %v", err)
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
		t.Fatal("test log file must not be nil")
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("close test log file: %v", err)
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
		t.Fatalf("create test file: %v", err)
	}

	cfg := &Config{
		Level:   "info",
		Format:  "json",
		Outputs: []string{"file"},
		LogPath: filepath.Join(notDirectory, "logs"),
	}

	instance, err := New(cfg)
	if err == nil {
		t.Fatal("file logger construction must fail")
	}
	if instance != nil {
		t.Fatal("failed construction must not return a partial Instance")
	}
}

func TestNewReturnsInvalidConfigBeforeIO(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "must-not-exist")
	cfg := &Config{
		Level:   "trace",
		Format:  "json",
		Outputs: []string{"file"},
		LogPath: logPath,
	}

	instance, err := New(cfg)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
	if instance != nil {
		t.Fatal("invalid configuration must not return a partial Instance")
	}
	if _, statErr := os.Stat(logPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("validation failure must not create directories, Stat error = %v", statErr)
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
		t.Fatalf("create Instance: %v", err)
	}
	t.Cleanup(func() {
		if err := instance.Close(); err != nil {
			t.Errorf("close Instance: %v", err)
		}
	})

	if cfg.Level != "DEBUG" || cfg.Format != "JSON" || cfg.Outputs[0] != "FILE" {
		t.Fatalf("New must not modify the input Config: %+v", cfg)
	}
}

func TestNewWithOptionsUsesConfiguredPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not provide equivalent POSIX mode semantics")
	}

	parent := t.TempDir()
	referenceDir := filepath.Join(parent, "reference")
	if err := os.Mkdir(referenceDir, 0o750); err != nil {
		t.Fatalf("create reference directory: %v", err)
	}
	referenceFile := filepath.Join(referenceDir, "reference.log")
	//nolint:gosec // Deliberately create a 0640 reference file to verify custom group-read permissions.
	if err := os.WriteFile(referenceFile, nil, 0o640); err != nil {
		t.Fatalf("create reference file: %v", err)
	}

	base := filepath.Join(parent, "logs")
	instance, err := NewWithOptions(
		fileOutputTestConfig(base, "app.log"),
		WithDirPerm(0o750),
		WithFilePerm(0o640),
	)
	if err != nil {
		t.Fatalf("create Instance with custom permissions: %v", err)
	}
	t.Cleanup(func() {
		if err := instance.Close(); err != nil {
			t.Errorf("close Instance: %v", err)
		}
	})

	assertSamePermission(t, base, referenceDir)
	assertSamePermission(t, filepath.Join(base, "app.log"), referenceFile)
}

func TestConfigureWithOptionsUsesConfiguredPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not provide equivalent POSIX mode semantics")
	}
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })

	parent := t.TempDir()
	referenceDir := filepath.Join(parent, "reference")
	if err := os.Mkdir(referenceDir, 0o750); err != nil {
		t.Fatalf("create reference directory: %v", err)
	}
	referenceFile := filepath.Join(referenceDir, "reference.log")
	//nolint:gosec // Deliberately create a 0640 reference file to verify custom group-read permissions.
	if err := os.WriteFile(referenceFile, nil, 0o640); err != nil {
		t.Fatalf("create reference file: %v", err)
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
		t.Fatalf("configure global logger with custom permissions: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Errorf("clean up global logger: %v", err)
		}
	})

	assertSamePermission(t, base, referenceDir)
	assertSamePermission(t, filepath.Join(base, "app.log"), referenceFile)
}

func TestNewRollsBackResourcesInReverseOrder(t *testing.T) {
	firstErr := errors.New("first close error")
	secondErr := errors.New("second close error")
	order := make([]string, 0, 2)
	closers := []io.Closer{
		&recordingCloser{name: "first", order: &order, err: firstErr},
		&recordingCloser{name: "second", order: &order, err: secondErr},
	}

	err := closeOwnedResources(closers, "roll back logger resources")
	if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("rollback lost part of the error chain: %v", err)
	}
	if len(order) != 2 || order[0] != "second" || order[1] != "first" {
		t.Fatalf("resource close order = %v, want [second first]", order)
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
		t.Fatalf("create Instance: %v", err)
	}
	instance.Logger().Info("cleanup test")
	if err := instance.Sync(); err != nil {
		t.Fatalf("sync Instance: %v", err)
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
			t.Errorf("Close returned an unexpected error: %v", closeErr)
		}
	}
	if err := instance.Sync(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("Sync error after close = %v, want wrapped os.ErrClosed", err)
	}
}

func TestConfigureCanRetryAfterFailure(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })

	tmpDir := t.TempDir()
	notDirectory := filepath.Join(tmpDir, "not-directory")
	if err := os.WriteFile(notDirectory, []byte("content"), 0o600); err != nil {
		t.Fatalf("create test file: %v", err)
	}
	outputs := []string{"file"}
	badPath := filepath.Join(notDirectory, "logs")

	cleanup, err := Configure(&ConfigPatch{Outputs: &outputs, LogPath: &badPath})
	if err == nil {
		t.Fatal("first Configure must fail")
	}
	if cleanup != nil {
		t.Fatal("failed Configure must not return cleanup")
	}
	if GetLogger() != nil {
		t.Fatal("failed Configure must not publish a partially initialized logger")
	}

	cleanup, err = Configure(nil)
	if err != nil {
		t.Fatalf("Configure must allow retry after correcting settings: %v", err)
	}
	if GetLogger() == nil {
		t.Fatal("successful Configure must publish a global logger")
	}
	if err := cleanup(); err != nil {
		t.Fatalf("clean up global logger: %v", err)
	}
}

func TestConfigureRejectsSecondSuccess(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })

	cleanup, err := Configure(nil)
	if err != nil {
		t.Fatalf("first Configure failed: %v", err)
	}
	first := GetLogger()

	secondCleanup, err := Configure(nil)
	if !errors.Is(err, ErrAlreadyConfigured) {
		t.Fatalf("second Configure error = %v, want ErrAlreadyConfigured", err)
	}
	if secondCleanup != nil {
		t.Fatal("second Configure must not return cleanup")
	}
	if GetLogger() != first {
		t.Fatal("second Configure must not replace the published logger")
	}
	if err := cleanup(); err != nil {
		t.Fatalf("clean up global logger: %v", err)
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
			t.Errorf("concurrent Configure returned an unexpected error: %v", result.err)
		}
	}
	if success != 1 {
		t.Fatalf("successful calls = %d, want 1", success)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("clean up global logger: %v", err)
	}
}

func TestLegacyInitCompatibility(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })

	Init(nil)
	if GetLogger() == nil {
		t.Fatal("Init(nil) must retain legacy initialization behavior")
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
