# 輸出模式

**繁體中文** | [English](../en/output-modes.md)

[返回文件首頁](README.md)

## 選擇方式

| 需求 | 入口 | rotation | ownership |
| --- | --- | --- | --- |
| console 或單檔 | `Configure`／`New` | 無 | cleanup／Instance |
| 每日三檔 | `GetSplitCore` | 每日 | 回傳的 cleanup |
| 每日三檔與關閉錯誤 | `GetSplitCoreWithCleanup` | 每日 | 先 Sync，再呼叫 `func() error` |
| 獨立每日 logger | `NewSplit` | 每日 | Instance.Close 自動 Sync／Close |
| 全域每日 logger | `ConfigureSplit` | 每日 | cleanup 自動 Sync／Close |
| 全域 custom core | `ConfigureCore` | 由 core 決定 | WithCoreCleanup 明確登記清理 |
| 直接持有每日三檔 | `NewSplitOutput`／`NewSplitOutputWithOptions` | 每日 | 呼叫端 Close |
| 自訂三路 sink | `NewSplitCore` | 由 sink 決定 | 呼叫端管理 sinks |

`Config.Outputs` 的 `file` 是單一檔案，不會自動建立 `SplitOutput`。

`GetSplitCoreWithCleanup` 接受既有前三個參數與可選 FileOutputOption，回傳
`(core, func() error, error)`，保留 Close 錯誤，不自動 Sync 或設定 globals。
NewSplit／ConfigureSplit 僅建立每日三檔，Outputs 不控制輸出。

## 每日分級

```go
core, cleanup, err := zlogger.GetSplitCore(
	"./logs",
	"app",
	zapcore.EncoderConfig{
		TimeKey:    "ts",
		LevelKey:   "level",
		MessageKey: "msg",
		EncodeTime: zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	},
)
if err != nil {
	return err
}
logger := zap.New(core)
defer func() {
	_ = logger.Sync()
	cleanup()
}()
```

- info 檔：DEBUG、INFO
- warn 檔：WARN
- error 檔：ERROR、DPANIC、PANIC、FATAL

cleanup 會停止換檔 goroutine 並關閉檔案。先停止使用 logger、呼叫 `Sync`，再執行
cleanup。自訂權限使用 `GetSplitCoreWithOptions`。

直接持有 `SplitOutput` 時，`Close` 可重複及並行呼叫；關閉後 `Write`、`Sync` 回傳
包裝 `os.ErrClosed` 的錯誤。

## 自訂 sinks

### 故障觀察

```go
observer, err := zlogger.NewFailureObserver(1)
if err != nil {
	return err
}
observedSink, err := zlogger.NewObservedWriteSyncer(sink, observer)
if err != nil {
	return err
}
// 將 observedSink 傳入 zapcore.NewCore 或 SplitSinks。
```

observer 容量為 1 至 1024，滿載丟棄新事件並累計 `Dropped()`。成功不發事件；失敗事件
包含 Operation（write／sync／rotate／close）、Output（sink／split）、UTC OccurredAt 與
原 Err。外部 wrapper 只回報 sink Write／Sync，保留原 n／error，不涵蓋編碼錯誤，
不介入 core Check／hooks。tee 每個已包裝 sink 各自回報，不聚合或跨 wrapper 去重。

每日 split 的所有 options 入口可用 `WithOutputObserver(observer)`；Write、Sync、
rotation 與一次性 Close 原生通知，不再另包同一輸出的 sink wrapper。一般 NewWithOptions／
ConfigureWithOptions 不支援此 option，回傳 ErrInvalidConfig；外部 sink 請自行包裝。

### 分級 core

```go
core, err := zlogger.NewSplitCore(
	zapcore.NewJSONEncoder(encoderConfig),
	zlogger.SplitSinks{Info: infoSink, Warn: warnSink, Error: errorSink},
)
if err != nil {
	if errors.Is(err, zlogger.ErrInvalidSplitCore) {
		return fmt.Errorf("分級輸出設定無效: %w", err)
	}
	return err
}
logger := zap.New(core)
```

三個欄位應使用不同 sink。`NewSplitCore` 會 clone encoder，但不取得 sink ownership，
也不呼叫 `Close`。呼叫端必須先同步 logger，再關閉外部 sinks。

檔案路徑與權限請參閱[安全性](security.md)；容量與壓縮請參閱
[timberjack 整合](integrations/timberjack.md)。
