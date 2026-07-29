package zlogger

import (
	"context"
	"slices"
)

// contextKey 避免與其他套件的 context key 衝突。
type contextKey string

const loggerContextKey = contextKey("zlogger_fields")

// WithContext 將欄位加入 context，並複製輸入 slice 以隔離呼叫端後續修改。
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

// FromContext 回傳 context 欄位的淺層副本。
// Field 內的 Interface 等參照型資料仍由呼叫端負責同步。
func FromContext(ctx context.Context) []Field {
	return slices.Clone(contextFields(ctx))
}

// contextFields 只供 package 內部唯讀，回傳值不得傳出 package 或修改。
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

// DebugContext 以 context 欄位記錄 debug 日誌。
func DebugContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Debug(msg, allFields...)
}

// InfoContext 以 context 欄位記錄 info 日誌。
func InfoContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Info(msg, allFields...)
}

// WarnContext 以 context 欄位記錄 warning 日誌。
func WarnContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Warn(msg, allFields...)
}

// ErrorContext 以 context 欄位記錄 error 日誌。
func ErrorContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Error(msg, allFields...)
}

// FatalContext 以 context 欄位記錄 fatal 日誌。
func FatalContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Fatal(msg, allFields...)
}

// WithRequestID 將 request ID 加入 context。
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return WithContext(ctx, String("request_id", requestID))
}

// WithUserID 將 user ID 加入 context。
func WithUserID(ctx context.Context, userID interface{}) context.Context {
	if userID == nil {
		return ctx
	}
	return WithContext(ctx, Any("user_id", userID))
}

// WithTraceID 將 trace ID 加入 context。
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}
	return WithContext(ctx, String("trace_id", traceID))
}

// WithOperation 將操作名稱加入 context。
func WithOperation(ctx context.Context, operation string) context.Context {
	if operation == "" {
		return ctx
	}
	return WithContext(ctx, String("operation", operation))
}

// WithComponent 將元件名稱加入 context。
func WithComponent(ctx context.Context, component string) context.Context {
	if component == "" {
		return ctx
	}
	return WithContext(ctx, String("component", component))
}

// mergeContextFields 將 context 欄位與本次日誌欄位合併。
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
