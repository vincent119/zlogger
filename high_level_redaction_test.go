package zlogger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestHighLevelRedactionConfig(t *testing.T) {
	for _, cfg := range []RedactionConfig{
		{}, {Keys: []string{""}}, {Enabled: true, Keys: []string{"token"}},
		{Enabled: true}, {Enabled: true, Keys: []string{""}},
		{Enabled: true, Keys: []string{strings.Repeat("x", 129)}},
		{Enabled: true, Keys: make([]string, 129)},
	} {
		c := DefaultConfig()
		c.Redaction = cfg
		err := c.Validate()
		_, resolveErr := (&ConfigPatch{Redaction: &cfg}).Resolve()
		wantInvalid := cfg.Enabled && (len(cfg.Keys) != 1 || len(cfg.Keys[0]) == 0 || len(cfg.Keys[0]) > 128)
		if errors.Is(err, ErrInvalidConfig) != wantInvalid || errors.Is(resolveErr, ErrInvalidConfig) != wantInvalid {
			t.Fatalf("validation mismatch: Validate=%v Resolve=%v", err, resolveErr)
		}
	}
	var patch ConfigPatch
	if err := json.Unmarshal([]byte(`{"redaction":{"enabled":true,"keys":["token"]}}`), &patch); err != nil {
		t.Fatal(err)
	}
	c, err := patch.Resolve()
	if err != nil || !c.Redaction.Enabled {
		t.Fatalf("resolved config=%v error=%v", c, err)
	}
	clone := c.normalizedCopy()
	merged := DefaultConfig().Merge(c)
	patch.Redaction.Keys[0] = "changed"
	if c.Redaction.Keys[0] != "token" {
		t.Fatal("Resolve retained caller keys")
	}
	c.Redaction.Keys[0] = "changed"
	if clone.Redaction.Keys[0] != "token" || merged.Redaction.Keys[0] != "token" {
		t.Fatal("copy or Merge retained caller keys")
	}
	merged.Merge(&Config{})
	if merged.Redaction.Enabled || len(merged.Redaction.Keys) != 0 {
		t.Fatal("Merge must replace the entire policy")
	}
	for _, p := range []*ConfigPatch{nil, {}, {Redaction: &RedactionConfig{}}} {
		c, err := p.Resolve()
		if err != nil || c.Redaction.Enabled {
			t.Fatal("redaction must default to disabled")
		}
	}
	for _, typ := range []reflect.Type{reflect.TypeFor[Config](), reflect.TypeFor[ConfigPatch](), reflect.TypeFor[RedactionConfig]()} {
		for i := range typ.NumField() {
			field := typ.Field(i)
			if typ != reflect.TypeFor[RedactionConfig]() && field.Name != "Redaction" {
				continue
			}
			for _, format := range []string{"json", "yaml", "toml", "mapstructure"} {
				if field.Tag.Get(format) == "" {
					t.Errorf("%s.%s missing %s tag", typ.Name(), field.Name, format)
				}
			}
		}
	}
}

func highLevelConfig(t *testing.T, enabled bool) *Config {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Format, cfg.FileName, cfg.LogPath = "json", "app.log", t.TempDir()
	cfg.Outputs = []string{"file"}
	cfg.AddCaller, cfg.ColorEnabled = false, false
	cfg.Redaction = RedactionConfig{Enabled: enabled, Keys: []string{"token", "path"}}
	return cfg
}

func highLevelPatch(cfg *Config) *ConfigPatch {
	return &ConfigPatch{Format: &cfg.Format, Level: &cfg.Level, Outputs: &cfg.Outputs,
		LogPath: &cfg.LogPath, FileName: &cfg.FileName, AddCaller: &cfg.AddCaller,
		ColorEnabled: &cfg.ColorEnabled, Redaction: &cfg.Redaction}
}

