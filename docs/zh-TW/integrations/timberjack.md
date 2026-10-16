# timberjack 整合

**繁體中文（台灣）** | [English](../../en/integrations/timberjack.md)

[返回文件首頁](../README.md)

範例使用 `github.com/DeRuina/timberjack` v1.4.5，請在應用程式自己的 module 安裝：

```bash
go get github.com/DeRuina/timberjack@v1.4.5
```

zlogger 的 SplitOutput 處理每日換檔。需要按容量換檔、限制備份數或壓縮時，
由 timberjack 管理檔案，再把 sink 接到 core。兩個元件不能同時管理同一檔案。
timberjack 不是 zlogger 的內建依賴。

## 單檔容量換檔

```go
package main

import (
	"github.com/DeRuina/timberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey: "ts", LevelKey: "level", MessageKey: "msg",
		EncodeTime: zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}
	sink := &timberjack.Logger{
		Filename: "./logs/app.log", MaxSize: 100,
		MaxBackups: 10, MaxAge: 30, Compression: "gzip",
	}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), zapcore.AddSync(sink), zap.InfoLevel)
	logger := zap.New(core, zap.AddCaller())
	defer func() {
		_ = logger.Sync()
		_ = sink.Close()
	}()

	logger.Info("server started")
}
```

這段建立獨立的 `*zap.Logger`。`zap.ReplaceGlobals(logger)` 只替換 zap 的全域 logger，
不會讓 zlogger.Info 使用它。若需要 zlogger 全域 API，使用下面的 ConfigureCore 接法。

## 三檔容量換檔

```go
package main

import (
	"github.com/DeRuina/timberjack"
	"github.com/vincent119/zlogger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey: "ts", LevelKey: "level", MessageKey: "msg",
		EncodeTime: zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}
	infoSink := &timberjack.Logger{Filename: "./logs/app-info.log", MaxSize: 100, Compression: "gzip"}
	warnSink := &timberjack.Logger{Filename: "./logs/app-warn.log", MaxSize: 100, Compression: "gzip"}
	errorSink := &timberjack.Logger{Filename: "./logs/app-error.log", MaxSize: 100, Compression: "gzip"}

	core, err := zlogger.NewSplitCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zlogger.SplitSinks{Info: infoSink, Warn: warnSink, Error: errorSink},
	)
	if err != nil {
		panic(err)
	}
	logger := zap.New(core)
	defer func() {
		_ = logger.Sync()
		_ = infoSink.Close()
		_ = warnSink.Close()
		_ = errorSink.Close()
	}()

	logger.Info("server started")
}
```

每個檔案只讓一個程序寫入。多程序環境要用不同檔名，或交給外部日誌收集器。
`NewSplitCore` 不關閉外部 sink，呼叫端要先 Sync 再 Close。
範例為了集中說明組裝方式而忽略清理錯誤，正式程式請回報或合併這些錯誤。

## 接到 zlogger 全域 API

已有三路 core 和 sink 時，可以用以下片段取代上例的 zap.New 與 defer 清理。
需要額外匯入 errors，並放在可回傳 error 的函式內。

```go
closeSinks := func() error {
	return errors.Join(infoSink.Close(), warnSink.Close(), errorSink.Close())
}
cleanup, err := zlogger.ConfigureCore(nil, core, zlogger.WithCoreCleanup(closeSinks))
if err != nil {
	return errors.Join(err, closeSinks())
}
defer func() { _ = cleanup() }()
zlogger.Info("service started")
```

設定成功才把 callback 清理責任交出去。cleanup 會先同步 core，再關閉三個 sink。
不要同時保留原本的關檔 defer。若要觀察 Write／Sync 故障，先包裝各個 sink，
再組 NewSplitCore，詳見[輸出模式](../output-modes.md)。
