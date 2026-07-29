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

### 2.1 完整設定與部分設定

```go
type Config struct {
    Level         string   `json:"level" yaml:"level" toml:"level"`
    Format        string   `json:"format" yaml:"format" toml:"format"`
    // ...
}

type ConfigPatch struct {
    Level         *string   `json:"level,omitempty" yaml:"level,omitempty" toml:"level,omitempty"`
    Format        *string   `json:"format,omitempty" yaml:"format,omitempty" toml:"format,omitempty"`
    Outputs       *[]string `json:"outputs,omitempty" yaml:"outputs,omitempty" toml:"outputs,omitempty"`
    ColorEnabled  *bool     `json:"color_enabled,omitempty" yaml:"color_enabled,omitempty" toml:"color_enabled,omitempty"`
    // ...
}
```

**設計要點：**

1. **多格式標籤支援**

   - 同時支援 JSON、YAML、TOML 標籤
   - 允許從任何格式的設定檔直接綁定
   - 無需額外轉換層

2. **三態部分設定**

   - `ConfigPatch` 以 pointer 區分未提供與明確零值
   - `Resolve()` 以 `DefaultConfig()` 為基底，正規化並嚴格驗證
   - Outputs 在輸入與輸出邊界複製，避免共享可變 slice
   - 既有 `Config.Merge()` 只保留來源碼相容，新程式不應使用

3. **預設配置**
   - `DefaultConfig()` 提供合理的預設值
   - 即使傳入 `nil` 也能正常運作

**使用範例：**

```go
// 方式 1：使用預設設定
cleanup, err := zlogger.Configure(nil)

// 方式 2：部分自定義
level := "debug"
cleanup, err := zlogger.Configure(&zlogger.ConfigPatch{Level: &level})

// 方式 3：從 YAML 綁定
type AppConfig struct {
    Log zlogger.ConfigPatch `yaml:"log"`
}
cleanup, err := zlogger.Configure(&appConfig.Log)
```

---

## 3. 初始化設計

### 3.1 全域設定狀態

```go
var (
    globalLogger *zap.Logger
    zapGlobalLevel = zap.NewAtomicLevel()
    configureMu sync.Mutex
    configured bool
)
```

**設計要點：**

1. **成功後只設定一次**

   - mutex 保護設定與發布狀態
   - 建構失敗後回到未設定狀態，可修正後重試
   - 成功後再次設定回傳 `ErrAlreadyConfigured`

2. **全局 Logger 實例**

   - 提供全局函數（`Info()`, `Error()` 等）
   - 無需傳遞 logger 實例

3. **動態級別調整**
   - 使用 `zap.NewAtomicLevel()` 支援運行時調整
   - `SetLevel()` 可動態修改日誌級別

### 3.2 初始化流程

```bash
Configure(patch)
  ├─> ConfigPatch.Resolve()         # 套用預設、正規化、驗證
  ├─> New(cfg)                      # 建立 Instance，不修改全域狀態
  │   ├─> buildConsoleCore()        # 建立控制台輸出
  │   ├─> buildFileCore()           # 建立檔案輸出與 owned resources
  │   └─> zapcore.NewTee()          # 合併多個輸出
  ├─> publish                       # 完整成功後才發布全域 logger
  └─> cleanup                       # 回復 zap globals 並關閉 owned resources
```

**設計特點：**

- 支援多輸出（console + file）
- 使用 `zapcore.NewTee()` 同時輸出到多個目標
- 自動建立日誌目錄
- 檔案名稱支援日期格式
- 設定與 I/O 錯誤直接回傳，不在安全入口 panic
- `Instance.Close()` 可重複及並行呼叫
- 呼叫端應先執行 `Sync()`，再執行 cleanup

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

4. **欄位 ownership 隔離**

   - `WithContext` 在寫入前防禦性複製 `[]Field`，呼叫端後續修改輸入 slice 不會污染 context
   - `FromContext` 回傳 `[]Field` 的淺層副本，修改回傳 slice 不會改變 context 內部欄位
   - package 內部透過唯讀 accessor 合併欄位，避免公開 clone 後再次複製
   - 淺層複製不會複製 `Field.Interface` 內的 map、slice、pointer 或其他參照物件；其並行安全仍由呼叫端負責

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
zlogger.Redacted("authorization") // 固定輸出 [REDACTED]
```

`Redacted(key)` 不接收秘密值，也不依欄位名稱自動猜測或遮罩敏感資料。

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

**設計目的：** 將不同級別的日誌寫入不同檔案（按級別分離，非 log rotation）

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
       return lvl == zapcore.DebugLevel || lvl == zapcore.InfoLevel
   })
   ```

   - DEBUG、INFO 寫入 info 檔
   - WARN 寫入 warn 檔
   - ERROR、DPANIC、PANIC、FATAL 寫入 error 檔

