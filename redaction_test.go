package zlogger

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func redactingTestCore(t *testing.T, sink zapcore.WriteSyncer, cfg RedactionConfig) zapcore.Core {
	t.Helper()
	core, err := NewRedactingCore(zapcore.NewJSONEncoder(testEncoderConfig()), sink, zap.DebugLevel, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return core
}

func TestRedactingCoreEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			var output bytes.Buffer
			cfg := RedactionConfig{Enabled: enabled, Keys: []string{"token"}}
			core := redactingTestCore(t, zapcore.AddSync(&output), cfg)
			cfg.Keys[0] = "other"
			zap.New(core).Info("test", zap.String("token", "測試秘密"), Redacted("manual"))
			if strings.Contains(output.String(), "測試秘密") == enabled {
				t.Fatal(output.String())
			}
			if !strings.Contains(output.String(), "[REDACTED]") {
				t.Fatal(output.String())
			}
		})
	}
}

func TestRedactingCoreConfig(t *testing.T) {
	for _, keys := range [][]string{nil, {""}, {strings.Repeat("a", 129)}, make([]string, 129)} {
		_, err := NewRedactingCore(zapcore.NewJSONEncoder(testEncoderConfig()), zapcore.AddSync(&bytes.Buffer{}), zap.DebugLevel, RedactionConfig{Enabled: true, Keys: keys})
		if !errors.Is(err, ErrInvalidConfig) {
			t.Fatal(err)
		}
	}
	if _, err := NewRedactingCore(nil, zapcore.AddSync(&bytes.Buffer{}), zap.DebugLevel, RedactionConfig{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatal(err)
	}
	encoder := zapcore.NewJSONEncoder(testEncoderConfig())
	if _, err := NewRedactingCore(encoder, nil, zap.DebugLevel, RedactionConfig{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatal(err)
	}
	if _, err := NewRedactingCore(encoder, zapcore.AddSync(&bytes.Buffer{}), nil, RedactionConfig{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatal(err)
	}
	if _, err := NewRedactingCore(encoder, zapcore.AddSync(&bytes.Buffer{}), zap.DebugLevel, RedactionConfig{Keys: []string{""}}); err != nil {
		t.Fatal("disabled redaction must ignore Keys", err)
	}
}

func TestRedactingCoreKeys(t *testing.T) {
	var output bytes.Buffer
	fields := []zap.Field{zap.String("ToKeN", "測試秘密"), zap.String("token.extra", "公開"), zap.Skip(), zap.Time("time", time.Now())}
	core := redactingTestCore(t, zapcore.AddSync(&output), RedactionConfig{Enabled: true, Keys: []string{"token", "TOKEN"}})
	if err := core.Write(zapcore.Entry{}, fields); err != nil {
		t.Fatal(err)
	}
	if fields[0].String != "測試秘密" || strings.Contains(output.String(), "測試秘密") || !strings.Contains(output.String(), "公開") {
		t.Fatal(output.String())
	}
}

type forbiddenRedactionValue struct{}

func (forbiddenRedactionValue) Error() string  { panic("must not evaluate error") }
func (forbiddenRedactionValue) String() string { panic("must not evaluate Stringer") }
func (forbiddenRedactionValue) MarshalJSON() ([]byte, error) {
	panic("must not encode through reflection")
}

func TestRedactingCoreOpaque(t *testing.T) {
	var output bytes.Buffer
	core := redactingTestCore(t, zapcore.AddSync(&output), RedactionConfig{Enabled: true, Keys: []string{"token"}})
	logger := zap.New(core).With(zap.Error(forbiddenRedactionValue{}), zap.Stringer("detail", forbiddenRedactionValue{}))
	logger.Info("test", zap.Reflect("object", forbiddenRedactionValue{}))
	if bytes.Count(output.Bytes(), []byte("[REDACTED]")) != 3 {
		t.Fatal(output.String())
	}
}

type redactionObjectFunc func(zapcore.ObjectEncoder) error

func (f redactionObjectFunc) MarshalLogObject(e zapcore.ObjectEncoder) error { return f(e) }

type redactionArrayFunc func(zapcore.ArrayEncoder) error

func (f redactionArrayFunc) MarshalLogArray(e zapcore.ArrayEncoder) error { return f(e) }

func TestRedactingCoreNested(t *testing.T) {
	var output bytes.Buffer
	core := redactingTestCore(t, zapcore.AddSync(&output), RedactionConfig{Enabled: true, Keys: []string{"token", "private"}})
	object := redactionObjectFunc(func(e zapcore.ObjectEncoder) error {
		e.AddString("TOKEN", "測試秘密")
		e.AddString("ok", "公開")
		return nil
	})
	array := redactionArrayFunc(func(e zapcore.ArrayEncoder) error { return e.AppendObject(object) })
	zap.New(core).With(zap.Namespace("space")).Info("test", zap.Object("child", object), zap.Array("list", array), zap.Inline(object))
	zap.New(core).With(zap.Namespace("private")).Info("test", zap.String("anything", "測試秘密"))
	if strings.Contains(output.String(), "測試秘密") || !strings.Contains(output.String(), "公開") {
		t.Fatal(output.String())
	}
	if strings.Count(output.String(), "[REDACTED]") != 4 {
		t.Fatal(output.String())
	}
}

func TestRedactingCoreFailsClosed(t *testing.T) {
	failed := redactionObjectFunc(func(e zapcore.ObjectEncoder) error {
		e.AddString("ok", "測試秘密")
		return errors.New("marshaler secret")
	})
	var recurse redactionObjectFunc
	recurse = func(e zapcore.ObjectEncoder) error { return e.AddObject("nested", recurse) }
	oversized := redactionArrayFunc(func(e zapcore.ArrayEncoder) error {
		for range 1025 {
			e.AppendInt(1)
		}
		return nil
	})
	failedArray := redactionArrayFunc(func(zapcore.ArrayEncoder) error { return errors.New("marshaler secret") })
	var recursiveArray redactionArrayFunc
	recursiveArray = func(e zapcore.ArrayEncoder) error { return e.AppendArray(recursiveArray) }
	panicObject := redactionObjectFunc(func(zapcore.ObjectEncoder) error { panic("測試秘密") })
	for _, field := range []zap.Field{zap.Object("panic", panicObject), zap.Array("failure", failedArray), zap.Array("deep", recursiveArray), zap.Object("object", failed), zap.Inline(failed), zap.Object("deep", recurse), zap.Array("huge", oversized), {Type: zapcore.UnknownType}, {Type: 255}} {
		var output bytes.Buffer
		core := redactingTestCore(t, zapcore.AddSync(&output), RedactionConfig{Enabled: true, Keys: []string{"token"}})
		err := core.Write(zapcore.Entry{}, []zap.Field{field})
		if !errors.Is(err, ErrRedactionFailed) || output.Len() != 0 || strings.Contains(err.Error(), "測試秘密") || strings.Contains(err.Error(), "marshaler secret") {
			t.Fatalf("error=%v, output=%s", err, output.String())
		}
		child := core.With([]zap.Field{field})
		if err := child.Write(zapcore.Entry{}, nil); !errors.Is(err, ErrRedactionFailed) {
			t.Fatal(err)
		}
	}
}

func TestRedactingCoreMaskedContainersAndIsolation(t *testing.T) {
	var output bytes.Buffer
	core := redactingTestCore(t, zapcore.AddSync(&output), RedactionConfig{Enabled: true, Keys: []string{"token"}})
	object := redactionObjectFunc(func(zapcore.ObjectEncoder) error { panic("must not evaluate") })
	array := redactionArrayFunc(func(zapcore.ArrayEncoder) error { panic("must not evaluate") })
	logger := zap.New(core)
	logger.Info("containers", zap.Object("token", object), zap.Array("TOKEN", array))
	logger.With(zap.Namespace("token")).Info("child logger", zap.String("other", "測試秘密"))
	logger.Info("parent logger", zap.String("other", "公開"))
	if strings.Contains(output.String(), "測試秘密") || !strings.Contains(output.String(), `"other":"公開"`) || strings.Count(output.String(), "[REDACTED]") != 3 {
		t.Fatal(output.String())
	}
}

func TestRedactingCoreEntrypoints(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			encoder := newEncoder(format, testEncoderConfig())
			core, err := NewRedactingCore(encoder, zapcore.AddSync(&output), zap.DebugLevel, RedactionConfig{Enabled: true, Keys: []string{"token"}})
			if err != nil {
				t.Fatal(err)
			}
			logger := zap.New(core).Named("child").With(zap.String("token", "測試秘密"))
			logger.WithLazy(zap.String("token", "測試秘密")).Info("test")
			logger.Sugar().Infow("test", "token", "測試秘密")
			checked := logger.Check(zap.InfoLevel, "test")
			if checked == nil {
				t.Fatal("entry must be accepted")
			}
			checked.Write(zap.String("token", "測試秘密"))
			if strings.Contains(output.String(), "測試秘密") {
				t.Fatal(output.String())
			}
		})
	}
}

