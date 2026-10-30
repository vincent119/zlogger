# zlogger

zlogger 是以 [zap](https://github.com/uber-go/zap) 為基礎的 Go 結構化日誌套件。
可以使用全域 logger，也可以讓各元件持有自己的 Instance。內建一般檔案輸出、
每日三檔、輸出故障通知，以及需自行啟用的敏感欄位遮罩。

[![CI](https://github.com/vincent119/zlogger/actions/workflows/ci.yml/badge.svg)](https://github.com/vincent119/zlogger/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/gh/vincent119/zlogger/branch/main/graph/badge.svg)](https://codecov.io/gh/vincent119/zlogger)
[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

**繁體中文** | [English](README.en.md)

## 安裝

需要 Go 1.25 以上版本。

```bash
go get github.com/vincent119/zlogger
```

本文件對應目前分支的原始碼，發布版本不一定已包含所有 API。
若安裝的版本找不到新介面，請先確認該版本是否包含對應功能，再選擇版本或 commit。

## 先寫一筆日誌

以下範例使用預設的終端機輸出。初始化成功後會看到 `service started` 與 `component=api` 欄位。

```go
package main

import (
	"fmt"
	"os"

	"github.com/vincent119/zlogger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cleanup, err := zlogger.Configure(nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := cleanup(); err != nil {
			fmt.Fprintln(os.Stderr, "close logger:", err)
		}
	}()

	zlogger.Info("service started", zlogger.String("component", "api"))
	if err := zlogger.Sync(); err != nil {
		fmt.Fprintln(os.Stderr, "sync logger:", err)
	}
	return nil
}
```

`Configure` 只允許在同一個程序中成功一次。清理後也不能重新設定。
不需要全域狀態時，改用 `New` 或 `NewSplit`。

某些終端機或管線不支援對標準輸出執行 Sync，可能回傳系統錯誤。
範例會印出該錯誤，但不因此中止程式。檔案輸出的同步錯誤應依應用程式的可靠性需求處理。

## 該選哪個入口？

| 你要做的事 | 入口 | 關閉方式 |
| --- | --- | --- |
| 設定全域的終端機／單檔輸出 | `Configure`、`ConfigureWithOptions` | 先 `Sync`，再執行 cleanup |
| 建立獨立的終端機／單檔 logger | `New`、`NewWithOptions` | 先 `Instance.Sync`，再 `Instance.Close` |
| 設定全域的每日三檔 | `ConfigureSplit` | cleanup 自動同步、停止換檔並關檔 |
| 建立獨立的每日三檔 logger | `NewSplit` | `Instance.Close` 自動同步、停止換檔並關檔 |
| 自行組裝每日 core，取得關閉錯誤 | `GetSplitCoreWithCleanup` | 先同步 logger，再執行 cleanup |
| 把自訂 core 接到全域 API | `ConfigureCore` | cleanup 先同步，再執行登記的 `WithCoreCleanup` |
| 把三個外部 sink 依級別分流 | `NewSplitCore` | 呼叫端同步並關閉各 sink |

每日三檔依序接收 DEBUG／INFO、WARN、ERROR 以上的日誌。
`Config.Outputs=["file"]` 只會建立一般單檔，不會自動改成每日三檔。
舊 `GetSplitCore` 回傳的 `func()` 不會回傳關閉錯誤，新程式可選 `GetSplitCoreWithCleanup`。

自訂 core 也能直接交給 `zap.New(core)`，完全不設定全域狀態。
目前沒有接受任意 custom core 的非全域 Instance 建構入口。

## 輸出故障與敏感資料

`NewFailureObserver(capacity)` 可接收輸出失敗事件。外部 sink 使用
`NewObservedWriteSyncer`，每日 split 則使用 `WithOutputObserver`。
通道滿載時會丟棄新事件並累計 `Dropped()`，不會等候事件消費者。
它不保證每次故障都能送達，也不涵蓋 core 編碼錯誤。

遮罩預設關閉。一般 `New`／`Configure` 系列及 `NewSplit`／`ConfigureSplit`
可透過 `Config.Redaction`／`ConfigPatch.Redaction` 設定
`RedactionConfig{Enabled: true, Keys: []string{"token", "password"}}`，
不必自行組裝 zapcore。設定在建構時固定，無法執行中切換。
message 自由文字不會被遮罩，Reflect、Stringer、Error 則會整欄遮罩，減少可用的診斷細節。

自訂輸出仍可使用 `NewRedactingCore(encoder, sink, level, cfg)`。
`ConfigureCore` 不套用高階遮罩設定，傳入 `Redaction.Enabled=true` 會回傳 `ErrInvalidConfig`。
用法及限制分別見[輸出模式](docs/zh-TW/output-modes.md)與[安全性](docs/zh-TW/security.md)。

## 設定與欄位由誰決定？

`ConfigPatch` 可區分「沒提供」和明確設定 `false`。
zlogger 不讀設定檔或環境變數，解析方式及來源優先順序由應用程式決定。

日誌欄位也一樣。你可以用 `With` 綁定固定欄位，或用 `WithContext` 傳遞請求欄位。
現有 `WithRequestID`、`WithTraceID` 等 helper 是便利入口，不強制所有服務採用相同名稱，
也不會自動從 OpenTelemetry 擷取追蹤資訊。

## 詳細文件

| 你想了解的事 | 文件 |
| --- | --- |
| 預設值、ConfigPatch、設定檔載入邊界 | [設定](docs/zh-TW/configuration.md) |
| 全域與獨立 logger、Sync、Close、清理責任 | [生命週期](docs/zh-TW/lifecycle.md) |
| 每日三檔、自訂 sink、故障事件 | [輸出模式](docs/zh-TW/output-modes.md) |
| 自訂欄位、context、重複 key 與複製行為 | [Context 與欄位](docs/zh-TW/context-and-fields.md) |
| 遮罩範例、檔案路徑與權限限制 | [安全性](docs/zh-TW/security.md) |
| 在應用端接入 HTTP 請求日誌 | [Gin 整合](docs/zh-TW/integrations/gin.md) |
| 依容量換檔、保留與壓縮 | [timberjack 整合](docs/zh-TW/integrations/timberjack.md) |

[文件首頁](docs/zh-TW/README.md) · [GoDoc](https://pkg.go.dev/github.com/vincent119/zlogger) ·
[設計說明](DESIGN.md) · [變更紀錄](CHANGELOG.md)

## 開發與驗證

在 repository 根目錄執行：

```bash
make help
make verify
```

`make verify` 會執行格式、vet、lint、race、90% 覆蓋率檢查與 logger 效能冒煙測試。
另用 `make vuln` 掃描可達漏洞。工具版本以 Makefile 為準。

CI 的 race 測試使用 Go 1.25.12、1.26.9，相容性工作涵蓋 Linux、macOS 15 與 Windows 2025。
這些是測試環境，不代表所有作業系統都有相同的檔案權限行為。

## 授權

[MIT](LICENSE)
