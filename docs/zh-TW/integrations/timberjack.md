# timberjack 整合

**繁體中文** | [English](../../en/integrations/timberjack.md)

[返回文件首頁](../README.md)

本文範例以 `github.com/DeRuina/timberjack` v1.4.5 驗證：

```bash
go get github.com/DeRuina/timberjack@v1.4.5
```

`SplitOutput` 提供每日換檔；timberjack 提供容量、備份數量、保存天數與 gzip／zstd
壓縮。不要讓兩個元件同時管理同一檔案。

## 單檔 rotation

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

	logger.Info("伺服器啟動")
}
```

這是獨立 `*zap.Logger`。即使呼叫 `zap.ReplaceGlobals(logger)`，也只替換 `zap.L()`，
不會設定 zlogger 自有的 global logger。

## 三檔 rotation

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

	logger.Info("伺服器啟動")
}
```

每個檔案應只有一個 process 寫入。多 process 環境使用不同檔名或外部 log collector。
`NewSplitCore` 不關閉外部 sinks；呼叫端負責先 `Sync` 再 `Close`。
