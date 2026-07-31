# 設定

**繁體中文** | [English](../en/configuration.md)

[返回文件首頁](README.md)

## 設定模型

新程式應使用 `ConfigPatch`。每個欄位都是 pointer：nil 表示未提供，非 nil 表示明確
覆寫，包含 `false`、空字串或空 slice。`Resolve` 會從 `DefaultConfig` 補齊未提供值，
複製 `outputs`，將 level、format、outputs 正規化為小寫，再執行驗證。

`Config` 是完整執行期設定。`Config.Merge` 因 bool 無法區分未提供與 `false`，已
deprecated。

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

## 設定來源責任

zlogger 不讀取 YAML、JSON、TOML 或環境變數，也不定義來源優先級。呼叫端負責解析
外部資料為 `ConfigPatch`；zlogger 只負責 resolve、normalize、validate 與初始化。

`ConfigPatch` 提供 `json`、`yaml`、`toml`、`mapstructure` tags，但不代表內建 loader。
未知 key 必須由 decoder 的嚴格模式拒絕；一旦 decoder 忽略該 key，`Validate` 無法還原。

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

以下 JSON loader 使用標準庫、限制輸入大小並拒絕未知欄位：

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

## 欄位契約

| key | 型別 | 預設值 | 合法值或條件 |
| --- | --- | --- | --- |
| `level` | string | `info` | `debug`、`info`、`warn`、`error`、`fatal`；不分大小寫 |
| `format` | string | `console` | `console`、`json`；不分大小寫 |
| `outputs` | []string | `[console]` | `console`、`file`；至少一項、不得重複 |
| `log_path` | string | `./logs` | 啟用 `file` 時不可為空 |
| `file_name` | string | 空字串 | 安全 leaf name；空字串使用日期命名 |
| `add_caller` | bool | `true` | 加入 caller |
| `add_stacktrace` | bool | `false` | 加入 ERROR 以上 stacktrace |
| `development` | bool | `false` | zap development mode |
| `color_enabled` | bool | `true` | 僅 console format 產生 ANSI 色碼 |

無效值可由 `errors.Is(err, zlogger.ErrInvalidConfig)` 判斷。file output 的不安全
`file_name` 同時保留 `ErrInvalidConfig` 與 `ErrUnsafeLogPath`。decoder 與檔案 I/O 錯誤
不會被包裝成 `ErrInvalidConfig`。

## 顏色契約

只有 `format=console` 且 `color_enabled=true` 會輸出 ANSI 色碼。JSON level 永遠不含
ANSI，即使 `color_enabled` 保持 true。

檔案建立權限不是設定檔欄位；請參閱[安全性](security.md)與 `ConfigureWithOptions`。
