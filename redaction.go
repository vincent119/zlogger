package zlogger

import (
	"errors"
	"fmt"
	"slices"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ErrRedactionFailed indicates that an entry cannot be processed safely. It contains no original values or marshaler errors.
var ErrRedactionFailed = errors.New("log redaction failed")

// RedactionConfig sets the construction-time policy. It defaults to disabled. Later changes do not affect existing cores.
type RedactionConfig struct {
	Enabled bool     `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Keys    []string `json:"keys" yaml:"keys" mapstructure:"keys"`
}

// NewRedactingCore creates a core that intercepts sensitive fields before With evaluates them.
// The encoder must have no bound fields. Existing encoder, sink, and level concurrency requirements still apply.
// When disabled, Keys is ignored and zapcore.NewCore is returned. When enabled, Keys requires 1 to 128 nonempty keys.
// Sink ownership remains with the caller. Free-text messages are not protected. Opaque values are fully masked when enabled.
func NewRedactingCore(encoder zapcore.Encoder, sink zapcore.WriteSyncer, level zapcore.LevelEnabler, cfg RedactionConfig) (zapcore.Core, error) {
	if encoder == nil || sink == nil || level == nil {
		return nil, fmt.Errorf("%w: encoder, sink, and level must not be nil", ErrInvalidConfig)
	}
	if !cfg.Enabled {
		return zapcore.NewCore(encoder, sink, level), nil
	}
	if len(cfg.Keys) == 0 || len(cfg.Keys) > 128 {
		return nil, fmt.Errorf("%w: redaction requires 1 to 128 keys", ErrInvalidConfig)
	}
	keys := make(map[string]struct{}, len(cfg.Keys))
	for _, key := range cfg.Keys {
		if len(key) == 0 || len(key) > 128 {
			return nil, fmt.Errorf("%w: redaction key length must be between 1 and 128 bytes", ErrInvalidConfig)
		}
		keys[redactionKey(key)] = struct{}{}
	}
	return &redactingCore{Core: zapcore.NewCore(encoder, sink, level), context: redactionObject{keys: keys}}, nil
}

func redactionKey(key string) string {
	value := []byte(key)
	for i, ch := range value {
		if ch >= 'A' && ch <= 'Z' {
			value[i] = ch + ('a' - 'A')
		}
	}
	return string(value)
}

type redactingCore struct {
	zapcore.Core
	context redactionObject
}

func (c *redactingCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

func (c *redactingCore) With(fields []zapcore.Field) zapcore.Core {
	context := c.context.clone()
	context.addFields(fields)
	return &redactingCore{Core: c.Core, context: context}
}

func (c *redactingCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	context := c.context.clone()
	context.addFields(fields)
	if context.state.err != nil {
		return ErrRedactionFailed
	}
	return c.Core.Write(entry, context.fields)
}

type redactionState struct {
	count int
	err   error
}

func (s *redactionState) take(depth int) bool {
	if s.err != nil {
		return false
	}
	s.count++
	if depth > 32 || s.count > 1024 {
		s.err = ErrRedactionFailed
		return false
	}
	return true
}

type redactionObject struct {
	keys       map[string]struct{}
	state      *redactionState
	fields     []zapcore.Field
	depth      int
	suppressed bool
}

func (r *redactionObject) clone() redactionObject {
	state := redactionState{}
	if r.state != nil {
		state = *r.state
	}
	return redactionObject{keys: r.keys, state: &state, fields: slices.Clone(r.fields), depth: r.depth, suppressed: r.suppressed}
}

func (r *redactionObject) allow(key string) bool {
	if !r.state.take(r.depth) || r.suppressed {
		return false
	}
	if _, matched := r.keys[redactionKey(key)]; matched {
		r.fields = append(r.fields, zap.String(key, "[REDACTED]"))
		return false
	}
	return true
}

func (r *redactionObject) addFields(fields []zapcore.Field) {
	for _, field := range fields {
		if r.state.err != nil {
			return
		}
		r.addField(field)
	}
}

func (r *redactionObject) addField(field zapcore.Field) {
	// User marshalers and invalid fields must not leak values through panics or original errors.
	defer func() {
		if recover() != nil {
			r.state.err = ErrRedactionFailed
		}
	}()
	if field.Type == zapcore.SkipType {
		return
	}
	if field.Type == zapcore.UnknownType || field.Type > zapcore.InlineMarshalerType {
		r.state.err = ErrRedactionFailed
		return
	}
	switch field.Type {
	case zapcore.ErrorType, zapcore.StringerType, zapcore.ReflectType:
		r.AddString(field.Key, "[REDACTED]")
	case zapcore.InlineMarshalerType:
		if r.allow(field.Key) {
			child := redactionObject{keys: r.keys, state: r.state, depth: r.depth + 1}
			if !r.state.take(child.depth) {
				return
			}
			if err := field.Interface.(zapcore.ObjectMarshaler).MarshalLogObject(&child); err != nil {
				r.state.err = ErrRedactionFailed
			}
			r.fields = append(r.fields, child.fields...)
			r.suppressed, r.depth = child.suppressed, child.depth-1
		}
	default:
		field.AddTo(r)
	}
}
