package zlogger

import (
	"time"

	"go.uber.org/zap"
)

// Field helper functions - create various types of log fields

// String creates a string type field
func String(key, value string) Field {
	return zap.String(key, value)
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
