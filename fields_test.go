package zlogger

import (
	"errors"
	"testing"
	"time"
)

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
