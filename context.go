package zlogger

import (
	"context"
	"slices"
)

// contextKey prevents collisions with context keys from other packages.
type contextKey string

const loggerContextKey = contextKey("zlogger_fields")

// WithContext adds fields to a context, copying the input slice to isolate later changes.
func WithContext(ctx context.Context, fields ...Field) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if len(fields) == 0 {
		return ctx
	}

	existingFields := contextFields(ctx)

	newFields := make([]Field, len(existingFields)+len(fields))
	copy(newFields, existingFields)
	copy(newFields[len(existingFields):], fields)

	return context.WithValue(ctx, loggerContextKey, newFields)
}

// FromContext returns a shallow copy of the context fields.
// Callers remain responsible for synchronizing reference values within each Field.
func FromContext(ctx context.Context) []Field {
	return slices.Clone(contextFields(ctx))
}

// contextFields is read-only internal access. Do not expose or modify its return value.
func contextFields(ctx context.Context) []Field {
	if ctx == nil {
		return nil
	}

	if val := ctx.Value(loggerContextKey); val != nil {
		if fields, ok := val.([]Field); ok {
			return fields
		}
	}
	return nil
}

// DebugContext logs a debug message with context fields.
func DebugContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Debug(msg, allFields...)
}

// InfoContext logs an info message with context fields.
func InfoContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Info(msg, allFields...)
}

// WarnContext logs a warning message with context fields.
func WarnContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Warn(msg, allFields...)
}

// ErrorContext logs an error message with context fields.
func ErrorContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Error(msg, allFields...)
}

// FatalContext logs a fatal message with context fields.
func FatalContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Fatal(msg, allFields...)
}

// WithRequestID adds a request ID to a context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return WithContext(ctx, String("request_id", requestID))
}

// WithUserID adds a user ID to a context.
func WithUserID(ctx context.Context, userID interface{}) context.Context {
	if userID == nil {
		return ctx
	}
	return WithContext(ctx, Any("user_id", userID))
}

// WithTraceID adds a trace ID to a context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}
	return WithContext(ctx, String("trace_id", traceID))
}

// WithOperation adds an operation name to a context.
func WithOperation(ctx context.Context, operation string) context.Context {
	if operation == "" {
		return ctx
	}
	return WithContext(ctx, String("operation", operation))
}

// WithComponent adds a component name to a context.
func WithComponent(ctx context.Context, component string) context.Context {
	if component == "" {
		return ctx
	}
	return WithContext(ctx, String("component", component))
}

// mergeContextFields combines context fields with fields for the current entry.
func mergeContextFields(ctx context.Context, fields []Field) []Field {
	if ctx == nil {
		return fields
	}

	ctxFields := contextFields(ctx)
	if len(ctxFields) == 0 {
		return fields
	}

	allFields := make([]Field, len(ctxFields)+len(fields))
	copy(allFields, ctxFields)
	copy(allFields[len(ctxFields):], fields)
	return allFields
}
