package zlogger

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestWithContext_NilContext(t *testing.T) {
	var nilCtx context.Context // nil context
	ctx := WithContext(nilCtx, String("key", "value"))

	if ctx == nil {
		t.Error("WithContext should return non-nil context")
	}

	fields := FromContext(ctx)
	if len(fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(fields))
	}
}

func TestWithContext_NoFields(t *testing.T) {
	ctx := context.Background()
	result := WithContext(ctx)

	if result != ctx {
		t.Error("WithContext with no fields should return original context")
	}
}

func TestWithContext_AddFields(t *testing.T) {
	ctx := context.Background()
	ctx = WithContext(ctx, String("key1", "value1"))
	ctx = WithContext(ctx, String("key2", "value2"))

	fields := FromContext(ctx)
	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fields))
	}
}

func TestFromContext_NilContext(t *testing.T) {
	var nilCtx context.Context // nil context
	fields := FromContext(nilCtx)

	if fields != nil {
		t.Errorf("FromContext(nil) should return nil, got %v", fields)
	}
}

func TestFromContext_NoFields(t *testing.T) {
	ctx := context.Background()
	fields := FromContext(ctx)

	if fields != nil {
		t.Errorf("FromContext with no fields should return nil, got %v", fields)
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")

	fields := FromContext(ctx)
	if len(fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(fields))
	}
	if fields[0].Key != "request_id" {
		t.Errorf("expected key 'request_id', got '%s'", fields[0].Key)
	}
}

func TestWithRequestID_EmptyString(t *testing.T) {
	ctx := context.Background()
	result := WithRequestID(ctx, "")

	// empty string should return original context
	fields := FromContext(result)
	if fields != nil {
		t.Errorf("WithRequestID with empty string should not add field, got %v", fields)
	}
}

func TestWithUserID(t *testing.T) {
	ctx := context.Background()
	ctx = WithUserID(ctx, 12345)

	fields := FromContext(ctx)
	if len(fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(fields))
	}
	if fields[0].Key != "user_id" {
		t.Errorf("expected key 'user_id', got '%s'", fields[0].Key)
	}
}

func TestWithUserID_Nil(t *testing.T) {
	ctx := context.Background()
	result := WithUserID(ctx, nil)

	fields := FromContext(result)
	if fields != nil {
		t.Errorf("WithUserID with nil should not add field, got %v", fields)
	}
}

func TestWithTraceID(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "trace-abc")

	fields := FromContext(ctx)
	if len(fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(fields))
	}
	if fields[0].Key != "trace_id" {
		t.Errorf("expected key 'trace_id', got '%s'", fields[0].Key)
	}
}

func TestWithOperation(t *testing.T) {
	ctx := context.Background()
	ctx = WithOperation(ctx, "login")

	fields := FromContext(ctx)
	if len(fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(fields))
	}
	if fields[0].Key != "operation" {
		t.Errorf("expected key 'operation', got '%s'", fields[0].Key)
	}
}

func TestWithComponent(t *testing.T) {
	ctx := context.Background()
	ctx = WithComponent(ctx, "auth-service")

	fields := FromContext(ctx)
	if len(fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(fields))
	}
	if fields[0].Key != "component" {
		t.Errorf("expected key 'component', got '%s'", fields[0].Key)
	}
}

func TestMergeContextFields_NilContext(t *testing.T) {
	var nilCtx context.Context // nil context
	fields := []Field{String("key", "value")}
	result := mergeContextFields(nilCtx, fields)

	if len(result) != 1 {
		t.Errorf("expected 1 field, got %d", len(result))
	}
}

func TestMergeContextFields_EmptyContext(t *testing.T) {
	ctx := context.Background()
	fields := []Field{String("key", "value")}
	result := mergeContextFields(ctx, fields)

	if len(result) != 1 {
		t.Errorf("expected 1 field, got %d", len(result))
	}
}

func TestMergeContextFields_MergeFields(t *testing.T) {
	ctx := context.Background()
	ctx = WithContext(ctx, String("ctx_key", "ctx_value"))

	fields := []Field{String("new_key", "new_value")}
	result := mergeContextFields(ctx, fields)

	if len(result) != 2 {
		t.Errorf("expected 2 fields, got %d", len(result))
	}
}

func TestContextLogFunctions(t *testing.T) {
	resetGlobalState()

	var buf bytes.Buffer
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zap.DebugLevel,
	)
	globalLogger = zap.New(core)
	globalConfig = DefaultConfig()

	// Create context with request_id
	ctx := WithRequestID(context.Background(), "req-123")

	// Use InfoContext
	InfoContext(ctx, "test message", String("extra", "data"))

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got: %s", output)
	}
	if !strings.Contains(output, `"request_id":"req-123"`) {
		t.Errorf("expected output to contain request_id, got: %s", output)
	}
	if !strings.Contains(output, `"extra":"data"`) {
		t.Errorf("expected output to contain extra field, got: %s", output)
	}
}

func TestContextLogFunctions_NilLogger(t *testing.T) {
	resetGlobalState()

	ctx := context.Background()

	// Should not panic when globalLogger is nil
	DebugContext(ctx, "debug")
	InfoContext(ctx, "info")
	WarnContext(ctx, "warn")
	ErrorContext(ctx, "error")
	// Do not test FatalContext as it calls os.Exit
}

func TestMultipleContextFields(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")
	ctx = WithUserID(ctx, 456)
	ctx = WithTraceID(ctx, "trace-789")
	ctx = WithOperation(ctx, "login")
	ctx = WithComponent(ctx, "auth")

	fields := FromContext(ctx)
	if len(fields) != 5 {
		t.Errorf("expected 5 fields, got %d", len(fields))
	}

	// Check all keys exist
	keys := make(map[string]bool)
	for _, f := range fields {
		keys[f.Key] = true
	}

	expectedKeys := []string{"request_id", "user_id", "trace_id", "operation", "component"}
	for _, key := range expectedKeys {
		if !keys[key] {
			t.Errorf("expected key '%s' not found", key)
		}
	}
}

func TestWithTraceID_EmptyString(t *testing.T) {
	ctx := context.Background()
	result := WithTraceID(ctx, "")

	fields := FromContext(result)
	if fields != nil {
		t.Errorf("WithTraceID with empty string should not add field, got %v", fields)
	}
}

func TestWithOperation_EmptyString(t *testing.T) {
	ctx := context.Background()
	result := WithOperation(ctx, "")

	fields := FromContext(result)
	if fields != nil {
		t.Errorf("WithOperation with empty string should not add field, got %v", fields)
	}
}

func TestWithComponent_EmptyString(t *testing.T) {
	ctx := context.Background()
	result := WithComponent(ctx, "")

	fields := FromContext(result)
	if fields != nil {
		t.Errorf("WithComponent with empty string should not add field, got %v", fields)
	}
}

func TestContextLogFunctions_AllLevels(t *testing.T) {
	resetGlobalState()

	var buf bytes.Buffer
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zap.DebugLevel,
	)
	globalLogger = zap.New(core)
	globalConfig = DefaultConfig()

	ctx := WithRequestID(context.Background(), "test-req")

	// Test all levels
	DebugContext(ctx, "debug message")
	InfoContext(ctx, "info message")
	WarnContext(ctx, "warn message")
	ErrorContext(ctx, "error message")

	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Error("expected debug message in output")
	}
	if !strings.Contains(output, "info message") {
		t.Error("expected info message in output")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("expected warn message in output")
	}
	if !strings.Contains(output, "error message") {
		t.Error("expected error message in output")
	}
}
