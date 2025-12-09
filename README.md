# zlogger

基於 [zap](https://github.com/uber-go/zap) 的結構化日誌庫，提供簡潔的 API 和豐富的功能。

## 安裝

```bash
go get github.com/vincent119/zlogger
```

## 基本使用

```go
import "github.com/vincent119/zlogger"

func main() {
    // 使用預設配置初始化
    zlogger.Init(nil)
    defer zlogger.Sync()

    // 記錄日誌
    zlogger.Info("Hello", zlogger.String("key", "value"))
    zlogger.Debug("除錯訊息", zlogger.Int("count", 42))
    zlogger.Error("錯誤", zlogger.Err(err))
}
```

## 自定義配置

```go
cfg := &zlogger.Config{
    Level:       "debug",
    Format:      "json",
    Outputs:     []string{"console", "file"},
    LogPath:     "./logs",
    FileName:    "app.log",
    AddCaller:   true,
    Development: true,
}
zlogger.Init(cfg)
```

## 從設定檔載入

### YAML 範例

```yaml
# config.yaml
log:
  level: debug
  format: json
  outputs:
    - console
    - file
  log_path: ./logs
  file_name: app.log
  add_caller: true
  add_stacktrace: false
  development: false
```

```go
type AppConfig struct {
    Log zlogger.Config `yaml:"log"`
}

// 載入設定後
var appConfig AppConfig
// ... 載入 YAML ...
zlogger.Init(&appConfig.Log)
```

### JSON 範例

```json
{
  "log": {
    "level": "info",
    "format": "json",
    "outputs": ["console"],
    "log_path": "./logs",
    "add_caller": true
  }
}
```

## Context 支援

```go
// 創建帶有追蹤資訊的 context
ctx := zlogger.WithRequestID(context.Background(), "req-123")
ctx = zlogger.WithUserID(ctx, 12345)
ctx = zlogger.WithTraceID(ctx, "trace-abc")

// 使用 context 記錄日誌（自動帶入追蹤資訊）
zlogger.InfoContext(ctx, "處理請求", zlogger.String("action", "login"))
zlogger.ErrorContext(ctx, "請求失敗", zlogger.Err(err))
```

## Field 輔助函數

```go
zlogger.String("key", "value")
zlogger.Int("count", 42)
zlogger.Int64("id", 123456789)
zlogger.Float64("price", 99.99)
zlogger.Bool("active", true)
zlogger.Err(err)
zlogger.Any("data", someStruct)
zlogger.Duration("latency", time.Second)
zlogger.Time("timestamp", time.Now())
```

## 動態調整日誌級別

```go
zlogger.SetLevel("debug")  // 運行時調整級別
```

## 應用程式日誌使用範例

### 一般日誌記錄

```go
// 基本訊息
zlogger.Info("伺服器啟動", zlogger.String("port", "8080"))

// 除錯訊息
zlogger.Debug("處理請求", zlogger.String("endpoint", "/api/users"))

// 警告訊息
zlogger.Warn("連線池接近上限", zlogger.Int("current", 95), zlogger.Int("max", 100))

// 錯誤訊息
zlogger.Error("註冊翻譯器失敗", zlogger.String("validator", "zh"), zlogger.Err(err))

// 多個欄位
zlogger.Info("用戶登入成功",
    zlogger.Uint("user_id", 12345),
    zlogger.String("username", "john"),
    zlogger.String("ip", "192.168.1.1"),
    zlogger.Duration("latency", time.Millisecond*150),
)
```

### 搭配 Gin Context 使用

```go
func GetUserHandler(c *gin.Context) {
    userID := c.GetUint("userID")

    user, err := userService.GetByID(userID)
    if err != nil {
        // 使用 Context 記錄錯誤，會自動帶入 request_id
        zlogger.ErrorContext(c.Request.Context(), "獲取用戶信息失敗",
            zlogger.Uint("id", userID),
            zlogger.Err(err),
        )
        c.JSON(500, gin.H{"error": "獲取用戶失敗"})
        return
    }

    zlogger.InfoContext(c.Request.Context(), "獲取用戶成功",
        zlogger.Uint("id", userID),
        zlogger.String("username", user.Username),
    )
    c.JSON(200, user)
}
```

### 資料庫操作日誌

```go
func (r *UserRepo) Create(user *User) error {
    result := r.db.Create(user)
    if result.Error != nil {
        zlogger.Error("創建用戶失敗",
            zlogger.String("username", user.Username),
            zlogger.Err(result.Error),
        )
        return result.Error
    }

    zlogger.Info("創建用戶成功",
        zlogger.Uint("id", user.ID),
        zlogger.String("username", user.Username),
    )
    return nil
}
```

### 服務層日誌

```go
func (s *AuthService) Login(ctx context.Context, username, password string) (*Token, error) {
    zlogger.DebugContext(ctx, "嘗試登入",
        zlogger.String("username", username),
    )

    user, err := s.userRepo.FindByUsername(username)
    if err != nil {
        zlogger.WarnContext(ctx, "用戶不存在",
            zlogger.String("username", username),
        )
        return nil, ErrUserNotFound
    }

    if !s.verifyPassword(user.Password, password) {
        zlogger.WarnContext(ctx, "密碼錯誤",
            zlogger.Uint("user_id", user.ID),
            zlogger.String("username", username),
        )
        return nil, ErrInvalidPassword
    }

    token, err := s.generateToken(user)
    if err != nil {
        zlogger.ErrorContext(ctx, "生成 Token 失敗",
            zlogger.Uint("user_id", user.ID),
            zlogger.Err(err),
        )
        return nil, err
    }

    zlogger.InfoContext(ctx, "登入成功",
        zlogger.Uint("user_id", user.ID),
        zlogger.String("username", username),
    )
    return token, nil
}
```

## Gin 中間件

建立 `middleware/logger.go`：

```go
package middleware

import (
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/vincent119/zlogger"
)

// Context key 常數
const (
    LogCategoryKey = "log_category"
    LogFunctionKey = "log_function"
    LogSkipKey     = "log_skip"   // 用於跳過中間件 log
    LogFieldsKey   = "log_fields" // 用於存放自定義欄位
)

// Zfn 定義 context 欄位函數型別
type Zfn func(*gin.Context) []zlogger.Field

// Zconfig 日誌中間件配置
type Zconfig struct {
    TimeFormat   string
    UTC          bool
    SkipPaths    []string
    Context      Zfn
    DefaultLevel zlogger.Level
    Category     string
}

// SetLogCategory 設定 log category（供 handler 使用）
func SetLogCategory(c *gin.Context, category string) {
    c.Set(LogCategoryKey, category)
}

// SetLogFunction 設定 log function（供 handler 使用）
func SetLogFunction(c *gin.Context, function string) {
    c.Set(LogFunctionKey, function)
}

// SkipMiddlewareLog 跳過中間件的 log（handler 已自行記錄時使用）
func SkipMiddlewareLog(c *gin.Context) {
    c.Set(LogSkipKey, true)
}

// SetLogFields 設定多個自定義欄位（供 handler 使用）
// 用法: middleware.SetLogFields(c, zlogger.String("key", "value"), zlogger.Int("count", 1))
func SetLogFields(c *gin.Context, fields ...zlogger.Field) {
    if existing, exists := c.Get(LogFieldsKey); exists {
        fields = append(existing.([]zlogger.Field), fields...)
    }
    c.Set(LogFieldsKey, fields)
}

func Logger() gin.HandlerFunc {
    return LoggerWithConfig(&Zconfig{
        TimeFormat:   time.RFC3339,
        UTC:          true,
        DefaultLevel: zlogger.InfoLevel,
        Category:     "http",
    })
}

// LoggerWithConfig 可配置的日誌中間件
func LoggerWithConfig(conf *Zconfig) gin.HandlerFunc {
    skipPaths := make(map[string]bool, len(conf.SkipPaths))
    for _, path := range conf.SkipPaths {
        skipPaths[path] = true
    }

    // 預設 category
    category := conf.Category

    return func(c *gin.Context) {
        start := time.Now()
        path := c.Request.URL.Path

        if _, ok := skipPaths[path]; ok {
            c.Next()
            return
        }

        requestID := uuid.New().String()
        c.Set("requestID", requestID)
        c.Header("X-Request-ID", requestID)

        ctx := zlogger.WithRequestID(c.Request.Context(), requestID)
        c.Request = c.Request.WithContext(ctx)

        c.Next()

        // 檢查是否跳過中間件 log
        if skip, exists := c.Get(LogSkipKey); exists && skip.(bool) {
            return
        }

        latency := time.Since(start)

        // 優先使用 handler 設定的 category，否則使用預設值
        logCategory := category
        if handlerCategory, exists := c.Get(LogCategoryKey); exists {
            logCategory = handlerCategory.(string)
        }

        fields := []zlogger.Field{
            zlogger.String("method", c.Request.Method),
            zlogger.String("path", path),
            zlogger.String("query", c.Request.URL.RawQuery),
            zlogger.String("ip", c.ClientIP()),
            zlogger.Int("status", c.Writer.Status()),
            zlogger.Duration("latency", latency),
            zlogger.String("user-agent", c.Request.UserAgent()),
            zlogger.String("category", logCategory),
        }

        // 如果 handler 設定了 function，加入 fields
        if function, exists := c.Get(LogFunctionKey); exists {
            fields = append(fields, zlogger.String("function", function.(string)))
        }

        // 加入 handler 設定的自定義欄位
        if customFields, exists := c.Get(LogFieldsKey); exists {
            fields = append(fields, customFields.([]zlogger.Field)...)
        }

        if conf.Context != nil {
            fields = append(fields, conf.Context(c)...)
        }

        if len(c.Errors) > 0 {
            fields = append(fields, zlogger.String("error", c.Errors.String()))
            zlogger.ErrorContext(ctx, "HTTP Request Error", fields...)
        } else {
            zlogger.InfoContext(ctx, "HTTP Request", fields...)
        }
    }
}

```

使用方式：

```go
// Package test is the route for the test api
package test

import (
    "fmt"
    "time"

    middleware "status-webhooks/internal/handled/middleware"

    "github.com/gin-gonic/gin"
    "github.com/vincent119/zlogger"
)

type H58body struct {
    Result   string `json:"result"`
    Leftover string `json:"leftover"`
}

func NumberCheck(c *gin.Context) {
    // 方式一：使用 SetLogFields 傳入任意欄位
    middleware.SetLogFields(c,
        zlogger.String("category", "nuage"),
        zlogger.String("function", "CallBack"),
        zlogger.String("data", "11111"),
    )

    // 方式二：使用 SetLogFields 傳入其他自定義欄位
    middleware.SetLogFields(c, zlogger.String("user_id", "12345"))
    // 可多次呼叫，欄位會累加
    middleware.SetLogFields(c, zlogger.Int("retry_count", 3))


    c.JSON(200, gin.H{
        "Status": "OK", "recv_time": fmt.Sprint(time.Now().Format("2006-01-02T15:04:05")),
    })

}


```

## 按級別分離日誌檔案

```go
import "go.uber.org/zap/zapcore"

// 創建分離輸出的核心
core, cleanup, err := zlogger.GetSplitCore("./logs", "app", zapcore.EncoderConfig{
    TimeKey:        "ts",
    LevelKey:       "level",
    MessageKey:     "msg",
    EncodeTime:     zapcore.ISO8601TimeEncoder,
    EncodeLevel:    zapcore.CapitalLevelEncoder,
})
if err != nil {
    panic(err)
}
defer cleanup()

// 會產生以下檔案：
// - logs/app-info-2024-01-01.log
// - logs/app-warn-2024-01-01.log
// - logs/app-error-2024-01-01.log
```

## Log Rotation（使用 timberjack）

zlogger 本身不包含 log rotation 功能，建議使用 [timberjack](https://github.com/DeRuina/timberjack) 處理：

```bash
go get github.com/DeRuina/timberjack
```

```go
package main

import (
    "github.com/DeRuina/timberjack"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
    "github.com/vincent119/zlogger"
)

func main() {
    // 設定 timberjack 日誌輪轉
    tjLogger := &timberjack.Logger{
        Filename:   "./logs/app.log",
        MaxSize:    100,   // 單檔最大大小（MB）
        MaxBackups: 10,    // 最大備份數
        MaxAge:     30,    // 保存天數
        Compress:   true,  // 是否壓縮舊日誌（gzip）
    }

    // 建立編碼器配置
    encoderConfig := zapcore.EncoderConfig{
        TimeKey:        "ts",
        LevelKey:       "level",
        NameKey:        "logger",
        CallerKey:      "caller",
        MessageKey:     "msg",
        StacktraceKey:  "stacktrace",
        LineEnding:     zapcore.DefaultLineEnding,
        EncodeLevel:    zapcore.CapitalLevelEncoder,
        EncodeTime:     zapcore.ISO8601TimeEncoder,
        EncodeDuration: zapcore.StringDurationEncoder,
        EncodeCaller:   zapcore.ShortCallerEncoder,
    }

    // 建立核心
    core := zapcore.NewCore(
        zapcore.NewJSONEncoder(encoderConfig),
        zapcore.AddSync(tjLogger),
        zap.InfoLevel,
    )

    // 建立 logger
    logger := zap.New(core, zap.AddCaller())
    zap.ReplaceGlobals(logger)

    // 現在可以使用 zlogger 的函數（如果需要）
    // 或直接使用 zap.L()
    logger.Info("伺服器啟動", zap.String("port", "8080"))
}
```

### 搭配 zlogger 使用

如果想同時輸出到 console 和帶有 rotation 的檔案：

```go
// Console 輸出
consoleCore := zapcore.NewCore(
    zapcore.NewConsoleEncoder(encoderConfig),
    zapcore.Lock(os.Stdout),
    zap.DebugLevel,
)

// File 輸出（帶 rotation）
fileCore := zapcore.NewCore(
    zapcore.NewJSONEncoder(encoderConfig),
    zapcore.AddSync(tjLogger),
    zap.InfoLevel,
)

// 合併輸出
core := zapcore.NewTee(consoleCore, fileCore)
logger := zap.New(core, zap.AddCaller())
zap.ReplaceGlobals(logger)
```

## 配置選項說明

| 選項             | 類型     | 預設值        | 說明                                      |
| ---------------- | -------- | ------------- | ----------------------------------------- |
| `level`          | string   | `"info"`      | 日誌級別：debug, info, warn, error, fatal |
| `format`         | string   | `"console"`   | 輸出格式：json, console                   |
| `outputs`        | []string | `["console"]` | 輸出目標：console, file                   |
| `log_path`       | string   | `"./logs"`    | 日誌檔案目錄                              |
| `file_name`      | string   | `""`          | 日誌檔案名稱（空則使用日期）              |
| `add_caller`     | bool     | `true`        | 是否顯示調用位置                          |
| `add_stacktrace` | bool     | `false`       | 是否顯示堆疊追蹤                          |
| `development`    | bool     | `false`       | 開發模式                                  |

> **Note:** Log rotation（檔案大小限制、備份、壓縮）請使用 timberjack，參考上方範例。

## License

MIT
