package zlogger

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestStructuredFieldHelpersEncoding(t *testing.T) {
	for _, pair := range [][2]reflect.Type{
		{reflect.TypeFor[ObjectMarshaler](), reflect.TypeFor[zapcore.ObjectMarshaler]()},
		{reflect.TypeFor[ArrayMarshaler](), reflect.TypeFor[zapcore.ArrayMarshaler]()},
		{reflect.TypeFor[ObjectEncoder](), reflect.TypeFor[zapcore.ObjectEncoder]()},
		{reflect.TypeFor[ArrayEncoder](), reflect.TypeFor[zapcore.ArrayEncoder]()},
	} {
		if pair[0] != pair[1] {
			t.Fatalf("expected a true type alias: %v != %v", pair[0], pair[1])
		}
	}
	for _, format := range []string{"json", "console"} {
		t.Run(format, func(t *testing.T) {
			calls := 0
			array := redactionArrayFunc(func(e zapcore.ArrayEncoder) error {
				calls++
				e.AppendInt(2)
				e.AppendInt(1)
				return nil
			})
			object := redactionObjectFunc(func(e zapcore.ObjectEncoder) error {
				calls++
				e.AddString("name", "reader")
				return e.AddArray("codes", array)
			})
			fields := []Field{Object("object", object), Array("array", array), Inline(object)}
			if calls != 0 {
				t.Fatal("field construction evaluated a marshaler")
			}
			actual := encodeStructuredFields(t, format, fields...)
			if calls != 5 {
				t.Fatalf("marshaler calls = %d, want 5", calls)
			}
			calls = 0
			want := encodeStructuredFields(t, format, zap.Object("object", object), zap.Array("array", array), zap.Inline(object))
			if actual != want || calls != 5 {
				t.Fatalf("helper output differs from zap: %q != %q, calls=%d", actual, want, calls)
			}
			if format == "json" && actual != "{\"object\":{\"name\":\"reader\",\"codes\":[2,1]},\"array\":[2,1],\"name\":\"reader\",\"codes\":[2,1]}\n" {
				t.Fatalf("unexpected object hierarchy or order: %s", actual)
			}
		})
	}
}

func encodeStructuredFields(t *testing.T, format string, fields ...Field) string {
	t.Helper()
	encoder := newEncoder(format, zapcore.EncoderConfig{
		EncodeTime:     zapcore.RFC3339TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	})
	return encodeTestEntry(t, encoder, zapcore.Entry{}, fields...)
}

func TestStructuredFieldHelpersNamespace(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		t.Run(format, func(t *testing.T) {
			var actual, expected bytes.Buffer
			build := func(output *bytes.Buffer) *zap.Logger {
				return zap.New(zapcore.NewCore(newEncoder(format, zapcore.EncoderConfig{}), zapcore.AddSync(output), zap.DebugLevel))
			}
			parent, reference := build(&actual), build(&expected)
			child := parent.With(String("duplicate", "one"), Inline(redactionObjectFunc(func(e zapcore.ObjectEncoder) error {
				e.AddString("duplicate", "two")
				return nil
			})), Namespace("request"), String("id", "bound"))
			upstream := reference.With(zap.String("duplicate", "one"), zap.Inline(redactionObjectFunc(func(e zapcore.ObjectEncoder) error {
				e.AddString("duplicate", "two")
				return nil
			})), zap.Namespace("request"), zap.String("id", "bound"))
			for range 2 {
				child.Info("", String("call", "child"))
				upstream.Info("", zap.String("call", "child"))
			}
			parent.Info("", String("call", "parent"))
			reference.Info("", zap.String("call", "parent"))
			if actual.String() != expected.String() {
				t.Fatalf("namespace scope differs: %q != %q", actual.String(), expected.String())
			}
			if format == "json" {
				lines := strings.Split(strings.TrimSpace(actual.String()), "\n")
				if len(lines) != 3 || strings.Count(lines[0], "\"duplicate\"") != 2 || !strings.Contains(lines[0], "\"request\":{\"id\":\"bound\",\"call\":\"child\"}") || lines[2] != "{\"call\":\"parent\"}" {
					t.Fatalf("duplicate keys or parent isolation changed: %s", actual.String())
				}
			}
		})
	}
}

func TestStringField(t *testing.T) {
	field := String("key", "value")
	if field.Key != "key" {
		t.Errorf("expected key 'key', got %s", field.Key)
	}
}

func TestStringsField(t *testing.T) {
	field := Strings("keys", []string{"a", "b", "c"})
	if field.Key != "keys" {
		t.Errorf("expected key 'keys', got %s", field.Key)
	}
}

