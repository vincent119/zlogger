# Gin Integration

[繁體中文（台灣）](../../zh-TW/integrations/gin.md) | **English**

[Documentation index](../README.md)

Put this middleware in your application and configure the zlogger global logger first.
These functions belong to your application. zlogger doesn't depend on Gin.
The encoder handles timestamps. The middleware measures elapsed time.

Names such as method, path, status, and category are choices made by this example.
Change them to fit your team. InfoContext and ErrorContext use the global logger, not an Instance.

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
			zlogger.ErrorContext(ctx, "HTTP request failed", fields...)
			return
		}
		zlogger.InfoContext(ctx, "HTTP request", fields...)
	}
}
```

## Add fields from a handler

```go
middleware.SetLogFields(c, zlogger.String("user_id", "12345"))
middleware.SetLogCategory(c, "callback")

zlogger.InfoContext(c.Request.Context(), "process callback")
middleware.SkipMiddlewareLog(c)
```

Call `SkipMiddlewareLog` after the handler has already recorded the event to avoid duplicate logs.
This example reuses X-Request-ID directly. If that header can come from an untrusted client,
validate its length, format, and source before using it as a request identifier in your logs.
Generate IDs in your application when needed.

The example logs no request body, complete headers, or query string, and doesn't enable redaction.
If a URL path contains personal data, use a route template or another safe representation instead.
The middleware captures ctx before the handler runs. It doesn't reread a request context replaced
by the handler.
