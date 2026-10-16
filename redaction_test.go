package zlogger

import (
	"bytes"
	"errors"
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
		t.Run(map[bool]string{false: "關閉", true: "開啟"}[enabled], func(t *testing.T) {
			var output bytes.Buffer
			cfg := RedactionConfig{Enabled: enabled, Keys: []string{"token"}}
			core := redactingTestCore(t, zapcore.AddSync(&output), cfg)
			cfg.Keys[0] = "other"
			zap.New(core).Info("測試", zap.String("token", "測試秘密"), Redacted("manual"))
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
		t.Fatal("停用應忽略 Keys", err)
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

func (forbiddenRedactionValue) Error() string                { panic("不應求值 error") }
func (forbiddenRedactionValue) String() string               { panic("不應求值 Stringer") }
func (forbiddenRedactionValue) MarshalJSON() ([]byte, error) { panic("不應反射編碼") }

func TestRedactingCoreOpaque(t *testing.T) {
	var output bytes.Buffer
	core := redactingTestCore(t, zapcore.AddSync(&output), RedactionConfig{Enabled: true, Keys: []string{"token"}})
	logger := zap.New(core).With(zap.Error(forbiddenRedactionValue{}), zap.Stringer("detail", forbiddenRedactionValue{}))
	logger.Info("測試", zap.Reflect("object", forbiddenRedactionValue{}))
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
	zap.New(core).With(zap.Namespace("space")).Info("測試", zap.Object("child", object), zap.Array("list", array), zap.Inline(object))
	zap.New(core).With(zap.Namespace("private")).Info("測試", zap.String("anything", "測試秘密"))
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
		return errors.New("測試秘密")
	})
	var recurse redactionObjectFunc
	recurse = func(e zapcore.ObjectEncoder) error { return e.AddObject("nested", recurse) }
	oversized := redactionArrayFunc(func(e zapcore.ArrayEncoder) error {
		for range 1025 {
			e.AppendInt(1)
		}
		return nil
	})
	failedArray := redactionArrayFunc(func(zapcore.ArrayEncoder) error { return errors.New("測試秘密") })
	var recursiveArray redactionArrayFunc
	recursiveArray = func(e zapcore.ArrayEncoder) error { return e.AppendArray(recursiveArray) }
	panicObject := redactionObjectFunc(func(zapcore.ObjectEncoder) error { panic("測試秘密") })
	for _, field := range []zap.Field{zap.Object("panic", panicObject), zap.Array("failure", failedArray), zap.Array("deep", recursiveArray), zap.Object("object", failed), zap.Inline(failed), zap.Object("deep", recurse), zap.Array("huge", oversized), {Type: zapcore.UnknownType}, {Type: 255}} {
		var output bytes.Buffer
		core := redactingTestCore(t, zapcore.AddSync(&output), RedactionConfig{Enabled: true, Keys: []string{"token"}})
		err := core.Write(zapcore.Entry{}, []zap.Field{field})
		if !errors.Is(err, ErrRedactionFailed) || output.Len() != 0 || strings.Contains(err.Error(), "測試秘密") {
			t.Fatalf("錯誤=%v，輸出=%s", err, output.String())
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
	object := redactionObjectFunc(func(zapcore.ObjectEncoder) error { panic("不得求值") })
	array := redactionArrayFunc(func(zapcore.ArrayEncoder) error { panic("不得求值") })
	logger := zap.New(core)
	logger.Info("容器", zap.Object("token", object), zap.Array("TOKEN", array))
	logger.With(zap.Namespace("token")).Info("子 logger", zap.String("other", "測試秘密"))
	logger.Info("父 logger", zap.String("other", "公開"))
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
			logger.WithLazy(zap.String("token", "測試秘密")).Info("測試")
			logger.Sugar().Infow("測試", "token", "測試秘密")
			checked := logger.Check(zap.InfoLevel, "測試")
			if checked == nil {
				t.Fatal("應接受記錄")
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
				logger.With(zap.String("token", "測試秘密")).Info("測試")
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
	sentinel := errors.New("測試 I/O 失敗")
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
		t.Fatal("低於門檻的日誌不可通過")
	}
	level.SetLevel(zap.DebugLevel)
	if core.Check(zapcore.Entry{Level: zap.InfoLevel}, nil) == nil {
		t.Fatal("動態調整門檻後應通過")
	}
	if err := core.Write(zapcore.Entry{}, []zap.Field{{Type: zapcore.UnknownType}}); !errors.Is(err, ErrRedactionFailed) {
		t.Fatal(err)
	}
	select {
	case event := <-observer.Events():
		t.Fatal("遮罩失敗不可產生 sink 事件", event.Operation)
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
			t.Fatal("缺少 sink 故障事件")
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
		return errors.New("測試秘密")
	})
	zap.New(core, zap.ErrorOutput(zapcore.AddSync(&diagnostic))).With(zap.Object("data", failed)).Info("測試")
	if output.Len() != 0 || strings.Contains(diagnostic.String(), "測試秘密") || !strings.Contains(diagnostic.String(), ErrRedactionFailed.Error()) {
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
		// 無效 UTF-8 key 由 JSON 正規化，不能拿原 key 當解碼索引。
		for _, got := range decoded {
			if got != "[REDACTED]" {
				t.Fatalf("未遮罩：%v", got)
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
			t.Fatal("巢狀資料未安全處理")
		}
	})
}