2. **每日自動換檔**

   - 每天零點自動切換到新檔案
   - 使用可停止的 timer 與 goroutine 等待下一個換檔時間
   - `Close` 會停止 timer、等待 goroutine 結束，再關閉檔案
   - 新檔案未完整開啟時保留既有檔案，避免換檔失敗中斷寫入
   - 注意：這是按日期換檔，不是按大小的 rotation

3. **並行安全與同步**
   - 使用 `sync.Mutex` 保護檔案操作
   - `Close` 可重複及並行呼叫，底層資源只關閉一次
   - `Sync` 會將三個目前開啟的檔案同步至儲存裝置
   - 關閉後的 Write 與 Sync 會回傳包裝 `os.ErrClosed` 的錯誤

### 7.2 與 timberjack 的差異

| 功能 | split_output.go | timberjack |
|------|-----------------|------------|
| 按級別分離 | 是，info/warn/error 分開檔案 | 否，單一檔案 |
| 每日換檔 | 是，每天零點切換 | 是，RotateAt 支援 |
| 大小限制 | 無 | MaxSize |
| 時間間隔 | 無 | RotationInterval |
| 備份數量 | 無 | MaxBackups |
| 保存天數 | 無 | MaxAge |
| 壓縮備份 | 無 | gzip/zstd |

**使用建議：**

- 需要按級別分離 → 使用 `GetSplitCore()`
- 需要大小限制/壓縮/時間輪轉 → 使用 timberjack
- 兩者可結合使用

---

## 8. Log Rotation 設計

### 8.1 設計決策

**zlogger 不內建完整 log rotation**，`SplitOutput` 只負責每日換檔。大小限制、備份與壓縮交由外部元件，理由如下：

1. **保持 lib 輕量** - 只依賴 zap
2. **避免強制依賴** - 不是所有專案都需要
3. **使用者自由選擇** - 可選 timberjack、系統 logrotate 等

### 8.2 建議方案

