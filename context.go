package zlogger

import (
	"context"
)

// contextKey defines the context key type
type contextKey string

const loggerContextKey = contextKey("zlogger_fields")

// WithContext adds fields to the context
func WithContext(ctx context.Context, fields ...Field) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if len(fields) == 0 {
		return ctx
	}

	// Get existing fields
	existingFields := FromContext(ctx)

	// If no existing fields, use new fields directly
	if len(existingFields) == 0 {
		return context.WithValue(ctx, loggerContextKey, fields)
	}

	// Merge fields
	newFields := make([]Field, len(existingFields)+len(fields))
	copy(newFields, existingFields)
	copy(newFields[len(existingFields):], fields)

	return context.WithValue(ctx, loggerContextKey, newFields)
}

// FromContext extracts fields from the context
func FromContext(ctx context.Context) []Field {
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

// DebugContext logs a debug message with context
func DebugContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Debug(msg, allFields...)
}

// InfoContext logs an info message with context
func InfoContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Info(msg, allFields...)
}

// WarnContext logs a warning message with context
func WarnContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Warn(msg, allFields...)
}

// ErrorContext logs an error message with context
func ErrorContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Error(msg, allFields...)
}

// FatalContext logs a fatal error with context
func FatalContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Fatal(msg, allFields...)
}

// WithRequestID adds request ID to context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return WithContext(ctx, String("request_id", requestID))
}

// WithUserID adds user ID to context
func WithUserID(ctx context.Context, userID interface{}) context.Context {
	if userID == nil {
		return ctx
	}
	return WithContext(ctx, Any("user_id", userID))
}

// WithTraceID adds trace ID to context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}
	return WithContext(ctx, String("trace_id", traceID))
}

// WithOperation adds operation name to context
func WithOperation(ctx context.Context, operation string) context.Context {
	if operation == "" {
		return ctx
	}
	return WithContext(ctx, String("operation", operation))
}

// WithComponent adds component name to context
func WithComponent(ctx context.Context, component string) context.Context {
	if component == "" {
		return ctx
	}
	return WithContext(ctx, String("component", component))
}

// mergeContextFields merges context fields with provided fields
func mergeContextFields(ctx context.Context, fields []Field) []Field {
	if ctx == nil {
		return fields
	}

	ctxFields := FromContext(ctx)
	if len(ctxFields) == 0 {
		return fields
	}

	allFields := make([]Field, len(ctxFields)+len(fields))
	copy(allFields, ctxFields)
	copy(allFields[len(ctxFields):], fields)
	return allFields
}
