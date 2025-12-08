# zlogger 設計方法說明

## 整體架構設計

### 1. 單一 Package 設計

**設計理念：** 將所有功能整合在單一 `zlogger` package 中，避免使用者需要 import 多個子 package。

```bash
zlogger/
├── config.go      # 配置結構
├── core.go        # 核心初始化與日誌函數
├── context.go     # Context 支援
├── fields.go      # Field 輔助函數
├── encoder.go     # 自定義編碼器
├── split_output.go # 分離輸出功能
└── zlogger.go     # 對外接口與類型別名
```

**優點：**

- 簡化 import：只需 `import "github.com/vincent119/zlogger"`
- 統一命名空間：所有函數都在 `zlogger` 下
- 降低學習成本：不需要記住多個 package 名稱

---

## 2. 配置系統設計

### 2.1 配置結構設計

```go
type Config struct {
    Level         string   `json:"level" yaml:"level" toml:"level"`
    Format        string   `json:"format" yaml:"format" toml:"format"`
    // ...
}
```

**設計要點：**

1. **多格式標籤支援**

   - 同時支援 JSON、YAML、TOML 標籤
   - 允許從任何格式的設定檔直接綁定
   - 無需額外轉換層

2. **零值友好設計**

   - 使用 `Merge()` 方法合併配置
   - 零值不覆蓋預設值（string 空字串、int 0、slice nil）
   - bool 類型直接覆蓋（因為 false 也是有效值）

3. **預設配置**
   - `DefaultConfig()` 提供合理的預設值
   - 即使傳入 `nil` 也能正常運作

**使用範例：**

```go
// 方式 1: 使用預設配置
zlogger.Init(nil)

// 方式 2: 部分自定義
cfg := &zlogger.Config{Level: "debug"}
zlogger.Init(cfg)

// 方式 3: 從 YAML 綁定
type AppConfig struct {
    Log zlogger.Config `yaml:"log"`
}
zlogger.Init(&appConfig.Log)
```

---

## 3. 初始化設計

### 3.1 單例模式

```go
var (
    globalLogger *zap.Logger
    once         sync.Once
    zapGlobalLevel = zap.NewAtomicLevel()
)
```

**設計要點：**

1. **sync.Once 保證只初始化一次**

   - 避免重複初始化造成資源浪費
   - 線程安全

2. **全局 Logger 實例**

   - 提供全局函數（`Info()`, `Error()` 等）
   - 無需傳遞 logger 實例

3. **動態級別調整**
   - 使用 `zap.NewAtomicLevel()` 支援運行時調整
   - `SetLevel()` 可動態修改日誌級別

### 3.2 初始化流程

```bash
Init(cfg)
  └─> once.Do(initLogger)
      ├─> DefaultConfig().Merge(cfg)  # 合併配置
      ├─> parseLevel()                # 解析級別
      ├─> buildConsoleCore()          # 建立控制台輸出
      ├─> buildFileCore()             # 建立檔案輸出
      ├─> zapcore.NewTee()           # 合併多個輸出
      └─> zap.ReplaceGlobals()        # 替換全局 logger
```

**設計特點：**

- 支援多輸出（console + file）
- 使用 `zapcore.NewTee()` 同時輸出到多個目標
- 自動建立日誌目錄
- 檔案名稱支援日期格式

---

## 4. Context 支援設計

### 4.1 Context 字段存儲

```go
type contextKey string
const loggerContextKey = contextKey("zlogger_fields")
```

**設計要點：**

1. **類型安全的 Context Key**

   - 使用自定義類型避免 key 衝突
   - 不導出 key，防止外部直接存取

2. **字段累積機制**

   ```go
   ctx := zlogger.WithRequestID(ctx, "req-123")
   ctx = zlogger.WithUserID(ctx, 12345)
   // 所有字段都會累積在 context 中
   ```

3. **自動合併**
   - `*Context()` 函數自動合併 context 中的字段
   - 無需手動提取和合併

### 4.2 使用場景

**HTTP 請求追蹤：**

```go
// Middleware 中
ctx := zlogger.WithRequestID(c.Request.Context(), requestID)

// Handler 中
zlogger.InfoContext(ctx, "處理請求")  // 自動帶入 request_id
```

**服務層追蹤：**

```go
ctx = zlogger.WithUserID(ctx, userID)
ctx = zlogger.WithOperation(ctx, "login")
zlogger.InfoContext(ctx, "登入成功")  // 自動帶入所有追蹤資訊
```

---

## 5. Field 輔助函數設計

### 5.1 設計理念

**問題：** zap 的 Field 函數在 `zap.String()`、`zap.Int()` 等，使用時需要 import zap。

**解決方案：** 提供包裝函數，統一在 `zlogger` 命名空間下。

```go
// 使用者不需要 import zap
zlogger.String("key", "value")
zlogger.Int("count", 42)
zlogger.Err(err)
```