func TestRedactingCoreContracts(t *testing.T) {
	sink := &recordingSplitSink{}
	observer := newTestObserver(t, 1)
	wrapped, err := NewObservedWriteSyncer(sink, observer)
	if err != nil {
		t.Fatal(err)
	}
	core := redactingTestCore(t, wrapped, RedactionConfig{Enabled: true, Keys: []string{"token"}})
	var mu sync.Mutex
	hooks := 0
	logger := zap.New(zapcore.RegisterHooks(core, func(zapcore.Entry) error { mu.Lock(); defer mu.Unlock(); hooks++; return nil }))
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 20 {
				logger.With(zap.String("token", "測試秘密")).Info("test")
			}
		})
	}
	wg.Wait()
	if err := logger.Sync(); err != nil {
		t.Fatal(err)
	}
	content, _, closes := sink.snapshot()
	if strings.Contains(content, "測試秘密") || strings.Count(content, "\n") != 160 || hooks != 160 || closes != 0 {
		t.Fatalf("hooks=%d closes=%d", hooks, closes)
	}
	select {
	case event := <-observer.Events():
		t.Fatal(event)
	default:
	}
}

func TestRedactingCoreLevelAndSinkErrors(t *testing.T) {
	observer := newTestObserver(t, 4)
	sentinel := errors.New("test I/O failed")
	sink, err := NewObservedWriteSyncer(failingObserverSink{writeErr: sentinel, syncErr: sentinel}, observer)
	if err != nil {
		t.Fatal(err)
	}
	level := zap.NewAtomicLevelAt(zap.ErrorLevel)
	core, err := NewRedactingCore(zapcore.NewJSONEncoder(testEncoderConfig()), sink, level, RedactionConfig{Enabled: true, Keys: []string{"token"}})
	if err != nil {
		t.Fatal(err)
	}
	if core.Check(zapcore.Entry{Level: zap.InfoLevel}, nil) != nil {
		t.Fatal("entries below the threshold must be rejected")
	}
	level.SetLevel(zap.DebugLevel)
	if core.Check(zapcore.Entry{Level: zap.InfoLevel}, nil) == nil {
		t.Fatal("entry must pass after the threshold changes")
	}
	if err := core.Write(zapcore.Entry{}, []zap.Field{{Type: zapcore.UnknownType}}); !errors.Is(err, ErrRedactionFailed) {
		t.Fatal(err)
	}
	select {
	case event := <-observer.Events():
		t.Fatal("redaction failure must not emit a sink event", event.Operation)
	default:
	}
	if err := core.Write(zapcore.Entry{}, []zap.Field{zap.String("token", "測試秘密")}); err != sentinel {
		t.Fatal(err)
	}
	if err := core.Sync(); err != sentinel {
		t.Fatal(err)
	}
	for _, operation := range []string{"write", "sync"} {
		select {
		case event := <-observer.Events():
			if event.Err != sentinel || event.Operation != operation {
				t.Fatal(event.Operation)
			}
		default:
			t.Fatal("missing sink failure event")
		}
	}
}