func writeHighLevelEntries(logger *zap.Logger) {
	logger.Info("direct", String("token", "secret-value"))
	logger.With(String("token", "secret-value")).Info("bound")
	logger.WithLazy(String("token", "secret-value")).Info("lazy")
	ctx := WithContext(context.Background(), String("token", "secret-value"))
	logger.Info("context", FromContext(ctx)...)
	logger.Sugar().Infow("sugar", "token", "secret-value")
	logger.Info("containers", Object("object", zapcore.ObjectMarshalerFunc(func(enc zapcore.ObjectEncoder) error {
		enc.AddString("token", "secret-value")
		return nil
	})), Array("array", zapcore.ArrayMarshalerFunc(func(enc zapcore.ArrayEncoder) error {
		return enc.AppendObject(zapcore.ObjectMarshalerFunc(func(obj zapcore.ObjectEncoder) error {
			obj.AddString("token", "secret-value")
			return nil
		}))
	})), String("request_id", "keep-value"), Redacted("manual"))
}

func assertHighLevelEntries(t *testing.T, data []byte, enabled bool) {
	t.Helper()
	text := string(data)
	if strings.Contains(text, "secret-value") == enabled {
		t.Fatalf("redaction enabled=%t: %s", enabled, data)
	}
	for _, message := range []string{"direct", "bound", "lazy", "context", "sugar", "containers", "keep-value", "[REDACTED]"} {
		if !strings.Contains(text, message) {
			t.Fatalf("missing %q: %s", message, data)
		}
	}
}

func readHighLevelFile(t *testing.T, path string) []byte {
	t.Helper()
	// Test paths are confined to t.TempDir and fixed log names.
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestHighLevelRedactionStandard(t *testing.T) {
	for _, entry := range []string{"New", "NewWithOptions", "Configure", "ConfigureWithOptions", "Init"} {
		for _, outputs := range [][]string{{"console"}, {"file"}, {"console", "file"}} {
			for _, format := range []string{"json", "console"} {
				for _, enabled := range []bool{false, true} {
					t.Run(entry+"/"+strings.Join(outputs, "+")+"/"+format+"/"+map[bool]string{false: "off", true: "on"}[enabled], func(t *testing.T) {
						resetGlobalState(t)
						t.Cleanup(func() { resetGlobalState(t) })
						cfg := highLevelConfig(t, enabled)
						cfg.Format, cfg.Outputs = format, outputs
						capture, err := os.CreateTemp(t.TempDir(), "stdout")
						if err != nil {
							t.Fatal(err)
						}
						previous := os.Stdout
						os.Stdout = capture
						t.Cleanup(func() { os.Stdout = previous; _ = capture.Close() })
						var logger *zap.Logger
						var cleanup func() error
						switch entry {
						case "New", "NewWithOptions":
							var instance *Instance
							if entry == "New" {
								instance, err = New(cfg)
							} else {
								instance, err = NewWithOptions(cfg, WithFilePerm(0o600))
							}
							if err == nil {
								logger, cleanup = instance.Logger(), instance.Close
							}
						case "Configure":
							cleanup, err = Configure(highLevelPatch(cfg))
							logger = GetLogger()
						case "ConfigureWithOptions":
							cleanup, err = ConfigureWithOptions(highLevelPatch(cfg), WithFilePerm(0o600))
							logger = GetLogger()
						case "Init":
							Init(cfg)
							logger, cleanup = GetLogger(), globalCleanup
						}
						if err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { _ = cleanup() })
						cfg.Redaction.Enabled, cfg.Redaction.Keys[0] = !enabled, "changed"
						writeHighLevelEntries(logger)
						if configured {
							InfoContext(WithContext(context.Background(), String("token", "secret-value")), "global context")
						}
						if err := logger.Sync(); err != nil {
							t.Fatal(err)
						}
						if err := cleanup(); err != nil {
							t.Fatal(err)
						}
						for _, output := range outputs {
							path := filepath.Join(cfg.LogPath, cfg.FileName)
							if output == "console" {
								path = capture.Name()
							}
							data := readHighLevelFile(t, path)
							assertHighLevelEntries(t, data, enabled)
							wantPath := cfg.LogPath
							if enabled {
								wantPath = "[REDACTED]"
							}
							gotPath, err := initializationPath(data)
							if err != nil || gotPath != wantPath || !strings.Contains(string(data), "logger initialized") {
								t.Fatalf("initialization must use the same policy: %s", data)
							}
						}
					})
				}
			}
		}
	}
}