### 5.2 類型別名

```go
type Field = zap.Field
```

**優點：**

- 與 zap.Field 完全相容
- 使用者可以混用 `zap.String()` 和 `zlogger.String()`
- 不增加額外開銷（編譯時展開）

---

## 6. SQL 處理設計

### 6.1 sqlProcessingCore 包裝器

```go
type sqlProcessingCore struct {
    zapcore.Core
}
```

**設計目的：**

- 自動處理 SQL 字串中的轉義字符
- 清理多餘的反斜線
- 改善日誌可讀性

**處理流程：**

```bash
Write() 被調用
  └─> 檢查 field.Key == "sql"
      └─> processSQLString() 處理轉義字符
          ├─> 移除 "\\\\" → "\"
          ├─> 移除 "\\\"" → "\""
          └─> 移除 "\\'" → "'"
```

**使用範例：**

```go
zlogger.Info("執行 SQL", zlogger.String("sql", "SELECT * FROM users"))
// 自動清理 SQL 中的轉義字符
```

---

## 7. 分離輸出設計

### 7.1 SplitOutput 結構

**設計目的：** 將不同級別的日誌寫入不同檔案

```bash
logs/
├── app-info-2024-01-01.log
├── app-warn-2024-01-01.log
└── app-error-2024-01-01.log
```

**設計要點：**

1. **級別過濾**

   ```go
   infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
       return lvl == zapcore.InfoLevel
   })
   ```

2. **自動輪轉**

   - 每天零點自動切換檔案
   - 使用 goroutine 定期檢查

3. **線程安全**
   - 使用 `sync.Mutex` 保護檔案操作
   - 確保併發安全

---

## 8. 錯誤處理設計

### 8.1 初始化錯誤

```go
if err := os.MkdirAll(logDir, 0755); err != nil {
    panic("無法建立日誌目錄: " + err.Error())
}
```

**設計決策：** 使用 `panic` 而非返回錯誤

**理由：**

- 日誌系統是基礎設施，初始化失敗應立即停止程式
- 避免程式在沒有日誌的情況下運行
- 簡化 API（`Init()` 無需返回錯誤）

### 8.2 運行時錯誤

```go
func Info(msg string, fields ...Field) {
    if globalLogger != nil {
        globalLogger.Info(msg, fields...)
    }
}
```

**設計決策：** 空檢查，靜默失敗

**理由：**

- 避免 nil pointer panic
- 允許在未初始化時調用（雖然不會輸出）
- 提高容錯性

---

## 9. 擴展性設計

### 9.1 類型別名導出

```go
type (
    Logger = zap.Logger
    Level  = zapcore.Level
    // ...
)
```

**設計目的：**

- 允許進階使用者直接使用 zap 的功能
- 提供 `GetLogger()` 返回原始 zap.Logger
- 不限制使用者的使用方式

### 9.2 選項模式支援

```go
func WithOptions(opts ...zap.Option) *Logger
```

**設計目的：**

- 允許使用者添加自定義 zap.Option
- 保持與 zap 生態系統的相容性

---

## 10. 性能考量

### 10.1 零分配設計

- Field 函數直接轉發到 zap，無額外分配
- Context 字段合併使用 `make()` 預分配容量
- 避免不必要的字串操作

### 10.2 延遲初始化

- 使用 `sync.Once` 確保只初始化一次
- 全局 logger 使用指針，避免複製開銷

---

## 11. 設計原則總結

### 11.1 簡潔性

- **單一 Package**：所有功能在 `zlogger` 下
- **統一命名**：函數名稱清晰一致
- **零配置可用**：`Init(nil)` 即可使用

### 11.2 靈活性

- **多格式配置**：支援 JSON/YAML/TOML
- **多輸出支援**：console + file
- **動態調整**：運行時修改級別

### 11.3 易用性

- **Context 自動合併**：無需手動處理
- **Field 輔助函數**：簡化常用操作
- **類型別名**：與 zap 完全相容

### 11.4 可擴展性

- **GetLogger()**：提供原始 zap.Logger
- **WithOptions()**：支援自定義選項
- **SplitOutput**：支援進階輸出需求

---

## 12. 與原生 zap 的差異

| 特性     | zap          | zlogger                     |
| -------- | ------------ | --------------------------- |
| 初始化   | 需要手動配置 | `Init(cfg)` 一鍵初始化      |
| 配置     | 程式碼配置   | 支援設定檔綁定              |
| Context  | 需手動處理   | 自動合併 context 字段       |
| 全局函數 | 無           | 提供 `Info()`, `Error()` 等 |
| SQL 處理 | 無           | 自動清理 SQL 轉義字符       |
| 分離輸出 | 需自行實現   | 提供 `GetSplitCore()`       |

**設計目標：** 在保持 zap 性能的同時，提供更簡潔的 API 和更豐富的功能。
