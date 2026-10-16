package zlogger

import (
	"bytes"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestRedactingCorePrimitiveTypes(t *testing.T) {
	var output bytes.Buffer
	core := redactingTestCore(t, zapcore.AddSync(&output), RedactionConfig{Enabled: true, Keys: []string{"mask"}})
	object := redactionObjectFunc(func(e zapcore.ObjectEncoder) error {
		e.AddBool("safeBool", true)
		e.AddBool("mask", true)
		e.AddComplex128("safeComplex128", complex(1, 2))
		e.AddComplex128("mask", complex(1, 2))
		e.AddComplex64("safeComplex64", complex64(complex(1, 2)))
		e.AddComplex64("mask", complex64(complex(1, 2)))
		e.AddDuration("safeDuration", time.Second)
		e.AddDuration("mask", time.Second)
		e.AddFloat64("safeFloat64", 1.25)
		e.AddFloat64("mask", 1.25)
		e.AddFloat32("safeFloat32", float32(1.25))
		e.AddFloat32("mask", float32(1.25))
		e.AddInt("safeInt", 1)
		e.AddInt("mask", 1)
		e.AddInt64("safeInt64", int64(1))
		e.AddInt64("mask", int64(1))
		e.AddInt32("safeInt32", int32(1))
		e.AddInt32("mask", int32(1))
		e.AddInt16("safeInt16", int16(1))
		e.AddInt16("mask", int16(1))
		e.AddInt8("safeInt8", int8(1))
		e.AddInt8("mask", int8(1))
		e.AddString("safeString", "公開")
		e.AddString("mask", "公開")
		e.AddTime("safeTime", time.Unix(1, 0).UTC())
		e.AddTime("mask", time.Unix(1, 0).UTC())
		e.AddUint("safeUint", uint(1))
		e.AddUint("mask", uint(1))
		e.AddUint64("safeUint64", uint64(1))
		e.AddUint64("mask", uint64(1))
		e.AddUint32("safeUint32", uint32(1))
		e.AddUint32("mask", uint32(1))
		e.AddUint16("safeUint16", uint16(1))
		e.AddUint16("mask", uint16(1))
		e.AddUint8("safeUint8", uint8(1))
		e.AddUint8("mask", uint8(1))
		e.AddUintptr("safeUintptr", uintptr(1))
		e.AddUintptr("mask", uintptr(1))
		e.AddByteString("safeByteString", []byte("公開"))
		e.AddByteString("mask", []byte("公開"))
		e.AddBinary("safeBinary", []byte("公開"))
		e.AddBinary("mask", []byte("公開"))
		return e.AddReflected("opaque", forbiddenRedactionValue{})
	})
	array := redactionArrayFunc(func(e zapcore.ArrayEncoder) error {
		e.AppendBool(true)
		e.AppendComplex128(complex(1, 2))
		e.AppendComplex64(complex64(complex(1, 2)))
		e.AppendDuration(time.Second)
		e.AppendFloat64(1.25)
		e.AppendFloat32(float32(1.25))
		e.AppendInt(1)
		e.AppendInt64(int64(1))
		e.AppendInt32(int32(1))
		e.AppendInt16(int16(1))
		e.AppendInt8(int8(1))
		e.AppendString("公開")
		e.AppendTime(time.Unix(1, 0).UTC())
		e.AppendUint(uint(1))
		e.AppendUint64(uint64(1))
		e.AppendUint32(uint32(1))
		e.AppendUint16(uint16(1))
		e.AppendUint8(uint8(1))
		e.AppendUintptr(uintptr(1))
		e.AppendByteString([]byte("公開"))
		if err := e.AppendReflected(forbiddenRedactionValue{}); err != nil {
			return err
		}
		if err := e.AppendArray(redactionArrayFunc(func(n zapcore.ArrayEncoder) error { n.AppendString("nested"); return nil })); err != nil {
			return err
		}
		return e.AppendObject(object)
	})
	zap.New(core).Info("測試", zap.Object("object", object), zap.Array("array", array))
	if !bytes.Contains(output.Bytes(), []byte("safeBool")) || !bytes.Contains(output.Bytes(), []byte("[REDACTED]")) {
		t.Fatal(output.String())
	}
}