// Both encoders serialize structured fields as JSON; decode before comparing OS-specific paths.
func initializationPath(data []byte) (string, error) {
	line := bytes.SplitN(data, []byte("\n"), 2)[0]
	start := bytes.IndexByte(line, '{')
	if start < 0 {
		return "", errors.New("missing initialization fields")
	}
	var fields struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(line[start:], &fields); err != nil {
		return "", err
	}
	return fields.Path, nil
}

func TestInitializationPath(t *testing.T) {
	for _, path := range []string{`C:\Users\runner\Temp\logs`, `/tmp/logs`, `C:\logs\"quoted"`, "[REDACTED]"} {
		encoded, err := json.Marshal(map[string]string{"path": path})
		if err != nil {
			t.Fatal(err)
		}
		for _, prefix := range []string{"", "2026-10-30 INFO logger initialized "} {
			got, err := initializationPath([]byte(prefix + string(encoded) + "\n{\"path\":\"later\"}\n"))
			if err != nil || got != path {
				t.Fatalf("path=%q got=%q error=%v", path, got, err)
			}
		}
	}
	for _, data := range []string{"", "missing fields", "{invalid}"} {
		if _, err := initializationPath([]byte(data)); err == nil {
			t.Fatalf("expected invalid record: %q", data)
		}
	}
}

func TestHighLevelRedactionSplit(t *testing.T) {
	for _, global := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			t.Run(map[bool]string{false: "instance", true: "global"}[global]+map[bool]string{false: "/off", true: "/on"}[enabled], func(t *testing.T) {
				resetGlobalState(t)
				t.Cleanup(func() { resetGlobalState(t) })
				cfg := highLevelConfig(t, enabled)
				var logger *zap.Logger
				var cleanup func() error
				var setLevel func(string) error
				if global {
					fn, err := ConfigureSplit(highLevelPatch(cfg), WithFilePerm(0o600))
					if err != nil {
						t.Fatal(err)
					}
					logger, cleanup = GetLogger(), fn
					setLevel = func(level string) error { SetLevel(level); return nil }
				} else {
					i, err := NewSplit(cfg, WithFilePerm(0o600))
					if err != nil {
						t.Fatal(err)
					}
					logger, cleanup, setLevel = i.Logger(), i.Close, i.SetLevel
				}
				t.Cleanup(func() { _ = cleanup() })
				cfg.Redaction.Keys[0] = "changed"
				writeHighLevelEntries(logger)
				logger.Warn("warn-only", String("token", "secret-value"))
				logger.Error("error-only", String("token", "secret-value"))
				logger.Debug("filtered-debug", String("token", "secret-value"))
				if err := setLevel("debug"); err != nil {
					t.Fatal(err)
				}
				logger.Debug("enabled-debug", String("token", "secret-value"))
				if err := cleanup(); err != nil {
					t.Fatal(err)
				}
				for _, level := range []string{"info", "warn", "error"} {
					files, err := filepath.Glob(filepath.Join(cfg.LogPath, cfg.FileName+"-"+level+"-*.log"))
					if err != nil || len(files) != 1 {
						t.Fatalf("files=%v err=%v", files, err)
					}
					data := readHighLevelFile(t, files[0])
					if strings.Contains(string(data), "secret-value") == enabled {
						t.Fatalf("%s: %s", level, data)
					}
					if level == "info" {
						assertHighLevelEntries(t, data, enabled)
						if !strings.Contains(string(data), "enabled-debug") || strings.Contains(string(data), "filtered-debug") {
							t.Fatal(string(data))
						}
					}
					for _, other := range []string{"warn", "error"} {
						if strings.Contains(string(data), other+"-only") != (level == other) {
							t.Fatalf("routing %s: %s", level, data)
						}
					}
					if runtime.GOOS != "windows" {
						stat, err := os.Stat(files[0])
						if err != nil || stat.Mode().Perm() != 0o600 {
							t.Fatalf("permissions: %v %v", stat, err)
						}
					}
				}
			})
		}
	}
}