func TestRedactingCoreGlobalAndSplit(t *testing.T) {
	resetGlobalState(t)
	t.Cleanup(func() { resetGlobalState(t) })
	info, warn, fail := &recordingSplitSink{}, &recordingSplitSink{}, &recordingSplitSink{}
	var cores []zapcore.Core
	for _, route := range []struct {
		sink  zapcore.WriteSyncer
		level zapcore.Level
	}{{info, zap.InfoLevel}, {warn, zap.WarnLevel}, {fail, zap.ErrorLevel}} {
		level := route.level
		core, err := NewRedactingCore(zapcore.NewJSONEncoder(testEncoderConfig()), route.sink, zap.LevelEnablerFunc(func(got zapcore.Level) bool { return got == level }), RedactionConfig{Enabled: true, Keys: []string{"token"}})
		if err != nil {
			t.Fatal(err)
		}
		cores = append(cores, core)
	}
	cleanup, err := ConfigureCore(nil, zapcore.NewTee(cores...))
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithContext(t.Context(), String("token", "測試秘密"))
	InfoContext(ctx, "info")
	Warn("warn", String("token", "測試秘密"))
	Error("error", String("token", "測試秘密"))
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	for _, sink := range []*recordingSplitSink{info, warn, fail} {
		content, _, closed := sink.snapshot()
		if strings.Contains(content, "測試秘密") || strings.Count(content, "\n") != 1 || closed != 0 {
			t.Fatal(content)
		}
	}
}

