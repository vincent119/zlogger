package zlogger

import (
	"slices"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func (r *redactionObject) AddBool(key string, value bool) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Bool(key, value))
	}
}

func (r *redactionObject) AddComplex128(key string, value complex128) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Complex128(key, value))
	}
}

func (r *redactionObject) AddComplex64(key string, value complex64) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Complex64(key, value))
	}
}

func (r *redactionObject) AddDuration(key string, value time.Duration) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Duration(key, value))
	}
}

func (r *redactionObject) AddFloat64(key string, value float64) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Float64(key, value))
	}
}

func (r *redactionObject) AddFloat32(key string, value float32) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Float32(key, value))
	}
}

func (r *redactionObject) AddInt(key string, value int) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Int(key, value))
	}
}

func (r *redactionObject) AddInt64(key string, value int64) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Int64(key, value))
	}
}

func (r *redactionObject) AddInt32(key string, value int32) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Int32(key, value))
	}
}

func (r *redactionObject) AddInt16(key string, value int16) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Int16(key, value))
	}
}

func (r *redactionObject) AddInt8(key string, value int8) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Int8(key, value))
	}
}

func (r *redactionObject) AddString(key string, value string) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.String(key, value))
	}
}

func (r *redactionObject) AddTime(key string, value time.Time) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Time(key, value))
	}
}

func (r *redactionObject) AddUint(key string, value uint) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Uint(key, value))
	}
}

func (r *redactionObject) AddUint64(key string, value uint64) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Uint64(key, value))
	}
}

func (r *redactionObject) AddUint32(key string, value uint32) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Uint32(key, value))
	}
}

func (r *redactionObject) AddUint16(key string, value uint16) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Uint16(key, value))
	}
}

func (r *redactionObject) AddUint8(key string, value uint8) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Uint8(key, value))
	}
}

func (r *redactionObject) AddUintptr(key string, value uintptr) {
	if r.allow(key) {

		r.fields = append(r.fields, zap.Uintptr(key, value))
	}
}

func (r *redactionObject) AddByteString(key string, value []byte) {
	if r.allow(key) {
		value = slices.Clone(value)
		r.fields = append(r.fields, zap.ByteString(key, value))
	}
}

func (r *redactionObject) AddBinary(key string, value []byte) {
	if r.allow(key) {
		value = slices.Clone(value)
		r.fields = append(r.fields, zap.Binary(key, value))
	}
}

func (r *redactionObject) AddReflected(key string, _ any) error {
	r.AddString(key, "[REDACTED]")
	return r.state.err
}

func (r *redactionObject) OpenNamespace(key string) {
	if r.allow(key) {
		r.depth++
		if !r.state.take(r.depth) {
			return
		}
		r.fields = append(r.fields, zap.Namespace(key))
	} else if r.state.err == nil {
		r.suppressed = true
	}
}

type redactedObject []zapcore.Field

func (o redactedObject) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	for _, field := range o {
		field.AddTo(encoder)
	}
	return nil
}

func (r *redactionObject) AddObject(key string, value zapcore.ObjectMarshaler) error {
	if !r.allow(key) {
		return r.state.err
	}
	child := redactionObject{keys: r.keys, state: r.state, depth: r.depth + 1}
	if !r.state.take(child.depth) {
		return r.state.err
	}
	if err := value.MarshalLogObject(&child); err != nil {
		r.state.err = ErrRedactionFailed
	}
	if r.state.err != nil {
		return r.state.err
	}
	r.fields = append(r.fields, zap.Object(key, redactedObject(child.fields)))
	return nil
}

func (r *redactionObject) AddArray(key string, value zapcore.ArrayMarshaler) error {
	if !r.allow(key) {
		return r.state.err
	}
	child := redactionArray{keys: r.keys, state: r.state, depth: r.depth + 1}
	if !r.state.take(child.depth) {
		return r.state.err
	}
	if err := value.MarshalLogArray(&child); err != nil {
		r.state.err = ErrRedactionFailed
	}
	if r.state.err != nil {
		return r.state.err
	}
	r.fields = append(r.fields, zap.Array(key, redactedArray(child.values)))
	return nil
}

type redactionArray struct {
	keys   map[string]struct{}
	state  *redactionState
	depth  int
	values []func(zapcore.ArrayEncoder) error
}

type redactedArray []func(zapcore.ArrayEncoder) error

func (a redactedArray) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	for _, value := range a {
		if err := value(encoder); err != nil {
			return err
		}
	}
	return nil
}

func (r *redactionArray) AppendBool(value bool) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendBool(value); return nil })
}

func (r *redactionArray) AppendComplex128(value complex128) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendComplex128(value); return nil })
}

func (r *redactionArray) AppendComplex64(value complex64) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendComplex64(value); return nil })
}

func (r *redactionArray) AppendDuration(value time.Duration) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendDuration(value); return nil })
}

func (r *redactionArray) AppendFloat64(value float64) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendFloat64(value); return nil })
}

func (r *redactionArray) AppendFloat32(value float32) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendFloat32(value); return nil })
}

func (r *redactionArray) AppendInt(value int) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendInt(value); return nil })
}

func (r *redactionArray) AppendInt64(value int64) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendInt64(value); return nil })
}

func (r *redactionArray) AppendInt32(value int32) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendInt32(value); return nil })
}

func (r *redactionArray) AppendInt16(value int16) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendInt16(value); return nil })
}

func (r *redactionArray) AppendInt8(value int8) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendInt8(value); return nil })
}

func (r *redactionArray) AppendString(value string) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendString(value); return nil })
}

func (r *redactionArray) AppendTime(value time.Time) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendTime(value); return nil })
}

func (r *redactionArray) AppendUint(value uint) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendUint(value); return nil })
}

func (r *redactionArray) AppendUint64(value uint64) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendUint64(value); return nil })
}

func (r *redactionArray) AppendUint32(value uint32) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendUint32(value); return nil })
}

func (r *redactionArray) AppendUint16(value uint16) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendUint16(value); return nil })
}

func (r *redactionArray) AppendUint8(value uint8) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendUint8(value); return nil })
}

func (r *redactionArray) AppendUintptr(value uintptr) {
	if !r.state.take(r.depth) {
		return
	}

	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendUintptr(value); return nil })
}

func (r *redactionArray) AppendByteString(value []byte) {
	if !r.state.take(r.depth) {
		return
	}
	value = slices.Clone(value)
	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { encoder.AppendByteString(value); return nil })
}

func (r *redactionArray) AppendReflected(_ any) error {
	r.AppendString("[REDACTED]")
	return r.state.err
}

func (r *redactionArray) AppendObject(value zapcore.ObjectMarshaler) error {
	if !r.state.take(r.depth + 1) {
		return r.state.err
	}
	child := redactionObject{keys: r.keys, state: r.state, depth: r.depth + 1}
	if err := value.MarshalLogObject(&child); err != nil {
		r.state.err = ErrRedactionFailed
	}
	if r.state.err != nil {
		return r.state.err
	}
	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { return encoder.AppendObject(redactedObject(child.fields)) })
	return nil
}

func (r *redactionArray) AppendArray(value zapcore.ArrayMarshaler) error {
	if !r.state.take(r.depth + 1) {
		return r.state.err
	}
	child := redactionArray{keys: r.keys, state: r.state, depth: r.depth + 1}
	if err := value.MarshalLogArray(&child); err != nil {
		r.state.err = ErrRedactionFailed
	}
	if r.state.err != nil {
		return r.state.err
	}
	r.values = append(r.values, func(encoder zapcore.ArrayEncoder) error { return encoder.AppendArray(redactedArray(child.values)) })
	return nil
}
