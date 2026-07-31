# 輸出模式

**繁體中文** | [English](../en/output-modes.md)

[返回文件首頁](README.md)

## 選擇方式

| 需求 | 入口 | rotation | ownership |
| --- | --- | --- | --- |
| console 或單檔 | `Configure`／`New` | 無 | cleanup／Instance |
| 每日三檔 | `GetSplitCore` | 每日 | 回傳的 cleanup |
| 直接持有每日三檔 | `NewSplitOutput`／`NewSplitOutputWithOptions` | 每日 | 呼叫端 Close |
| 自訂三路 sink | `NewSplitCore` | 由 sink 決定 | 呼叫端管理 sinks |

`Config.Outputs` 的 `file` 是單一檔案，不會自動建立 `SplitOutput`。

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