func TestHighLevelRedactionCustomCore(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })
	sink := &recordingWriteSyncCloser{}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(testEncoderConfig()), sink, zap.DebugLevel)
	calls := 0
	policy := RedactionConfig{Enabled: true, Keys: []string{"token"}}
	cleanup, err := ConfigureCore(&ConfigPatch{Redaction: &policy}, core, WithCoreCleanup(func() error { calls++; return nil }))
	if !errors.Is(err, ErrInvalidConfig) || cleanup != nil || configured || calls != 0 {
		t.Fatalf("cleanup=%v err=%v calls=%d", cleanup == nil, err, calls)
	}
	if syncs, closes := sink.counts(); syncs != 0 || closes != 0 {
		t.Fatalf("sync=%d close=%d", syncs, closes)
	}
	var buffer bytes.Buffer
	redacting, err := NewRedactingCore(zapcore.NewJSONEncoder(testEncoderConfig()), zapcore.AddSync(&buffer), zap.DebugLevel, policy)
	if err != nil {
		t.Fatal(err)
	}
	policy.Enabled, policy.Keys = false, []string{""}
	cleanup, err = ConfigureCore(&ConfigPatch{Redaction: &policy}, redacting)
	if err != nil {
		t.Fatal(err)
	}
	Info("external", String("token", "secret-value"))
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buffer.String(), "secret-value") || !strings.Contains(buffer.String(), "[REDACTED]") {
		t.Fatal(buffer.String())
	}
}

func TestHighLevelRedactionLifecycle(t *testing.T) {
	for _, entry := range []string{"New", "NewSplit", "Configure", "ConfigureSplit"} {
		t.Run(entry, func(t *testing.T) {
			resetGlobalState(t)
			t.Cleanup(func() { resetGlobalState(t) })
			cfg := highLevelConfig(t, true)
			cfg.LogPath = filepath.Join(cfg.LogPath, "not-created")
			cfg.Redaction.Keys = nil
			construct := func() (func() error, error) {
				switch entry {
				case "Configure":
					return Configure(highLevelPatch(cfg))
				case "ConfigureSplit":
					return ConfigureSplit(highLevelPatch(cfg))
				}
				var i *Instance
				var err error
				if entry == "New" {
					i, err = New(cfg)
				} else {
					i, err = NewSplit(cfg)
				}
				if err != nil {
					return nil, err
				}
				return i.Close, nil
			}
			if cleanup, err := construct(); !errors.Is(err, ErrInvalidConfig) || cleanup != nil {
				t.Fatalf("invalid policy: %v", err)
			}
			if _, err := os.Stat(cfg.LogPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("validation created resources: %v", err)
			}
			cfg.Redaction.Keys = []string{"token"}
			if err := os.WriteFile(cfg.LogPath, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if cleanup, err := construct(); err == nil || cleanup != nil {
				t.Fatal("file path must fail")
			}
			if configured || GetLogger() != nil {
				t.Fatal("failed construction published globals")
			}
			if err := os.Remove(cfg.LogPath); err != nil {
				t.Fatal(err)
			}
			cleanup, err := construct()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cleanup() })
			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					if err := cleanup(); err != nil {
						t.Error(err)
					}
				})
			}
			wg.Wait()
		})
	}
	t.Run("split errors and worker shutdown", func(t *testing.T) {
		cfg := highLevelConfig(t, true)
		i, err := NewSplit(cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = i.Close() })
		output := i.closers[0].(*splitInstanceResource).output
		wantSync, wantClose := errors.New("sync failed"), errors.New("close failed")
		sink := &recordingWriteSyncCloser{syncErr: wantSync, closeErr: wantClose}
		output.mutex.Lock()
		if err := output.infoOut.Close(); err != nil {
			t.Fatal(err)
		}
		output.infoOut = sink
		output.mutex.Unlock()
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				if err := i.Close(); !errors.Is(err, wantSync) || !errors.Is(err, wantClose) {
					t.Errorf("Close=%v", err)
				}
			})
		}
		wg.Wait()
		select {
		case <-output.done:
		default:
			t.Fatal("worker still running")
		}
		if syncs, closes := sink.counts(); syncs != 1 || closes != 1 {
			t.Fatalf("sync=%d close=%d", syncs, closes)
		}
		if err := output.openFiles(); !errors.Is(err, os.ErrClosed) {
			t.Fatal(err)
		}
	})
	t.Run("standard close error", func(t *testing.T) {
		i, err := New(highLevelConfig(t, true))
		if err != nil {
			t.Fatal(err)
		}
		if err := i.closers[0].Close(); err != nil {
			t.Fatal(err)
		}
		first := i.Close()
		if !errors.Is(first, os.ErrClosed) || i.Close() != first {
			t.Fatalf("Close=%v", first)
		}
	})
}

