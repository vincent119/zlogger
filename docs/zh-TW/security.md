# 安全性與敏感資料

**繁體中文** | [English](../en/security.md)

[文件首頁](README.md)

只記錄排查問題需要的欄位。完整 request、response、Config 或任意 struct
都可能帶入密碼、token、cookie 或個資，遮罩也無法替你判斷所有自由文字。

## 手動遮罩

如果根本不需要保留原值，用 `Redacted` 就好。
它不接受秘密值，也不受自動遮罩開關影響。

```go
zlogger.Info("authentication request",
	zlogger.Redacted("authorization"),
	zlogger.String("request_id", "req-123"),
)
```

`authorization` 固定輸出 `[REDACTED]`。其他欄位不會因此被掃描。

## 依欄位名稱自動遮罩

`NewRedactingCore(encoder, sink, level, cfg)` 自行建立 core，預設不啟用遮罩。
要開啟時，明確提供 `Enabled: true` 和 `Keys`。一般與 split 建構入口不會自動套用這個設定。

以下是完整範例，輸出中的 token 會變成 `[REDACTED]`，request_id 保留原值。

```go
package main

import (
	"fmt"
	"os"

	"github.com/vincent119/zlogger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	core, err := zlogger.NewRedactingCore(
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{MessageKey: "msg"}),
		zapcore.AddSync(os.Stdout),
		zap.InfoLevel,
		zlogger.RedactionConfig{
			Enabled: true,
			Keys:    []string{"token", "password"},
		},
	)
	if err != nil {
		return err
	}

	logger := zap.New(core)
	logger.With(zlogger.String("token", "example value")).Info(
		"authentication request", zlogger.String("request_id", "req-123"),
	)
	return nil
}
```

這個範例寫到標準輸出，不關閉 os.Stdout。換成檔案或其他 sink 時，
請依[生命週期](lifecycle.md)同步並關閉自己持有的資源。
也可以將 core 傳入 `ConfigureCore`，使用 zlogger 全域 API。

### 規則在建構時固定

| 設定 | 行為 |
| --- | --- |
| `Enabled=false` 或零值設定 | 建立一般 zap core，忽略且不驗證 Keys |
| `Enabled=true` | 驗證並複製 Keys，不受原 slice 後續修改影響 |
| Keys | 1 至 128 個項目，每個長度 1 至 128 bytes |
| 名稱比對 | ASCII 大小寫不敏感，完整比對單一欄位名稱 |
| 重複名稱 | 正規化後合併 |
| 空白、點號、正則 | 不去除空白，不解析路徑或正則 |
| 執行中更新 | 不支援，要改規則就重新建構 |

`token` 能匹配 `TOKEN`，但不會匹配名為 `user.token` 的欄位。
巢狀物件中的 `token` 仍會匹配，因為檢查的是每一層的欄位名稱。
nil encoder、sink、level 或啟用時無效的 Keys 回傳 `ErrInvalidConfig`。
介面的 nil 檢查不保證能辨識 typed nil，請提供有效的實作。

### 哪些值會被處理？

匹配的欄位會先換成固定字串，不呼叫原值的 marshaler。
匹配的 namespace 則遮罩整個子樹。Object、Array、Inline 會逐層處理。

Reflect、Stringer、Error 即使名稱沒匹配，也會整欄遮罩。
這類值不呼叫原來的 String、Error 或 MarshalJSON，代價是失去錯誤細節與部分物件內容。
Any 依 zap 實際建立的 Field 型別處理，不能一概當成可展開的物件。

先建構遮罩 core，再使用 With、WithLazy 或 context 欄位。
encoder 必須尚未綁定欄位，否則先前寫進去的內容無法回頭遮罩。

### 處理失敗時不寫出該筆日誌

深度超過 32、記錄預算超過 1024、未知 Field 型別、marshaler 錯誤或 panic，
都會拒絕整筆日誌並回傳固定的 `ErrRedactionFailed`。
記錄預算包含欄位、陣列元素及容器／namespace 操作，不等同輸出 JSON 的欄位數。

失敗時不會先寫出一部分，也不附帶來源錯誤。
With 無法回傳 error，因此會保存失敗狀態，等後續 Write 回報。
直接呼叫 core.Write 可取得錯誤，透過 logger 寫入則交給 zap ErrorOutput。
遮罩失敗還沒接觸 sink，不會產生 sink 故障事件。

message、自由文字、欄位名稱、建立 Field 前已執行的程式碼、外部 hook、
既有 encoder 欄位與原始 I/O error 都不在保護範圍。
marshaler 仍是受信任程式碼，無法中止永不返回的實作。

## 故障事件也可能帶有敏感資訊

`OutputFailure` 不保存日誌本文或欄位，Err 也不參與 JSON／YAML 序列化。
但原始 Err 仍可能包含路徑或內部資訊。送到外部告警前，
請自行檢查內容，不要直接輸出整個事件的 `%+v`。

observer 使用有界通道，應觀察 `Dropped()`。
事件遺失代表通知不完整，不能據此宣稱所有寫入都成功。

## 路徑與檔案權限

`log_path` 或 SplitOutput 的 directory 是由呼叫端選定的可信任目錄。
`file_name`、`filePrefix` 只能是單一檔名，不能含路徑。
`.`、`..`、斜線、反斜線、NUL、絕對路徑與 Windows 磁碟前綴都會拒絕。

不安全名稱可用 `errors.Is(err, zlogger.ErrUnsafeLogPath)` 判斷。
透過 Config 驗證時，錯誤鏈也保留 `ErrInvalidConfig`。

開檔使用 `os.OpenRoot`、相對於 root 的 Lstat 與 OpenFile。
已存在的最終符號連結會拒絕，路徑競態也受 os.Root 邊界限制。
不過 base directory 本身的符號連結仍會被跟隨，root 內的競態符號連結也可能被跟隨，
此機制不隔離掛載點、特殊裝置或惡意檔案系統。呼叫端仍要保護目錄與設定來源。
Go 在 js、plan9、wasip1 的標準庫支援有限，本專案的 CI 相容性環境為 Linux、macOS、Windows。

新目錄預設 `0700`，新檔案預設 `0600`。要調整可使用：

```go
instance, err := zlogger.NewWithOptions(
	cfg,
	zlogger.WithDirPerm(0o750),
	zlogger.WithFilePerm(0o640),
)
```

這段沿用應用程式的 cfg，取得 Instance 後仍須處理 err 與清理。
目錄須包含擁有者 rwx，檔案須包含擁有者 rw，不允許其他使用者寫入或非權限位元。
不合法值回傳 `ErrInvalidFilePermission`，同類選項最後一個值生效。

選項只影響新建立的物件，umask 仍可限縮權限，既有權限不會改寫。
Windows 接受相同 API，但不保證 POSIX 權限語意。權限選項不放在設定檔欄位。