func TestRedactingCoreFailureDiagnostic(t *testing.T) {
	var output, diagnostic bytes.Buffer
	core := redactingTestCore(t, zapcore.AddSync(&output), RedactionConfig{Enabled: true, Keys: []string{"token"}})
	failed := redactionObjectFunc(func(e zapcore.ObjectEncoder) error {
		e.AddString("ok", "測試秘密")
		return errors.New("marshaler secret")
	})
	zap.New(core, zap.ErrorOutput(zapcore.AddSync(&diagnostic))).With(zap.Object("data", failed)).Info("test")
	if output.Len() != 0 || strings.Contains(diagnostic.String(), "測試秘密") || strings.Contains(diagnostic.String(), "marshaler secret") || !strings.Contains(diagnostic.String(), ErrRedactionFailed.Error()) {
		t.Fatal(output.String(), diagnostic.String())
	}
}

func FuzzRedaction(f *testing.F) {
	f.Add("token", "測試秘密")
	f.Add("ToKeN", "value")
	f.Fuzz(func(t *testing.T, key, value string) {
		if len(key) == 0 || len(key) > 128 {
			return
		}
		var output bytes.Buffer
		core, err := NewRedactingCore(zapcore.NewJSONEncoder(zapcore.EncoderConfig{}), zapcore.AddSync(&output), zap.DebugLevel, RedactionConfig{Enabled: true, Keys: []string{key}})
		if err != nil {
			t.Fatal(err)
		}
		if err := core.Write(zapcore.Entry{}, []zap.Field{zap.String(key, value)}); err != nil {
			t.Fatal(err)
		}
		decoded := decodeTestJSON(t, output.String())
		// JSON normalizes invalid UTF-8 keys, so the original key cannot index the decoded object.
		for _, got := range decoded {
			if got != "[REDACTED]" {
				t.Fatalf("value was not redacted: %v", got)
			}
		}
		output.Reset()
		depth := len(value) % 40
		var nested redactionObjectFunc
		nested = func(e zapcore.ObjectEncoder) error {
			if depth == 0 {
				e.AddString(key, "結構測試秘密")
				return nil
			}
			depth--
			return e.AddArray("", redactionArrayFunc(func(a zapcore.ArrayEncoder) error { return a.AppendObject(nested) }))
		}
		err = core.Write(zapcore.Entry{}, []zap.Field{zap.Object("", nested)})
		if err != nil && !errors.Is(err, ErrRedactionFailed) {
			t.Fatal(err)
		}
		if strings.Contains(output.String(), "結構測試秘密") || (err != nil && output.Len() != 0) {
			t.Fatal("nested data was not processed safely")
		}
	})
}

