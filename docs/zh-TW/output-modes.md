# 輸出模式與故障通知

**繁體中文（台灣）** | [English](../en/output-modes.md)

[文件首頁](README.md)

一般 file 輸出只寫單一檔案。要每天分開 info、warn、error，使用 split 入口。
容量換檔、保留天數和壓縮則交給外部 sink，例如 [timberjack](integrations/timberjack.md)。

## 選擇輸出方式

| 需求 | 入口 | 資源責任 |
| --- | --- | --- |
| 終端機或一般單檔 | `Configure`／`New` | 先 Sync，再 cleanup／Close |
| 全域每日三檔 | `ConfigureSplit` | cleanup 自動 Sync／Close |
| 獨立每日三檔 | `NewSplit` | Instance.Close 自動 Sync／Close |
| 自行組裝每日 core | `GetSplitCoreWithCleanup` | 先同步 logger，再執行 cleanup，取得關閉錯誤 |
| 直接持有每日輸出 | `NewSplitOutput`／`NewSplitOutputWithOptions` | 呼叫端同步並 Close |
| 三個外部 sink 分級 | `NewSplitCore` | 呼叫端同步並關閉 sink |
| 全域自訂 core | `ConfigureCore` | cleanup 同步，再執行明確登記的清理 |

`GetSplitCore` 和 `GetSplitCoreWithOptions` 仍保留，但舊 `func()` 會忽略 Close 錯誤。
新的 `GetSplitCoreWithCleanup` 不自動 Sync，也不設定全域 logger。

高階 `New`／`Configure` 系列與 `NewSplit`／`ConfigureSplit` 可透過
`Config.Redaction`／`ConfigPatch.Redaction` 啟用遮罩，預設關閉。
政策涵蓋一般輸出的所有 sink 與 split 三檔，保留原分級及清理行為。
低階 core／SplitOutput 入口不讀取這份設定；custom core 請自行使用 `NewRedactingCore`。
完整範例與限制見[安全性](security.md)。

## 每日三檔

Info 檔接收 DEBUG、INFO，warn 檔接收 WARN，error 檔接收 ERROR、DPANIC、PANIC、FATAL。
設定的 level 門檻仍會先過濾。例如預設 info 不會記錄 DEBUG。

以下是可放在專案中的函式，呼叫它時會在 ./logs 建立每日三檔。
回傳錯誤同時保留同步和關閉失敗。

```go
package example

import (
	"errors"

	"github.com/vincent119/zlogger"
	"go.uber.org/zap"
)

func WriteDailyLog() (err error) {
	core, cleanup, err := zlogger.GetSplitCoreWithCleanup(
		"./logs", "app", zap.NewProductionEncoderConfig(),
	)
	if err != nil {
		return err
	}
	logger := zap.New(core)
	defer func() {
		err = errors.Join(err, logger.Sync(), cleanup())
	}()

	logger.Info("service started")
	return nil
}
```

低階 core 範例沒有加 level gate，會接受 split 路由支援的級別。
若需要 Config 的門檻與 Instance 管理，選 `NewSplit` 或 `ConfigureSplit`。

SplitOutput.Close 可重複及並行呼叫，會停止換檔並關閉所有檔案。
關閉後 Write、Sync 回傳包裝 `os.ErrClosed` 的錯誤。清理前先停止所有寫入者。

## 外部三路 sink

`NewSplitCore(encoder, SplitSinks{Info: infoSink, Warn: warnSink, Error: errorSink})`
會複製 encoder 並依上述級別分流，不設定全域 logger，也不取得 sink 所有權。
三個 sink 都要提供有效實作，並使用不同的輸出資源。無效設定回傳 `ErrInvalidSplitCore`。

拿到 core 後，可以使用 `zap.New(core)` 或 `ConfigureCore`。
外部 sink 仍由呼叫端同步、關閉。需要自動清理時，可在 ConfigureCore 明確登記
`WithCoreCleanup`，詳見[生命週期](lifecycle.md)。

## 接收輸出故障

每日 split 可以直接登記 observer。以下片段放在可回傳 error 的函式中：

```go
observer, err := zlogger.NewFailureObserver(64)
if err != nil {
	return err
}
cfg := zlogger.DefaultConfig()
instance, err := zlogger.NewSplit(cfg, zlogger.WithOutputObserver(observer))
if err != nil {
	return err
}
defer func() { _ = instance.Close() }()
instance.Logger().Info("daily output started")
```

觀察外部 sink 時，先建立 `NewObservedWriteSyncer(sink, observer)` 再交給 core。
這個包裝只觀察 Write／Sync，原樣保留 n 與 error，不提供 Close，
也不增加 sink 的並行安全保證。必要時由呼叫端先加 zapcore.Lock。

| 事件欄位 | 內容 |
| --- | --- |
| Operation | write、sync，原生 split 另有 rotate、close |
| Output | 外部包裝為 sink，原生每日輸出為 split |
| OccurredAt | UTC 時間 |
| Err | 原始錯誤，可用 errors.Is／As 判斷 |

只有失敗才發事件。容量可設 1 至 1024，滿載丟棄新事件並累計 Dropped，不等待消費者。
事件本身不帶日誌本文，原始 Err 仍可能包含敏感資訊。
每個已包裝的 sink 各自回報，tee 不會彙整或跨包裝去重。
這裡丟棄的是故障通知，不是對一般日誌做取樣。

原生 split 通知在釋放輸出鎖後發送。重複 Close 仍回傳保存的錯誤，
但只對實際執行的 Close 發送一次事件。不要再包同一個輸出而重複觀察。

一般 `NewWithOptions`／`ConfigureWithOptions` 不支援 WithOutputObserver，會回傳 ErrInvalidConfig。
core 編碼錯誤與遮罩拒絕不在 sink 觀察範圍，仍由原錯誤途徑或 zap ErrorOutput 處理。

observer 通道不會關閉。消費者的停止順序見[生命週期](lifecycle.md)，
原始錯誤的資料限制見[安全性](security.md)。