使用 [timberjack](https://github.com/DeRuina/timberjack)：

```go
import "github.com/DeRuina/timberjack"

tjLogger := &timberjack.Logger{
    Filename:   "./logs/app.log",
    MaxSize:    100,   // MB
    MaxBackups: 10,
    MaxAge:     30,    // days
    Compress:   true,  // gzip 壓縮
}
```

詳細範例請參考 README.md。

---

## 9. 錯誤處理設計

### 9.1 初始化錯誤

```go
instance, err := zlogger.New(cfg)
if err != nil {
    return fmt.Errorf("建立 logger: %w", err)
}
defer instance.Close()
```

**設計決策：** 安全入口回傳可分類錯誤，完整成功後才發布 logger

**理由：**

- 安全入口回傳設定與 I/O 錯誤，由應用程式啟動邊界決定是否終止
- 建構失敗不發布半初始化 logger，並回收已建立資源
- 既有 `Init()` 只作 deprecated 相容層，其他錯誤維持 legacy panic 行為

### 9.2 運行時錯誤

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

### 9.3 檔案輸出安全邊界

- `LogPath`／`directory` 是呼叫端信任的 base directory。
- `FileName`／`filePrefix` 是不可信任 leaf，不得包含 `.`、`..`、`/`、`\`、NUL、絕對路徑或 Windows drive prefix。
- 路徑錯誤以 `ErrUnsafeLogPath` 分類；Config 同時保留 `ErrInvalidConfig`。
- 新目錄 mode 為 `0700`，新檔 mode 為 `0600`；umask 可進一步收緊。
- 不對既有目錄或檔案執行 chmod，避免改變共享資源權限。
- 每批檔案先以 `os.OpenRoot` 開啟可信任 base，再以 root-relative leaf 執行 `Root.Lstat` 與 `Root.OpenFile`；SplitOutput 同批三檔共用單一 root。
- 最終目標穩定存在為 symlink 時拒絕開啟，不跟隨或覆寫；若在檢查後並行替換，`Root.OpenFile` 保證解析結果不逸出 root。
- `os.Root` 不等同 filesystem sandbox：`OpenRoot` 會跟隨 base path symlink，且不防 mount boundary、bind mount、特殊裝置或惡意 filesystem。競態中的 root 內 symlink 可能被跟隨，不承諾原子拒絕所有 symlink。
- Go `js`、`plan9`、`wasip1` 另有標準庫限制；目前跨平台 CI 契約為 Linux、macOS 與 Windows。
- 敏感欄位採白名單；token、密碼、Authorization、cookie 與完整個資不得進入日誌。
- `Redacted` 需由呼叫端主動使用，不提供自動 redaction。

---

## 10. 擴展性設計

### 10.1 類型別名導出

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

### 10.2 選項模式支援

```go
func WithOptions(opts ...zap.Option) *Logger
```

**設計目的：**

- 允許使用者添加自定義 zap.Option
- 保持與 zap 生態系統的相容性

---

## 11. 性能考量

### 11.1 零分配設計

- Field 函數直接轉發到 zap，無額外分配
- Context 字段合併使用 `make()` 預分配容量
- 避免不必要的字串操作

### 11.2 受控初始化

- 使用 mutex 狀態確保只成功初始化一次，且失敗後可重試
- 全局 logger 使用指針，避免複製開銷

---

## 12. 設計原則總結

### 12.1 簡潔性

- **單一 Package**：所有功能在 `zlogger` 下
- **統一命名**：函數名稱清晰一致
- **零配置可用**：`Configure(nil)` 即可使用，並回傳 error 與 cleanup

### 12.2 靈活性

- **多格式配置**：支援 JSON/YAML/TOML
- **多輸出支援**：console + file
- **動態調整**：運行時修改級別

### 12.3 易用性

- **Context 自動合併**：無需手動處理
- **Field 輔助函數**：簡化常用操作
- **類型別名**：與 zap 完全相容

### 12.4 可擴展性

- **GetLogger()**：提供原始 zap.Logger
- **WithOptions()**：支援自定義選項
- **SplitOutput**：支援進階輸出需求

---

## 13. 與原生 zap 的差異

| 特性     | zap          | zlogger                     |
| -------- | ------------ | --------------------------- |
| 初始化   | 需要手動配置 | `Configure(patch)` 安全初始化 |
| 配置     | 程式碼配置   | 支援設定檔綁定              |
| Context  | 需手動處理   | 自動合併 context 字段       |
| 全局函數 | 無           | 提供 `Info()`, `Error()` 等 |
| SQL 處理 | 無           | 自動清理 SQL 轉義字符       |
| 分離輸出 | 需自行實現   | 提供 `GetSplitCore()`       |

**設計目標：** 在保持 zap 性能的同時，提供更簡潔的 API 和更豐富的功能。

---

## 14. 工具鏈與 CI 品質基線

### 14.1 Go 支援政策

- `go.mod` 最低版本為 Go 1.25.0，不加入 `toolchain` directive。
- CI 最低版本固定為 Go 1.25.11，現行版本固定為 Go 1.26.5。
- 新的 security patch 由獨立 PR 更新精確版本，不使用 `stable` 或浮動版本。
- 提高最低 minor 版本屬 build-time 相容性變更，必須在 README 與 PR 說明。

### 14.2 CI 分工

| Job | Runner | 驗證 |
|-----|--------|------|
| Race | Ubuntu 24.04 | Go 1.25.11、1.26.5 執行 `-race` |
| Portability | macOS 15、Windows 2025 | Go 1.26.5 一般測試 |
| Lint | Ubuntu 24.04 | golangci-lint v2.12.2、go vet、格式 |
| Coverage | Ubuntu 24.04 | atomic coverage 不低於 90% |
| Benchmark | Ubuntu 24.04 | logger benchmark smoke test |

所有外部 GitHub Actions 使用官方 tag 對應的完整 40 字元 commit SHA，workflow
token 只有 `contents: read`。品質判定不依賴 Codecov 或 repository secret。

### 14.3 本機驗證與 benchmark

`make verify` 是本機與 CI 共用的非產品修改型驗證入口，包含格式、vet、lint、
race、coverage 與 benchmark。lint 工具缺少或版本不符時必須失敗，不得靜默跳過。

關鍵路徑 benchmark 使用 `io.Discard`，不碰網路、stdout 或檔案系統：

- `BenchmarkLoggerInfoDisabled`：量測未啟用 level 的呼叫成本。
- `BenchmarkLoggerInfoFields`：量測 JSON 結構化欄位寫入成本。

CI 只確認 benchmark 可執行；工具鏈或程式碼升級的效能比較需在同一硬體各執行
五次，再以固定版本 benchstat 比較，避免共享 runner 雜訊形成錯誤閘門。
