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

1. 停止接受新工作。
2. 等待使用 logger 的進行中工作完成。
3. 呼叫 global `Sync` 或 `Instance.Sync`。
4. 執行 Configure cleanup 或 `Instance.Close`。

`SetLevel` 可動態調整 global level。未知字串維持 legacy 行為並回退 `info`，不回傳
錯誤。
