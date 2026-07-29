# zlogger

[![GitHub](https://img.shields.io/badge/github-vincent119/zlogger-blue?logo=github)](https://github.com/vincent119/zlogger)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![CI](https://github.com/vincent119/zlogger/actions/workflows/ci.yml/badge.svg)](https://github.com/vincent119/zlogger/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/gh/vincent119/zlogger/branch/main/graph/badge.svg)](https://codecov.io/gh/vincent119/zlogger)
[![Go Report Card](https://goreportcard.com/badge/github.com/vincent119/zlogger)](https://goreportcard.com/report/github.com/vincent119/zlogger)

**[English](README.en.md)**

基於 [zap](https://github.com/uber-go/zap) 的結構化日誌庫，提供簡潔的 API 和豐富的功能。

## 安裝

需要 Go 1.25 或更新版本。

```bash
go get github.com/vincent119/zlogger
```

## 基本使用

```go
import (
	"log"

	"github.com/vincent119/zlogger"
)

func main() {
	cleanup, err := zlogger.Configure(nil)
	if err != nil {
		log.Fatalf("初始化 logger 失敗：%v", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			log.Printf("關閉 logger 失敗：%v", err)
		}
	}()

	zlogger.Info("服務啟動", zlogger.String("key", "value"))
	zlogger.Debug("除錯訊息", zlogger.Int("count", 42))

	if err := zlogger.Sync(); err != nil {
		log.Printf("同步 logger 失敗：%v", err)
	}
}
```

## 自定義配置

```go
level := "debug"
format := "json"
outputs := []string{"console", "file"}
logPath := "./logs"
fileName := "app.log"
development := true

cleanup, err := zlogger.Configure(&zlogger.ConfigPatch{
	Level:       &level,
	Format:      &format,
	Outputs:     &outputs,
	LogPath:     &logPath,
	FileName:    &fileName,
	Development: &development,
})
if err != nil {
	return fmt.Errorf("設定 logger: %w", err)
}
defer func() {
	if err := cleanup(); err != nil {
		log.Printf("關閉 logger 失敗：%v", err)
	}
}()
```

`ConfigPatch` 以 pointer 區分「未提供」與明確零值。例如只提供 `Level` 時，
`AddCaller=true` 與 `ColorEnabled=true` 會保留；明確提供 `false` 時才會關閉。

若不需要全域 logger，可直接持有具名生命週期：

```go
instance, err := zlogger.New(zlogger.DefaultConfig())
if err != nil {
	return fmt.Errorf("建立 logger: %w", err)
}
defer func() {
	if err := instance.Close(); err != nil {
		log.Printf("關閉 logger 失敗：%v", err)
	}
}()

instance.Logger().Info("服務啟動")
if err := instance.Sync(); err != nil {
	return fmt.Errorf("同步 logger: %w", err)
}
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
  color_enabled: true
```

```go
type AppConfig struct {
	Log zlogger.ConfigPatch `yaml:"log"`
}

// 載入設定後
var appConfig AppConfig
// ... 載入 YAML ...
cleanup, err := zlogger.Configure(&appConfig.Log)
if err != nil {
	return fmt.Errorf("設定 logger: %w", err)
}
defer func() {
	if err := cleanup(); err != nil {
		log.Printf("關閉 logger 失敗：%v", err)
	}
}()
```

`Configure` 會先完成嚴格驗證，再建立任何目錄或檔案。未知 Level、Format、
Output、重複 Output，以及 file output 明確使用空白 LogPath 都會回傳
`ErrInvalidConfig`。初始化失敗不會發布半成品，修正設定後可再次呼叫。

既有 `Init(*Config)` 保留來源碼相容，但無法回傳設定或 I/O 錯誤，且
`Config` 的 bool 無法區分未提供與 `false`。新程式應使用 `Configure`。

## 檔案輸出安全

`LogPath` 與 SplitOutput 的 `directory` 是呼叫端信任並選定的基準目錄；
`FileName` 與 `filePrefix` 只允許單一 leaf name，不得包含以下內容：

- `.`、`..`
- `/` 或 `\` 路徑分隔符
- 絕對路徑或 Windows drive prefix
- NUL

不安全名稱會回傳可由 `errors.Is(err, zlogger.ErrUnsafeLogPath)` 判斷的錯誤。
Config 驗證錯誤同時保留 `ErrInvalidConfig` 分類。

新建立的日誌目錄使用 `0700`，新建立的日誌檔使用 `0600`。實際權限可能
被 umask 進一步收緊；已存在的目錄或檔案不會被主動 chmod。

需要讓同一 Unix group 讀取日誌時，可透過 functional options 明確放寬
新建物件的權限：

```go
instance, err := zlogger.NewWithOptions(
	cfg,
	zlogger.WithDirPerm(0o750),
	zlogger.WithFilePerm(0o640),
)
if err != nil {
	return err
}
defer func() {
	_ = instance.Close()
}()
```

分級輸出使用相同 options，且每日換檔會沿用解析後的 file mode：

```go
core, cleanup, err := zlogger.GetSplitCoreWithOptions(
	"./logs",
	"app",
	encoderConfig,
	zlogger.WithDirPerm(0o750),
	zlogger.WithFilePerm(0o640),
)
```

目錄 mode 必須包含 owner `rwx`，檔案 mode 必須包含 owner `rw`；兩者皆
不得包含非 permission bits 或 other-write。無效設定會回傳可由
`errors.Is(err, zlogger.ErrInvalidFilePermission)` 判斷的錯誤，且不會建立
目錄或檔案。同類 option 重複提供時以最後一個為準。

Options 只影響新建立的物件，不能繞過 umask，也不會修改既有權限。
Windows 可使用相同 API，但不保證具有 POSIX mode 的可觀察語意。放寬
group／other 讀取權限前，呼叫端必須確認日誌不含不應共享的敏感資料。

每批日誌檔案會先以 `os.OpenRoot` 開啟可信任的 base directory，再透過
root-relative leaf 執行 `Lstat` 與 `OpenFile`。穩定存在的最終 symlink 仍會
被拒絕；若 leaf 在檢查後被並行替換，`os.Root` 會阻止解析結果逸出 root。

此 containment 不是完整 filesystem sandbox。`OpenRoot` 會跟隨 base path
本身的 symlink，且不阻止 mount boundary、bind mount、特殊裝置或惡意
filesystem；呼叫端仍須保護 base directory 與設定來源。競態中替換成指向
root 內部的 symlink 可能被跟隨，因此不承諾原子拒絕所有 symlink。Go 的
`js`、`plan9` 與 `wasip1` 平台另有標準庫限制；本專案的跨平台 CI 驗收範圍
為 Linux、macOS 與 Windows。

### 敏感資訊

日誌欄位應採白名單，只記錄診斷所需的最小資料。禁止直接記錄：

- token、API key、密碼與私鑰
- `Authorization`、cookie 與 session identifier
- 身分證號、金融資料、完整地址及其他完整個資
- 含秘密欄位的完整 request、response、Config 或任意 struct

需保留欄位存在性時，使用不接收原始秘密值的 `Redacted`：

```go
zlogger.Info("驗證請求",
	zlogger.Redacted("authorization"),
	zlogger.String("request_id", requestID),
)
```

`Redacted` 只輸出固定 `[REDACTED]`，不會自動掃描或遮罩其他欄位。

### JSON 範例

```json
{
  "log": {
    "level": "info",
    "format": "json",
    "outputs": ["console"],
    "log_path": "./logs",
    "add_caller": true,
    "color_enabled": true
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

### 使用方式

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

    // 方式三：直接使用 zlogger 的 Context 函數記錄日誌
    zlogger.InfoContext(c.Request.Context(), "NumberCheck",
    zlogger.String("category", "clicksend"),
    zlogger.String("function", "bounce"),
    zlogger.String("data", "11111"),
    )
    middleware.skipMiddlewareLog(c) // 跳過中間件日誌

    c.JSON(200, gin.H{
        "Status": "OK", "recv_time": fmt.Sprint(time.Now().Format("2006-01-02T15:04:05")),
    })

}


```

## 按級別分離日誌檔案

```go
import (
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

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
logger := zap.New(core)
defer func() {
    _ = logger.Sync()
    cleanup()
}()

// 會產生以下檔案：
// - logs/app-info-2024-01-01.log
// - logs/app-warn-2024-01-01.log
// - logs/app-error-2024-01-01.log
```

級別路由規則：

- DEBUG、INFO 寫入 info 檔
- WARN 寫入 warn 檔
- ERROR、DPANIC、PANIC、FATAL 寫入 error 檔

`cleanup` 會等待每日換檔 goroutine 結束並關閉全部檔案。logger 不再使用時，應先呼叫 `Sync`，再執行 `cleanup`；換檔期間若新檔案無法完整開啟，現有檔案會保留並繼續提供寫入。

如需自訂新建目錄與檔案權限，改用 `GetSplitCoreWithOptions` 並傳入
`WithDirPerm`、`WithFilePerm`。既有 `GetSplitCore` 保持 `0700`／`0600`
安全預設。

## Log Rotation（使用 timberjack）

`SplitOutput` 僅提供每日換檔，不包含大小限制、備份數量與壓縮等完整 log rotation 功能。需要這些能力時，建議使用 [timberjack](https://github.com/DeRuina/timberjack) 處理：

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
| `color_enabled`  | bool     | `true`        | 是否啟用顏色輸出（僅 console 格式有效）   |

### 顏色輸出

當 `color_enabled` 為 `true` 且 `format` 為 `console` 時，不同日誌級別會以不同顏色顯示：

| 級別        | 顏色   |
| ----------- | ------ |
| DEBUG       | 洋紅色 |
| INFO        | 藍色   |
| WARN        | 黃色   |
| ERROR/FATAL | 紅色   |

```go
// 未提供 ColorEnabled 時會使用預設 true；以下範例明確禁用顏色。
disabled := false
cleanup, err := zlogger.Configure(&zlogger.ConfigPatch{
	ColorEnabled: &disabled,
})
```

> **注意：** Log rotation（檔案大小限制、備份、壓縮）請使用 timberjack，參考上方範例。

## 開發與品質驗證

本機完整驗證入口：

```bash
make verify
```

主要子命令：

```bash
make test-race       # race detector 測試
make lint            # golangci-lint v2.12.2
make vuln            # govulncheck v1.6.0 可達漏洞掃描
make coverage-check  # 總覆蓋率不得低於 90%
make bench           # logger 關鍵路徑 benchmark
make fmt-check       # 驗證格式但不修改檔案
```

漏洞掃描使用固定版本 scanner 與 Go 官方即時資料庫：

```bash
go install golang.org/x/vuln/cmd/govulncheck@v1.6.0
make vuln
```

`make vuln` 會先驗證 scanner 版本，再查詢 `https://vuln.go.dev`。發現可達漏洞、
版本不符、網路或資料庫錯誤都會回傳失敗；由於依賴外部服務，此 target 不包含在
`make verify`。CI 會以 Go 1.25.12 與 Go 1.26.5 分別執行掃描。

CI 在 Linux 使用 Go 1.25.12 與 Go 1.26.5 執行 race 測試，並使用 Go 1.26.5
執行 macOS 15、Windows 2025 相容性測試。GitHub Actions 均釘選完整 commit
SHA，workflow token 僅具 `contents: read` 權限。coverage job 通過 90% 門檻後，
會將 `coverage.out` 上傳至 Codecov，供 README badge 與歷史趨勢使用。

Dependabot 於每週一 Asia/Taipei 09:00 檢查 Go modules，09:30 檢查 GitHub
Actions。每個 ecosystem 同時最多提出 2 個 version update PR；minor／patch 會分組，
major 與 security update 維持獨立人工審查，不會自動合併。

## License

MIT
