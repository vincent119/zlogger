# 初始化與生命週期

**繁體中文** | [English](../en/lifecycle.md)

[返回文件首頁](README.md)

## 入口比較

| 入口 | 錯誤處理 | 資源責任 | global 影響 |
| --- | --- | --- | --- |
| `Configure` | 回傳 error | 執行 cleanup | 設定 zlogger 與 zap global |
| `ConfigureWithOptions` | 回傳 error | 執行 cleanup | 設定 zlogger 與 zap global |
| `Init` | 失敗可能 panic | compatibility 路徑 | 設定 zlogger 與 zap global |
| `New`／`NewWithOptions` | 回傳 error | `Instance.Close` | 無 |
| 直接 `zap.New` | 呼叫端定義 | 呼叫端管理 sink | 無，除非另行 `zap.ReplaceGlobals` |

## Global logger

`Configure` 在同一 process 只能成功一次。失敗不會發布半成品，修正設定後可重試；
第一次成功後的呼叫回傳 `ErrAlreadyConfigured`。cleanup 可重複呼叫，但不重設再次
Configure 的資格。

```go
cleanup, err := zlogger.Configure(nil)
if err != nil {
	return err
}
defer func() { _ = cleanup() }()
```

`Init(*Config)` 只為來源碼相容保留。它無法回傳設定或 I/O 錯誤，初始化失敗可能
panic；新程式應使用 `Configure`。

## Instance logger

每日三檔使用 `NewSplit(cfg, opts...)`，回傳既有 `Instance`，不修改 globals。
`Instance.SetLevel("debug")` 只調整該 Instance；無效字串回傳 `ErrInvalidConfig`，
nil 或已關閉 Instance 回傳包裝 `os.ErrClosed` 的錯誤。不同 Instance 使用不同目錄或 prefix。

split Instance.Close 自動先 Sync，再停止 worker 並關檔，即使 Sync 失敗仍關閉並合併錯誤。
一般 New／NewWithOptions 維持先由呼叫端自行 Sync，再 Close。

```go
instance, err := zlogger.New(zlogger.DefaultConfig())
if err != nil {
	return err
}
defer func() { _ = instance.Close() }()

instance.Logger().Info("服務啟動")
if err := instance.Sync(); err != nil {
	return err
}
```

`Instance.Close` 可安全重複及並行呼叫。Close 後不得使用 `Logger()` 回傳的 logger；
`Instance.Sync` 會回傳包裝 `os.ErrClosed` 的錯誤。

## 關機順序

`ConfigureCore(patch, core, opts...)` 與 `ConfigureSplit(patch, opts...)` 共用 Configure
的一次成功 guard，發布 zlogger／zap globals。cleanup 可重複及並行呼叫，恢復先前 globals，
但不重設成功資格；應先停止所有 logger 使用者。

- ConfigureCore 自動先 Sync，再執行 `WithCoreCleanup(func() error)` 登記的 callback。
  預設不關閉外部 sinks；失敗設定不取得 callback ownership；nil 或重複 callback 不合法。
- ConfigureSplit 委派 split Instance.Close，一次完成 Sync、停止 worker 與關檔。
- 新入口保留合併錯誤；既有 Configure cleanup 維持 Close-only。
- custom core level 與原 core 篩選取交集，SetLevel 無法啟用原本被拒絕的級別。

ConfigureCore 只使用 Level、AddCaller、AddStacktrace、Development，編碼與輸出由 core
決定；其餘欄位仍受 Resolve 驗證。NewSplit／ConfigureSplit 使用 LogPath 作目錄、FileName
作 prefix，Outputs 不控制輸出但仍受驗證，Format／ColorEnabled 沿用既有 encoder 規則。

1. 停止接受新工作。
2. 等待使用 logger 的進行中工作完成。
3. 呼叫 global `Sync` 或 `Instance.Sync`。
4. 執行 Configure cleanup 或 `Instance.Close`。

`SetLevel` 可動態調整 global level。未知字串維持 legacy 行為並回退 `info`，不回傳
錯誤。
