package zlogger

import (
	"time"

	"go.uber.org/zap"
)

// Field 輔助函數 - 創建各種類型的日誌字段

// String 創建字符串類型的字段
func String(key, value string) Field {
	return zap.String(key, value)
}

// Strings 創建字符串切片類型的字段
func Strings(key string, value []string) Field {
	return zap.Strings(key, value)
}

// Int 創建整數類型的字段
func Int(key string, value int) Field {
	return zap.Int(key, value)
}

// Int8 創建 int8 類型的字段
func Int8(key string, value int8) Field {
	return zap.Int8(key, value)
}

// Int16 創建 int16 類型的字段
func Int16(key string, value int16) Field {
	return zap.Int16(key, value)
}

// Int32 創建 int32 類型的字段
func Int32(key string, value int32) Field {
	return zap.Int32(key, value)
}

// Int64 創建 int64 類型的字段
func Int64(key string, value int64) Field {
	return zap.Int64(key, value)
}

// Uint 創建 uint 類型的字段
func Uint(key string, value uint) Field {
	return zap.Uint(key, value)
}

// Uint8 創建 uint8 類型的字段
func Uint8(key string, value uint8) Field {
	return zap.Uint8(key, value)
}

// Uint16 創建 uint16 類型的字段
func Uint16(key string, value uint16) Field {
	return zap.Uint16(key, value)
}

// Uint32 創建 uint32 類型的字段
func Uint32(key string, value uint32) Field {
	return zap.Uint32(key, value)
}

// Uint64 創建 uint64 類型的字段
func Uint64(key string, value uint64) Field {
	return zap.Uint64(key, value)
}

// Float32 創建 float32 類型的字段
func Float32(key string, value float32) Field {
	return zap.Float32(key, value)
}

// Float64 創建浮點類型的字段
func Float64(key string, value float64) Field {
	return zap.Float64(key, value)
}

// Bool 創建布爾類型的字段
func Bool(key string, value bool) Field {
	return zap.Bool(key, value)
}

// Err 創建錯誤類型的字段
func Err(err error) Field {
	return zap.Error(err)
}

// NamedError 創建命名錯誤類型的字段
func NamedError(key string, err error) Field {
	return zap.NamedError(key, err)
}

// Any 創建任意類型的字段
func Any(key string, value interface{}) Field {
	return zap.Any(key, value)
}

// Duration 創建時間間隔類型的字段
func Duration(key string, value time.Duration) Field {
	return zap.Duration(key, value)
}

// Time 創建時間類型的字段
func Time(key string, value time.Time) Field {
	return zap.Time(key, value)
}

// Binary 創建二進制數據類型的字段
func Binary(key string, value []byte) Field {
	return zap.Binary(key, value)
}

// ByteString 創建字節字符串類型的字段
func ByteString(key string, value []byte) Field {
	return zap.ByteString(key, value)
}

// Stringer 創建 Stringer 類型的字段
func Stringer(key string, value interface{ String() string }) Field {
	return zap.Stringer(key, value)
}

// Reflect 創建反射類型的字段（性能較低，建議使用 Any）
func Reflect(key string, value interface{}) Field {
	return zap.Reflect(key, value)
}

// Stack 創建堆疊追蹤字段
func Stack(key string) Field {
	return zap.Stack(key)
}

// StackSkip 創建跳過指定層數的堆疊追蹤字段
func StackSkip(key string, skip int) Field {
	return zap.StackSkip(key, skip)
}