func TestStructuredFieldHelpersRedaction(t *testing.T) {
	object := redactionObjectFunc(func(e zapcore.ObjectEncoder) error {
		e.AddString("token", "source secret")
		e.AddInt("count", 2)
		return nil
	})
	array := redactionArrayFunc(func(e zapcore.ArrayEncoder) error { return e.AppendObject(object) })
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "enabled"
		}
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			core := redactingTestCore(t, zapcore.AddSync(&output), RedactionConfig{Enabled: enabled, Keys: []string{"token"}})
			fields := []Field{Object("object", object), Array("array", array), Inline(object), Ints("numbers", []int{2, 1}), Bools("flags", []bool{true, false}), ByteStrings("texts", [][]byte{[]byte("text")})}
			if err := core.Write(zapcore.Entry{}, fields); err != nil {
				t.Fatal(err)
			}
			encoded := output.String()
			if strings.Contains(encoded, "source secret") == enabled {
				t.Fatalf("unexpected redaction: %s", encoded)
			}
			for _, want := range []string{"\"count\":2", "\"numbers\":[2,1]", "\"flags\":[true,false]", "\"texts\":[\"text\"]"} {
				if !strings.Contains(encoded, want) {
					t.Fatalf("public primitive changed: %s", encoded)
				}
			}
			if enabled && strings.Count(encoded, "[REDACTED]") != 3 {
				t.Fatalf("nested object, array, or inline secret was not masked: %s", encoded)
			}
		})
	}
	t.Run("matched containers are not evaluated", func(t *testing.T) {
		calls := 0
		object := redactionObjectFunc(func(zapcore.ObjectEncoder) error { calls++; return errors.New("must not evaluate object") })
		array := redactionArrayFunc(func(zapcore.ArrayEncoder) error { calls++; return errors.New("must not evaluate array") })
		var output bytes.Buffer
		core := redactingTestCore(t, zapcore.AddSync(&output), RedactionConfig{Enabled: true, Keys: []string{"token"}})
		fields := []Field{Object("token", object), Array("TOKEN", array), Namespace("token"), Object("child", object), Array("children", array)}
		if err := core.Write(zapcore.Entry{}, fields); err != nil {
			t.Fatal(err)
		}
		if calls != 0 || strings.Contains(output.String(), "child") {
			t.Fatalf("matched container or namespace was evaluated: calls=%d, output=%s", calls, output.String())
		}
	})
	t.Run("With and WithLazy isolation", func(t *testing.T) {
		var output bytes.Buffer
		core := redactingTestCore(t, zapcore.AddSync(&output), RedactionConfig{Enabled: true, Keys: []string{"token"}})
		parent := zap.New(core)
		child := parent.With(Namespace("request"), Object("profile", object))
		lazy := parent.WithLazy(Namespace("token"), Object("hidden", object))
		child.Info("child", Ints("attempts", []int{1, 2}))
		lazy.Info("lazy")
		lazy.Info("lazy again")
		parent.Info("parent", String("side", "root"))
		lines := strings.Split(strings.TrimSpace(output.String()), "\n")
		if len(lines) != 4 || strings.Contains(output.String(), "source secret") || !strings.Contains(lines[0], "\"request\":{\"profile\":") || strings.Contains(lines[3], "request") || strings.Contains(lines[3], "token") || !strings.Contains(lines[3], "\"side\":\"root\"") {
			t.Fatalf("bound fields leaked into parent or lost protection: %s", output.String())
		}
	})
	t.Run("fail closed", testStructuredFieldHelpersFailClosed)
}

func testStructuredFieldHelpersFailClosed(t *testing.T) {
	t.Helper()
	failedObject := redactionObjectFunc(func(e zapcore.ObjectEncoder) error {
		e.AddString("partial", "source secret")
		return errors.New("marshaler secret")
	})
	failedArray := redactionArrayFunc(func(e zapcore.ArrayEncoder) error {
		e.AppendString("source secret")
		return errors.New("marshaler secret")
	})
	panicObject := redactionObjectFunc(func(zapcore.ObjectEncoder) error { panic("marshaler secret") })
	panicArray := redactionArrayFunc(func(zapcore.ArrayEncoder) error { panic("marshaler secret") })
	var recursiveObject redactionObjectFunc
	recursiveObject = func(e zapcore.ObjectEncoder) error { return e.AddObject("nested", recursiveObject) }
	var recursiveArray redactionArrayFunc
	recursiveArray = func(e zapcore.ArrayEncoder) error { return e.AppendArray(recursiveArray) }
	oversized := redactionArrayFunc(func(e zapcore.ArrayEncoder) error {
		for range 1025 {
			e.AppendInt(1)
		}
		return nil
	})
	fields := []Field{Object("object", failedObject), Array("array", failedArray), Inline(failedObject), Object("panic", panicObject), Array("panic", panicArray), Object("deep", recursiveObject), Array("deep", recursiveArray), Array("budget", oversized)}
	for index, field := range fields {
		t.Run(fmt.Sprintf("case_%d", index), func(t *testing.T) {
			var output, diagnostic bytes.Buffer
			core := redactingTestCore(t, zapcore.AddSync(&output), RedactionConfig{Enabled: true, Keys: []string{"token"}})
			for _, target := range []zapcore.Core{core, core.With([]Field{field})} {
				err := target.Write(zapcore.Entry{}, []Field{field})
				if err != ErrRedactionFailed || output.Len() != 0 {
					t.Fatalf("unsafe partial entry: error=%v, output=%s", err, output.String())
				}
			}
			zap.New(core, zap.ErrorOutput(zapcore.AddSync(&diagnostic))).WithLazy(field).Info("lazy failure")
			if output.Len() != 0 || !strings.Contains(diagnostic.String(), ErrRedactionFailed.Error()) || strings.Contains(diagnostic.String(), "source secret") || strings.Contains(diagnostic.String(), "marshaler secret") {
				t.Fatalf("unsafe failure diagnostic: %s", diagnostic.String())
			}
		})
	}
}
