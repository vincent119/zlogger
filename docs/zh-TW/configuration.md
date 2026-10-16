# 設定

**繁體中文（台灣）** | [English](../en/configuration.md)

[返回文件首頁](README.md)

## 需要部分覆寫時，用 ConfigPatch

`ConfigPatch` 的每個欄位都是指標。nil 代表沒提供，非 nil 就使用該值，
包含明確設定的 false、空字串或空 slice。先區分這兩種情況，才不會把「關掉功能」誤當成「使用預設值」。

`Resolve` 會從 `DefaultConfig` 補齊缺少的值，複製 outputs，
把 level、format、outputs 轉成小寫，再驗證結果。

`Config` 則是完整設定，可由 `DefaultConfig()` 開始修改。
舊 `Config.Merge` 無法區分未提供的 bool 和 false，已棄用。

以下片段把 level 設成 debug、格式改為 JSON，並同時開啟終端機與檔案輸出。
取得 cleanup 後，要在關機流程先 Sync 再執行清理，詳見[生命週期](lifecycle.md)。

```go
level := "debug"
format := "json"
outputs := []string{"console", "file"}
colorEnabled := false

cleanup, err := zlogger.Configure(&zlogger.ConfigPatch{
	Level:        &level,
	Format:       &format,
	Outputs:      &outputs,
	ColorEnabled: &colorEnabled,
})
```

## 設定檔由應用程式載入

zlogger 不讀 YAML、JSON、TOML 或環境變數，也不決定哪個來源優先。
應用程式把資料解析成 `ConfigPatch` 後，再交給 zlogger 補預設值、驗證並初始化。

`ConfigPatch` 有 json、yaml、toml、mapstructure 標籤，方便搭配你選的解析器。
要拒絕未知 key，請開啟解析器的嚴格模式。
解析器已經忽略的拼字錯誤，`Validate` 無法找回來。

```yaml
log:
  level: debug
  format: json
  outputs: [console, file]
  log_path: ./logs
  file_name: app.log
  add_caller: true
  add_stacktrace: false
  development: false
  color_enabled: false
```

以下片段使用標準庫解析 JSON，最多讀取 1 MiB，並拒絕未知欄位。
它不是完整的檔案大小與尾端資料驗證器，正式應用仍須依輸入需求補齊檢查。
將片段放在可回傳 error 的函式中，並匯入 encoding/json、fmt、io、os 與 zlogger。

```go
type AppConfig struct {
	Log zlogger.ConfigPatch `json:"log"`
}

file, err := os.Open("config.json")
if err != nil {
	return fmt.Errorf("開啟設定檔: %w", err)
}
defer func() { _ = file.Close() }()

var appConfig AppConfig
decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
decoder.DisallowUnknownFields()
if err := decoder.Decode(&appConfig); err != nil {
	return fmt.Errorf("解析設定檔: %w", err)
}

cleanup, err := zlogger.Configure(&appConfig.Log)
if err != nil {
	return fmt.Errorf("設定 logger: %w", err)
}
defer func() { _ = cleanup() }()
```

## 預設值與合法設定

| key | 型別 | 預設值 | 合法值或條件 |
| --- | --- | --- | --- |
| `level` | string | `info` | `debug`、`info`、`warn`、`error`、`fatal`；不分大小寫 |
| `format` | string | `console` | `console`、`json`；不分大小寫 |
| `outputs` | []string | `[console]` | `console`、`file`；至少一項、不得重複 |
| `log_path` | string | `./logs` | 啟用 `file` 時不可為空 |
| `file_name` | string | 空字串 | 只能是單一檔名，空字串使用日期命名 |
| `add_caller` | bool | `true` | 加入呼叫位置 |
| `add_stacktrace` | bool | `false` | 加入 ERROR 以上的堆疊資訊 |
| `development` | bool | `false` | zap development mode |
| `color_enabled` | bool | `true` | 僅 console format 產生 ANSI 色碼 |

無效值可由 `errors.Is(err, zlogger.ErrInvalidConfig)` 判斷。file output 的不安全
`file_name` 同時保留 `ErrInvalidConfig` 與 `ErrUnsafeLogPath`。decoder 與檔案 I/O 錯誤
不會被包裝成 `ErrInvalidConfig`。

## 顏色只影響終端機格式

只有 `format=console` 且 `color_enabled=true` 會輸出 ANSI 色碼。JSON level 永遠不含
ANSI，即使 `color_enabled` 保持 true。

檔案建立權限不是設定檔欄位；請參閱[安全性](security.md)與 `ConfigureWithOptions`。

`RedactionConfig` 也是獨立設定，只交給 `NewRedactingCore`，不是 ConfigPatch 的欄位。
需要遮罩時請看[安全性範例](security.md)。
