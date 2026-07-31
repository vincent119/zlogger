# timberjack Integration

[繁體中文](../../zh-TW/integrations/timberjack.md) | **English**

[Documentation index](../README.md)

The examples are verified with `github.com/DeRuina/timberjack` v1.4.5:

```bash
go get github.com/DeRuina/timberjack@v1.4.5
```

`SplitOutput` provides daily rotation. timberjack provides size limits, backup counts, retention,
and gzip or zstd compression. Do not let both components manage the same file.

## Single-File Rotation

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

This is an independent `*zap.Logger`. Even `zap.ReplaceGlobals(logger)` replaces only `zap.L()`;
it does not configure zlogger's own global logger.

## Three-File Rotation

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

Each file should have one writing process. Use distinct names or an external log collector in
multi-process environments. `NewSplitCore` never closes external sinks; the caller syncs first and
then closes them.
