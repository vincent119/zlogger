package zlogger

import (
	"context"
)

// 定義 context key
type contextKey string

const loggerContextKey = contextKey("zlogger_fields")

// WithContext 將字段添加到上下文
func WithContext(ctx context.Context, fields ...Field) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if len(fields) == 0 {
		return ctx
	}

	// 獲取現有字段
	existingFields := FromContext(ctx)

	// 如果沒有現有字段，直接使用新字段
	if len(existingFields) == 0 {
		return context.WithValue(ctx, loggerContextKey, fields)
	}

	// 合併字段
	newFields := make([]Field, len(existingFields)+len(fields))
	copy(newFields, existingFields)
	copy(newFields[len(existingFields):], fields)

	return context.WithValue(ctx, loggerContextKey, newFields)
}

// FromContext 從上下文中提取字段
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

// DebugContext 使用上下文記錄調試信息
func DebugContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Debug(msg, allFields...)
}

// InfoContext 使用上下文記錄信息
func InfoContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Info(msg, allFields...)
}

// WarnContext 使用上下文記錄警告信息
func WarnContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Warn(msg, allFields...)
}

// ErrorContext 使用上下文記錄錯誤信息
func ErrorContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Error(msg, allFields...)
}

// FatalContext 使用上下文記錄致命錯誤
func FatalContext(ctx context.Context, msg string, fields ...Field) {
	if globalLogger == nil {
		return
	}

	allFields := mergeContextFields(ctx, fields)
	globalLogger.Fatal(msg, allFields...)
}

// WithRequestID 將請求 ID 添加到上下文
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return WithContext(ctx, String("request_id", requestID))
}

// WithUserID 將用戶 ID 添加到上下文
func WithUserID(ctx context.Context, userID interface{}) context.Context {
	if userID == nil {
		return ctx
	}
	return WithContext(ctx, Any("user_id", userID))
}

// WithTraceID 將追蹤 ID 添加到上下文
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}
	return WithContext(ctx, String("trace_id", traceID))
}

// WithOperation 將操作名稱添加到上下文
func WithOperation(ctx context.Context, operation string) context.Context {
	if operation == "" {
		return ctx
	}
	return WithContext(ctx, String("operation", operation))
}

// WithComponent 將組件名稱添加到上下文
func WithComponent(ctx context.Context, component string) context.Context {
	if component == "" {
		return ctx
	}
	return WithContext(ctx, String("component", component))
}

// mergeContextFields 合併上下文字段和傳入字段
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