func TestHighLevelRedactionObservation(t *testing.T) {
	cfg := highLevelConfig(t, true)
	o := newTestObserver(t, 16)
	i, err := NewSplit(cfg, WithOutputObserver(o))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i.Close() })
	output := i.closers[0].(*splitInstanceResource).output
	// Core.Write exposes encoding failures without writing diagnostics to stderr.
	err = i.Logger().Core().Write(zapcore.Entry{Level: zap.InfoLevel}, []Field{{Type: zapcore.UnknownType}})
	if !errors.Is(err, ErrRedactionFailed) || len(o.Events()) != 0 {
		t.Fatalf("err=%v events=%d", err, len(o.Events()))
	}
	want := errors.New("disk write failed")
	output.mutex.Lock()
	if err := output.infoOut.Close(); err != nil {
		t.Fatal(err)
	}
	output.infoOut = failingObserverSink{writeErr: want}
	output.mutex.Unlock()
	err = i.Logger().Core().Write(zapcore.Entry{Level: zap.InfoLevel}, []Field{String("token", "secret-value")})
	if !errors.Is(err, want) || len(o.Events()) != 1 {
		t.Fatalf("err=%v events=%d", err, len(o.Events()))
	}
	event := <-o.Events()
	if event.Operation != "write" || event.Output != "split" || !errors.Is(event.Err, want) {
		t.Fatal(event)
	}
}

func TestHighLevelRedactionSplitRotation(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	clock := newManualRotationClock(now)
	directory := t.TempDir()
	output, err := newSplitOutput(directory, "app", clock, openSplitFiles)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = output.Close() })
	core := splitCoreForOutputWithRedaction(zapcore.NewJSONEncoder(testEncoderConfig()), output, RedactionConfig{Enabled: true, Keys: []string{"token"}})
	logger := zap.New(core)
	logger.Info("before", String("token", "secret-value"))
	timer := clock.nextTimer(t)
	next := now.AddDate(0, 0, 1)
	clock.setNow(next)
	timer.fire(next)
	clock.nextTimer(t)
	logger.Info("after", String("token", "secret-value"))
	logger.Warn("after-warn", String("token", "secret-value"))
	logger.Error("after-error", String("token", "secret-value"))
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	for _, date := range []time.Time{now, next} {
		for _, level := range []string{"info", "warn", "error"} {
			data := readHighLevelFile(t, filepath.Join(directory, "app-"+level+"-"+date.Format("2006-01-02")+".log"))
			if strings.Contains(string(data), "secret-value") {
				t.Fatal(string(data))
			}
			if (level == "info" || date.Equal(next)) && !strings.Contains(string(data), "[REDACTED]") {
				t.Fatalf("missing masked entry: %s", data)
			}
		}
	}
	select {
	case <-output.done:
	default:
		t.Fatal("rotation worker still running")
	}
}
