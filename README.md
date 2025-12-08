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
  max_size: 100
  max_age: 30
  max_backups: 10
  compress: true
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

// Logger 日誌中間件
func Logger() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 開始時間
        start := time.Now()

        // 生成請求 ID
        requestID := uuid.New().String()
        c.Set("requestID", requestID)
        c.Header("X-Request-ID", requestID)

        // 使用 zlogger.WithRequestID 將 requestID 添加到 context
        ctx := zlogger.WithRequestID(c.Request.Context(), requestID)
        c.Request = c.Request.WithContext(ctx)

        // 處理請求
        c.Next()

        // 請求結束，記錄詳細信息
        latency := time.Since(start)
        status := c.Writer.Status()
        clientIP := c.ClientIP()
        method := c.Request.Method
        path := c.Request.URL.Path
        query := c.Request.URL.RawQuery

        // 使用 Context 版本的日誌記錄函數
        if len(c.Errors) > 0 {
            zlogger.ErrorContext(ctx, "HTTP 請求",
                zlogger.String("method", method),
                zlogger.String("path", path),
                zlogger.String("query", query),
                zlogger.String("client_ip", clientIP),
                zlogger.Int("status", status),
                zlogger.Duration("latency", latency),
                zlogger.String("error", c.Errors.String()),
            )
        } else {
            zlogger.InfoContext(ctx, "HTTP 請求",
                zlogger.String("method", method),
                zlogger.String("path", path),
                zlogger.String("query", query),
                zlogger.String("client_ip", clientIP),
                zlogger.Int("status", status),
                zlogger.Duration("latency", latency),
            )
        }
    }
}
```

使用方式：

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/vincent119/zlogger"
    "your-project/middleware"
)

func main() {
    zlogger.Init(nil)
    defer zlogger.Sync()

    r := gin.New()
    r.Use(middleware.Logger())
    r.Run(":8080")
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

## 配置選項說明

| 選項             | 類型     | 預設值        | 說明                                      |
| ---------------- | -------- | ------------- | ----------------------------------------- |
| `level`          | string   | `"info"`      | 日誌級別：debug, info, warn, error, fatal |
| `format`         | string   | `"console"`   | 輸出格式：json, console                   |
| `outputs`        | []string | `["console"]` | 輸出目標：console, file                   |
| `log_path`       | string   | `"./logs"`    | 日誌檔案目錄                              |
| `file_name`      | string   | `""`          | 日誌檔案名稱（空則使用日期）              |
| `max_size`       | int      | `100`         | 單檔最大大小（MB）                        |
| `max_age`        | int      | `30`          | 保存天數                                  |
| `max_backups`    | int      | `10`          | 最大備份數                                |
| `compress`       | bool     | `true`        | 是否壓縮舊日誌                            |
| `add_caller`     | bool     | `true`        | 是否顯示調用位置                          |
| `add_stacktrace` | bool     | `false`       | 是否顯示堆疊追蹤                          |
| `development`    | bool     | `false`       | 開發模式                                  |

## License

MIT