func TestIntFields(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"Int", "int_key"},
		{"Int8", "int8_key"},
		{"Int16", "int16_key"},
		{"Int32", "int32_key"},
		{"Int64", "int64_key"},
	}

	// Test Int
	field := Int("int_key", 42)
	if field.Key != "int_key" {
		t.Errorf("Int: expected key 'int_key', got %s", field.Key)
	}

	// Test Int8
	field8 := Int8("int8_key", 8)
	if field8.Key != "int8_key" {
		t.Errorf("Int8: expected key 'int8_key', got %s", field8.Key)
	}

	// Test Int16
	field16 := Int16("int16_key", 16)
	if field16.Key != "int16_key" {
		t.Errorf("Int16: expected key 'int16_key', got %s", field16.Key)
	}

	// Test Int32
	field32 := Int32("int32_key", 32)
	if field32.Key != "int32_key" {
		t.Errorf("Int32: expected key 'int32_key', got %s", field32.Key)
	}

	// Test Int64
	field64 := Int64("int64_key", 64)
	if field64.Key != "int64_key" {
		t.Errorf("Int64: expected key 'int64_key', got %s", field64.Key)
	}

	// Avoid unused warning
	_ = tests
}

func TestUintFields(t *testing.T) {
	// Test Uint
	field := Uint("uint_key", 42)
	if field.Key != "uint_key" {
		t.Errorf("Uint: expected key 'uint_key', got %s", field.Key)
	}

	// Test Uint8
	field8 := Uint8("uint8_key", 8)
	if field8.Key != "uint8_key" {
		t.Errorf("Uint8: expected key 'uint8_key', got %s", field8.Key)
	}

	// Test Uint16
	field16 := Uint16("uint16_key", 16)
	if field16.Key != "uint16_key" {
		t.Errorf("Uint16: expected key 'uint16_key', got %s", field16.Key)
	}

	// Test Uint32
	field32 := Uint32("uint32_key", 32)
	if field32.Key != "uint32_key" {
		t.Errorf("Uint32: expected key 'uint32_key', got %s", field32.Key)
	}

	// Test Uint64
	field64 := Uint64("uint64_key", 64)
	if field64.Key != "uint64_key" {
		t.Errorf("Uint64: expected key 'uint64_key', got %s", field64.Key)
	}
}

func TestFloatFields(t *testing.T) {
	// Test Float32
	field32 := Float32("float32_key", 3.14)
	if field32.Key != "float32_key" {
		t.Errorf("Float32: expected key 'float32_key', got %s", field32.Key)
	}

	// Test Float64
	field64 := Float64("float64_key", 3.14159)
	if field64.Key != "float64_key" {
		t.Errorf("Float64: expected key 'float64_key', got %s", field64.Key)
	}
}

func TestBoolField(t *testing.T) {
	field := Bool("bool_key", true)
	if field.Key != "bool_key" {
		t.Errorf("expected key 'bool_key', got %s", field.Key)
	}
}

func TestErrField(t *testing.T) {
	err := errors.New("test error")
	field := Err(err)
	if field.Key != "error" {
		t.Errorf("expected key 'error', got %s", field.Key)
	}
}

func TestNamedErrorField(t *testing.T) {
	err := errors.New("test error")
	field := NamedError("custom_error", err)
	if field.Key != "custom_error" {
		t.Errorf("expected key 'custom_error', got %s", field.Key)
	}
}

func TestAnyField(t *testing.T) {
	type customStruct struct {
		Name string
		Age  int
	}
	data := customStruct{Name: "test", Age: 30}
	field := Any("data", data)
	if field.Key != "data" {
		t.Errorf("expected key 'data', got %s", field.Key)
	}
}

func TestDurationField(t *testing.T) {
	field := Duration("duration", 5*time.Second)
	if field.Key != "duration" {
		t.Errorf("expected key 'duration', got %s", field.Key)
	}
}

func TestTimeField(t *testing.T) {
	now := time.Now()
	field := Time("timestamp", now)
	if field.Key != "timestamp" {
		t.Errorf("expected key 'timestamp', got %s", field.Key)
	}
}

func TestBinaryField(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	field := Binary("binary", data)
	if field.Key != "binary" {
		t.Errorf("expected key 'binary', got %s", field.Key)
	}
}

func TestByteStringField(t *testing.T) {
	data := []byte("hello")
	field := ByteString("bytes", data)
	if field.Key != "bytes" {
		t.Errorf("expected key 'bytes', got %s", field.Key)
	}
}

// TestStringerField tests Stringer type
type testStringer struct {
	value string
}

func (s testStringer) String() string {
	return s.value
}

func TestStringerField(t *testing.T) {
	s := testStringer{value: "test_value"}
	field := Stringer("stringer", s)
	if field.Key != "stringer" {
		t.Errorf("expected key 'stringer', got %s", field.Key)
	}
}

func TestReflectField(t *testing.T) {
	data := map[string]int{"a": 1, "b": 2}
	field := Reflect("reflect", data)
	if field.Key != "reflect" {
		t.Errorf("expected key 'reflect', got %s", field.Key)
	}
}

func TestStackField(t *testing.T) {
	field := Stack("stack")
	if field.Key != "stack" {
		t.Errorf("expected key 'stack', got %s", field.Key)
	}
}

func TestStackSkipField(t *testing.T) {
	field := StackSkip("stack_skip", 1)
	if field.Key != "stack_skip" {
		t.Errorf("expected key 'stack_skip', got %s", field.Key)
	}
}

