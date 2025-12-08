// Package zlogger 提供基於 zap 的結構化日誌庫
//
// # 基本使用
//
//	import "github.com/vincentsys/zlogger"
//
//	func main() {
//	    // 使用預設配置初始化
//	    zlogger.Init(nil)
//	    defer zlogger.Sync()
//
//	    // 記錄日誌
//	    zlogger.Info("Hello", zlogger.String("key", "value"))
//	}
//
// # 自定義配置
//
//	cfg := &zlogger.Config{
//	    Level:   "debug",
//	    Format:  "json",
//	    Outputs: []string{"console", "file"},
//	    LogPath: "./logs",
//	}
//	zlogger.Init(cfg)
//
// # 從設定檔載入（YAML 範例）
//
//	type AppConfig struct {
//	    Log zlogger.Config `yaml:"log"`
//	}
//
//	// 載入後
//	zlogger.Init(&appConfig.Log)
//
// # Context 支援
//
//	ctx := zlogger.WithRequestID(context.Background(), "req-123")
//	zlogger.InfoContext(ctx, "處理請求", zlogger.String("action", "login"))
//
// # Gin 中間件使用方式
//
//	import (
//	    "github.com/gin-gonic/gin"
//	    "github.com/google/uuid"
//	    "github.com/vincentsys/zlogger"
//	    "time"
//	)
//
//	func ZLoggerMiddleware() gin.HandlerFunc {
//	    return func(c *gin.Context) {
//	        start := time.Now()
//	        path := c.Request.URL.Path
//	        query := c.Request.URL.RawQuery
//
//	        // 生成請求 ID
//	        requestID := uuid.New().String()
//	        c.Set("requestID", requestID)
//	        c.Header("X-Request-ID", requestID)
//
//	        // 創建帶請求ID的上下文
//	        ctx := zlogger.WithRequestID(c.Request.Context(), requestID)
//	        c.Request = c.Request.WithContext(ctx)
//
//	        c.Next()
//
//	        latency := time.Since(start)
//	        clientIP := c.ClientIP()
//	        method := c.Request.Method
//	        statusCode := c.Writer.Status()
//
//	        if len(c.Errors) > 0 {
//	            zlogger.ErrorContext(ctx, "HTTP 請求",
//	                zlogger.String("method", method),
//	                zlogger.String("path", path),
//	                zlogger.String("query", query),
//	                zlogger.String("client_ip", clientIP),
//	                zlogger.Int("status", statusCode),
//	                zlogger.Duration("latency", latency),
//	                zlogger.String("error", c.Errors.String()),
//	            )
//	        } else {
//	            zlogger.InfoContext(ctx, "HTTP 請求",
//	                zlogger.String("method", method),
//	                zlogger.String("path", path),
//	                zlogger.String("query", query),
//	                zlogger.String("client_ip", clientIP),
//	                zlogger.Int("status", statusCode),
//	                zlogger.Duration("latency", latency),
//	            )
//	        }
//	    }
//	}
//
//	// 使用方式
//	r := gin.New()
//	r.Use(ZLoggerMiddleware())
package zlogger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 重新導出 zap 的常用類型，方便使用者直接使用
type (
	// Logger 是 zap.Logger 的別名
	Logger = zap.Logger
	// SugaredLogger 是 zap.SugaredLogger 的別名
	SugaredLogger = zap.SugaredLogger
	// Level 是 zapcore.Level 的別名
	Level = zapcore.Level
	// EncoderConfig 是 zapcore.EncoderConfig 的別名
	EncoderConfig = zapcore.EncoderConfig
	// Core 是 zapcore.Core 的別名
	Core = zapcore.Core
)

// 常用的日誌級別常量
const (
	DebugLevel  = zapcore.DebugLevel
	InfoLevel   = zapcore.InfoLevel
	WarnLevel   = zapcore.WarnLevel
	ErrorLevel  = zapcore.ErrorLevel
	DPanicLevel = zapcore.DPanicLevel
	PanicLevel  = zapcore.PanicLevel
	FatalLevel  = zapcore.FatalLevel
)

// Sugar 返回 SugaredLogger，提供更方便的 API
func Sugar() *SugaredLogger {
	if globalLogger != nil {
		return globalLogger.Sugar()
	}
	return nil
}

// Named 創建具名的子 logger
func Named(name string) *Logger {
	if globalLogger != nil {
		return globalLogger.Named(name)
	}
	return nil
}

// With 創建帶有預設字段的子 logger
func With(fields ...Field) *Logger {
	if globalLogger != nil {
		return globalLogger.With(fields...)
	}
	return nil
}

// WithOptions 創建帶有額外選項的 logger
func WithOptions(opts ...zap.Option) *Logger {
	if globalLogger != nil {
		return globalLogger.WithOptions(opts...)
	}
	return nil
}

// NewNop 創建一個不輸出任何內容的 logger（用於測試）
func NewNop() *Logger {
	return zap.NewNop()
}

// NewDevelopment 創建開發模式的 logger
func NewDevelopment() (*Logger, error) {
	return zap.NewDevelopment()
}

// NewProduction 創建生產模式的 logger
func NewProduction() (*Logger, error) {
	return zap.NewProduction()
}

