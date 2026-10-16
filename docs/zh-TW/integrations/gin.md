# Gin 整合

**繁體中文（台灣）** | [English](../../en/integrations/gin.md)

[返回文件首頁](../README.md)

把這份 middleware 放進自己的應用程式，並先設定 zlogger 全域 logger。
zlogger 本身不依賴 Gin，也沒有內建這些函式。時間格式交給 encoder，middleware 只計算耗時。

method、path、status、category 都是範例選的名稱，可以依團隊規範修改。
這裡使用全域 InfoContext／ErrorContext，不會自動改用獨立 Instance。

## Middleware

```go
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vincent119/zlogger"
)

const (
	logCategoryKey = "log_category"
	logSkipKey     = "log_skip"
	logFieldsKey   = "log_fields"
)

type Zfn func(*gin.Context) []zlogger.Field

type Zconfig struct {
	SkipPaths []string
	Context   Zfn
	Category  string
}

func SkipMiddlewareLog(c *gin.Context) { c.Set(logSkipKey, true) }

func SetLogFields(c *gin.Context, fields ...zlogger.Field) {
	current, _ := c.Get(logFieldsKey)
	existing, _ := current.([]zlogger.Field)
	c.Set(logFieldsKey, append(existing, fields...))
}

func SetLogCategory(c *gin.Context, category string) {
	c.Set(logCategoryKey, category)
}

func Logger(conf Zconfig) gin.HandlerFunc {
	skipPaths := make(map[string]struct{}, len(conf.SkipPaths))
	for _, path := range conf.SkipPaths {
		skipPaths[path] = struct{}{}
	}

	return func(c *gin.Context) {
		if _, skip := skipPaths[c.Request.URL.Path]; skip {
			c.Next()
			return
		}

		start := time.Now()
		ctx := c.Request.Context()
		if requestID := c.GetHeader("X-Request-ID"); requestID != "" {
			ctx = zlogger.WithRequestID(ctx, requestID)
			c.Request = c.Request.WithContext(ctx)
		}

		c.Next()
		if skip, _ := c.Get(logSkipKey); skip == true {
			return
		}

		category := conf.Category
		if value, ok := c.Get(logCategoryKey); ok {
			category, _ = value.(string)
		}
		fields := []zlogger.Field{
			zlogger.String("method", c.Request.Method),
			zlogger.String("path", c.Request.URL.Path),
			zlogger.Int("status", c.Writer.Status()),
			zlogger.Duration("latency", time.Since(start)),
			zlogger.String("category", category),
		}
		if value, ok := c.Get(logFieldsKey); ok {
			custom, _ := value.([]zlogger.Field)
			fields = append(fields, custom...)
		}
		if conf.Context != nil {
			fields = append(fields, conf.Context(c)...)
		}

		if len(c.Errors) > 0 {
			zlogger.ErrorContext(ctx, "HTTP 請求處理失敗", fields...)
			return
		}
		zlogger.InfoContext(ctx, "HTTP 請求", fields...)
	}
}
```

## 在 handler 加入欄位

```go
middleware.SetLogFields(c, zlogger.String("user_id", "12345"))
middleware.SetLogCategory(c, "callback")

zlogger.InfoContext(c.Request.Context(), "處理 callback")
middleware.SkipMiddlewareLog(c)
```

handler 已自行記錄完整事件時，才呼叫 `SkipMiddlewareLog`，避免重複記錄。
這個範例直接沿用 X-Request-ID。正式服務應在可信任邊界檢查長度、格式與來源，
需要自行產生 ID 時由應用程式提供產生器。

範例不記錄 request body、完整 headers 或 query string，也不自動啟用遮罩。
若路徑本身含有個資，請在應用端改用路由樣板或其他不含敏感資料的表示方式。
ctx 在進入 handler 前擷取，handler 若另建 request context，這段 middleware 不會自動重新讀取。
