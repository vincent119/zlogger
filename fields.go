package zlogger

import (
	"time"

	"go.uber.org/zap"
)

// Field helper functions - create various types of log fields

// Object encodes a nested object when the core evaluates the field. It does not copy value.
func Object(key string, value ObjectMarshaler) Field {
	return zap.Object(key, value)
}

// Array preserves marshaler element order and leaves evaluation and ownership to the core and caller.
func Array(key string, value ArrayMarshaler) Field {
	return zap.Array(key, value)
}

// Inline expands an object into the current scope without deduplicating keys.
func Inline(value ObjectMarshaler) Field {
	return zap.Inline(value)
}

// Namespace places subsequent fields in a nested object until the encoder scope ends.
func Namespace(key string) Field {
	return zap.Namespace(key)
}

// String creates a string type field
func String(key, value string) Field {
	return zap.String(key, value)
}

// Redacted creates a string field with a fixed replacement without accepting the original secret.
func Redacted(key string) Field {
	return zap.String(key, "[REDACTED]")
}

// Strings creates a string slice type field
func Strings(key string, value []string) Field {
	return zap.Strings(key, value)
}

// Int creates an integer type field
func Int(key string, value int) Field {
	return zap.Int(key, value)
}

// Int8 creates an int8 type field
func Int8(key string, value int8) Field {
	return zap.Int8(key, value)
}

// Int16 creates an int16 type field
func Int16(key string, value int16) Field {
	return zap.Int16(key, value)
}

// Int32 creates an int32 type field
func Int32(key string, value int32) Field {
	return zap.Int32(key, value)
}

// Int64 creates an int64 type field
func Int64(key string, value int64) Field {
	return zap.Int64(key, value)
}

// Uint creates a uint type field
func Uint(key string, value uint) Field {
	return zap.Uint(key, value)
}

// Uint8 creates a uint8 type field
func Uint8(key string, value uint8) Field {
	return zap.Uint8(key, value)
}

// Uint16 creates a uint16 type field
func Uint16(key string, value uint16) Field {
	return zap.Uint16(key, value)
}

// Uint32 creates a uint32 type field
func Uint32(key string, value uint32) Field {
	return zap.Uint32(key, value)
}

// Uint64 creates a uint64 type field
func Uint64(key string, value uint64) Field {
	return zap.Uint64(key, value)
}

// Float32 creates a float32 type field
func Float32(key string, value float32) Field {
	return zap.Float32(key, value)
}

// Float64 creates a float64 type field
func Float64(key string, value float64) Field {
	return zap.Float64(key, value)
}

// Bool creates a boolean type field
func Bool(key string, value bool) Field {
	return zap.Bool(key, value)
}

// Err creates an error type field
func Err(err error) Field {
	return zap.Error(err)
}

// NamedError creates a named error type field
func NamedError(key string, err error) Field {
	return zap.NamedError(key, err)
}

// Any creates a field of any type
func Any(key string, value interface{}) Field {
	return zap.Any(key, value)
}

// Duration creates a duration type field
func Duration(key string, value time.Duration) Field {
	return zap.Duration(key, value)
}

// Time creates a time type field
func Time(key string, value time.Time) Field {
	return zap.Time(key, value)
}

// Binary creates a binary data type field
func Binary(key string, value []byte) Field {
	return zap.Binary(key, value)
}

// ByteString creates a byte string type field
func ByteString(key string, value []byte) Field {
	return zap.ByteString(key, value)
}

// Stringer creates a Stringer type field
func Stringer(key string, value interface{ String() string }) Field {
	return zap.Stringer(key, value)
}

// Reflect creates a reflect type field (lower performance, recommend using Any)
func Reflect(key string, value interface{}) Field {
	return zap.Reflect(key, value)
}

// Stack creates a stack trace field
func Stack(key string) Field {
	return zap.Stack(key)
}

// StackSkip creates a stack trace field that skips specified frames
func StackSkip(key string, skip int) Field {
	return zap.StackSkip(key, skip)
}

// Bools retains zap's nil-slice behavior and does not copy values.
func Bools(key string, values []bool) Field {
	return zap.Bools(key, values)
}

// Ints retains zap's nil-slice behavior and does not copy values.
func Ints(key string, values []int) Field {
	return zap.Ints(key, values)
}

// Int8s retains zap's nil-slice behavior and does not copy values.
func Int8s(key string, values []int8) Field {
	return zap.Int8s(key, values)
}

// Int16s retains zap's nil-slice behavior and does not copy values.
func Int16s(key string, values []int16) Field {
	return zap.Int16s(key, values)
}

// Int32s retains zap's nil-slice behavior and does not copy values.
func Int32s(key string, values []int32) Field {
	return zap.Int32s(key, values)
}

// Int64s retains zap's nil-slice behavior and does not copy values.
func Int64s(key string, values []int64) Field {
	return zap.Int64s(key, values)
}

// Uints retains zap's nil-slice behavior and does not copy values.
func Uints(key string, values []uint) Field {
	return zap.Uints(key, values)
}

// Uint8s encodes numeric byte values rather than text or Base64. It does not copy values.
func Uint8s(key string, values []uint8) Field {
	return zap.Uint8s(key, values)
}

// Uint16s retains zap's nil-slice behavior and does not copy values.
func Uint16s(key string, values []uint16) Field {
	return zap.Uint16s(key, values)
}

// Uint32s retains zap's nil-slice behavior and does not copy values.
func Uint32s(key string, values []uint32) Field {
	return zap.Uint32s(key, values)
}

// Uint64s retains zap's nil-slice behavior and does not copy values.
func Uint64s(key string, values []uint64) Field {
	return zap.Uint64s(key, values)
}

// Float32s retains zap's nil-slice behavior and does not copy values.
func Float32s(key string, values []float32) Field {
	return zap.Float32s(key, values)
}

// Float64s retains zap's nil-slice behavior and does not copy values.
func Float64s(key string, values []float64) Field {
	return zap.Float64s(key, values)
}

// Durations uses the encoder's duration format and does not copy values.
func Durations(key string, values []time.Duration) Field {
	return zap.Durations(key, values)
}

// Times uses the encoder's time format and does not copy values.
func Times(key string, values []time.Time) Field {
	return zap.Times(key, values)
}

// ByteStrings encodes UTF-8 text elements, not Base64. It does not copy the slices.
func ByteStrings(key string, values [][]byte) Field {
	return zap.ByteStrings(key, values)
}