func TestRedactedField(t *testing.T) {
	var output bytes.Buffer
	encoderConfig := zapcore.EncoderConfig{
		MessageKey: "message",
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&output),
		zapcore.DebugLevel,
	)
	logger := zap.New(core)

	logger.Info("sensitive field test", Redacted("authorization"))
	encoded := output.String()
	if !strings.Contains(encoded, `"authorization":"[REDACTED]"`) {
		t.Fatalf("output does not contain the redacted field: %s", encoded)
	}
}

type sliceHelperTest struct {
	name string
	run  func(*testing.T)
}

func sliceHelperCase[T any](name string, helper, upstream func(string, []T) Field, values []T, expected string) sliceHelperTest {
	return sliceHelperTest{name: name, run: func(t *testing.T) {
		for _, state := range []struct {
			name   string
			values []T
			want   string
		}{
			{"nil", nil, "[]"},
			{"empty", []T{}, "[]"},
			{"multiple", values, expected},
		} {
			t.Run(state.name, func(t *testing.T) {
				before, err := json.Marshal(state.values)
				if err != nil {
					t.Fatal(err)
				}
				for _, format := range []string{"json", "console"} {
					actual := encodeStructuredFields(t, format, helper("values", state.values))
					want := encodeStructuredFields(t, format, upstream("values", state.values))
					if actual != want {
						t.Fatalf("%s output differs from zap: %q != %q", format, actual, want)
					}
					if format == "json" && actual != "{\"values\":"+state.want+"}\n" {
						t.Fatalf("unexpected array: %s", actual)
					}
				}
				after, err := json.Marshal(state.values)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(before, after) {
					t.Fatalf("input changed: %s -> %s", before, after)
				}
			})
		}
	}}
}

func TestSliceFieldHelpersEncoding(t *testing.T) {
	cases := []sliceHelperTest{
		sliceHelperCase("Bools", Bools, zap.Bools, []bool{true, false}, "[true,false]"),
		sliceHelperCase("Ints", Ints, zap.Ints, []int{-2, 1}, "[-2,1]"),
		sliceHelperCase("Int8s", Int8s, zap.Int8s, []int8{-2, 1}, "[-2,1]"),
		sliceHelperCase("Int16s", Int16s, zap.Int16s, []int16{-2, 1}, "[-2,1]"),
		sliceHelperCase("Int32s", Int32s, zap.Int32s, []int32{-2, 1}, "[-2,1]"),
		sliceHelperCase("Int64s", Int64s, zap.Int64s, []int64{-2, 1}, "[-2,1]"),
		sliceHelperCase("Uints", Uints, zap.Uints, []uint{2, 1}, "[2,1]"),
		sliceHelperCase("Uint8s", Uint8s, zap.Uint8s, []uint8{2, 1}, "[2,1]"),
		sliceHelperCase("Uint16s", Uint16s, zap.Uint16s, []uint16{2, 1}, "[2,1]"),
		sliceHelperCase("Uint32s", Uint32s, zap.Uint32s, []uint32{2, 1}, "[2,1]"),
		sliceHelperCase("Uint64s", Uint64s, zap.Uint64s, []uint64{2, 1}, "[2,1]"),
		sliceHelperCase("Float32s", Float32s, zap.Float32s, []float32{1.5, -2.25}, "[1.5,-2.25]"),
		sliceHelperCase("Float64s", Float64s, zap.Float64s, []float64{1.5, -2.25}, "[1.5,-2.25]"),
		sliceHelperCase("Durations", Durations, zap.Durations, []time.Duration{time.Second, 1500 * time.Millisecond}, "[\"1s\",\"1.5s\"]"),
		sliceHelperCase("Times", Times, zap.Times, []time.Time{time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 16, 8, 0, 0, 0, time.FixedZone("local", 8*60*60))}, "[\"2026-10-16T00:00:00Z\",\"2026-10-16T08:00:00+08:00\"]"),
		sliceHelperCase("ByteStrings", ByteStrings, zap.ByteStrings, [][]byte{[]byte("alpha"), nil, {}}, "[\"alpha\",\"\",\"\"]"),
	}
	if len(cases) != 16 {
		t.Fatalf("slice helper count = %d, want 16", len(cases))
	}
	for _, test := range cases {
		t.Run(test.name, test.run)
	}
	t.Run("encoder policy", func(t *testing.T) {
		encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{EncodeTime: zapcore.EpochTimeEncoder, EncodeDuration: zapcore.SecondsDurationEncoder})
		actual := encodeTestEntry(t, encoder, zapcore.Entry{}, Times("times", []time.Time{time.Unix(1, 0).UTC()}), Durations("durations", []time.Duration{1500 * time.Millisecond}), Uint8s("numbers", []uint8{65, 66}), ByteStrings("text", [][]byte{[]byte("AB")}))
		if actual != "{\"times\":[1],\"durations\":[1.5],\"numbers\":[65,66],\"text\":[\"AB\"]}\n" {
			t.Fatalf("encoder policy or byte representation changed: %s", actual)
		}
	})
}
