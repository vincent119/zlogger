# 初始化與生命週期

**繁體中文（台灣）** | [English](../en/lifecycle.md)

[文件首頁](README.md)

先選好要不要使用全域狀態，再確認誰負責同步與關檔。
這兩件事由建構入口決定，不能只看最後拿到的是不是 `*zap.Logger`。

## 入口與清理責任

| 入口 | 是否設定 zlogger／zap 全域 logger | 清理方式 |
| --- | --- | --- |
| `Configure`、`ConfigureWithOptions` | 是 | 先 `Sync`，再 cleanup。cleanup 只關閉自有資源 |
| `New`、`NewWithOptions` | 否 | 先 `Instance.Sync`，再 `Instance.Close` |
| `ConfigureSplit` | 是 | cleanup 自動同步、停止換檔、關檔 |
| `NewSplit` | 否 | `Instance.Close` 自動同步、停止換檔、關檔 |
| `ConfigureCore` | 是 | cleanup 先同步，再執行明確登記的 callback |
| `GetSplitCoreWithCleanup` | 否 | 呼叫端先同步 logger，再執行 `func() error` |
| `GetSplitCore`、`GetSplitCoreWithOptions` | 否 | 呼叫端先同步。舊 `func()` 會忽略關閉錯誤 |
| `zap.New(core)` | 否 | 呼叫端同步，並依原資源責任關閉 sink |

`Init(*Config)` 是舊版相容入口，初始化失敗可能 panic。
新程式用能回傳錯誤的 `Configure`。

## 全域設定只成功一次

`Configure`、`ConfigureWithOptions`、`ConfigureSplit`、`ConfigureCore` 共用同一個限制。
第一次成功後，後續設定回傳 `ErrAlreadyConfigured`。設定失敗不會發布半成品，可以修正後重試。

成功取得的 cleanup 可重複及並行呼叫，會恢復先前的全域 logger，但不重設一次成功的限制。
呼叫 cleanup 前，要先讓所有使用該 logger 的工作結束。
不要一邊發布或清理全域 logger，一邊讓其他 goroutine 寫入。

`SetLevel("debug")` 會調整全域門檻。為保留既有行為，未知字串會退回 `info`，不回傳錯誤。

## 每個元件持有自己的 Instance

一般輸出使用 `New`，每日三檔使用 `NewSplit`。
以下片段放在可回傳 `error` 的函式中，省略的清理錯誤請在正式應用中依需求處理。

```go
cfg := zlogger.DefaultConfig()
cfg.LogPath = "./logs/worker"
cfg.FileName = "worker"

instance, err := zlogger.NewSplit(cfg)
if err != nil {
	return err
}
defer func() { _ = instance.Close() }()

if err := instance.SetLevel("debug"); err != nil {
	return err
}
instance.Logger().Info("worker started")
```

每個每日 Instance 要用不同的目錄或檔名前綴。
`Instance.SetLevel` 只影響自己，無效值回傳 `ErrInvalidConfig`，不改動原級別。

`Instance.Close` 可重複及並行呼叫。關閉後不可再使用先前取得的 logger，
`Instance.Sync` 和 `Instance.SetLevel` 會回傳包裝 `os.ErrClosed` 的錯誤。
每日 Instance 即使同步失敗仍會關檔，並合併同步與關閉錯誤。
一般 Instance 的 Close 不會替你同步。

## 自訂 core 與資源所有權

`ConfigureCore(patch, core, opts...)` 讓自訂 core 使用 zlogger 全域 API。
編碼和輸出由 core 決定，patch 只用 Level、AddCaller、AddStacktrace、Development，
其餘設定仍須通過 Resolve 驗證。

`Redaction.Enabled=true` 是例外，會回傳 `ErrInvalidConfig`，避免誤認外部 core 已受保護。
外部遮罩 core 請先用 `NewRedactingCore` 建立，再搭配 nil／停用的高階遮罩設定。

外部 sink 預設仍由呼叫端負責。`WithCoreCleanup(func() error)` 可明確登記清理工作，
cleanup 會先 Sync，再執行該 callback，即使 Sync 失敗仍會執行並合併錯誤。
callback 不可為 nil 或重複登記，設定失敗時也不會執行，呼叫端仍須自行回收資源。

全域 SetLevel 與原 core 的篩選條件取交集，不能打開原 core 已拒絕的級別。
只想建立非全域自訂 logger，直接使用 `zap.New(core)`。
目前沒有接受任意 custom core 的 Instance 建構入口。

`NewSplit`／`ConfigureSplit` 使用 LogPath 作目錄、FileName 作前綴。
Outputs 仍會驗證，但不決定輸出位置。

## 遮罩與故障通知的清理

一般與每日三檔高階入口可透過 `Config.Redaction`／`ConfigPatch.Redaction` 啟用遮罩。
設定會先驗證，才建立檔案或 worker；不新增資源，也不改變原先的 Sync、Close 或 cleanup 順序。
split 可同時使用 `WithOutputObserver`；遮罩拒絕不產生 sink 故障事件。

`NewRedactingCore` 要接收尚未綁定欄位的 encoder。
它自行建立 core，不接受任意既有 core，不取得 sink 所有權，也不新增背景工作。
可以把結果交給 `zap.New` 或 `ConfigureCore`。

若要同時觀察 sink 故障，先用 `NewObservedWriteSyncer` 包裝 sink，
再傳入 `NewRedactingCore`。遮罩拒絕的日誌還沒進入 sink，因此不會產生 sink 故障事件。

`FailureObserver` 不啟動 goroutine，Events 通道永遠不會關閉。
消費者要有自己的停止方式，不能靠 `range observer.Events()` 等待結束。

```go
for {
	select {
	case <-ctx.Done():
		return
	case event := <-observer.Events():
		// Use operation metadata for health checks. Keep raw errors out of external alerts.
		_ = event.Operation
	}
}
```

## 關機時先停寫入，再清理

停止接收新工作，等仍在使用 logger 的工作完成，再依上表執行同步及關閉。
有故障事件消費者時，先完成輸出清理、取出需要保留的事件，再停止消費者。
事件通知不取代原錯誤或 zap 的 ErrorOutput，也不保證所有故障都能送達。

預設 Fatal 會結束程序，不會替應用程式執行完整關機流程，也不能依賴 defer 清理。
若服務需要優雅關機，請由應用程式回傳錯誤並安排停止工作、同步和資源清理。
