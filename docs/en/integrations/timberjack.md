# timberjack Integration

[繁體中文（台灣）](../../zh-TW/integrations/timberjack.md) | **English**

[Documentation index](../README.md)

The examples use `github.com/DeRuina/timberjack` v1.4.5. Install it in your application's module:

```bash
go get github.com/DeRuina/timberjack@v1.4.5
```

SplitOutput handles daily rotation. For size limits, backup counts, or compression, let
timberjack own the files and pass its sinks to a core. Don't let both components manage the same file.
timberjack isn't a built-in zlogger dependency.

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

This creates an independent `*zap.Logger`. Calling `zap.ReplaceGlobals(logger)` changes zap's
global logger, but zlogger.Info won't use it. Use ConfigureCore below for zlogger's global API.

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

Give each file one writing process. For multiple processes, use different filenames or an
external log collector. NewSplitCore leaves sinks open, so the caller must sync and close them.
These assembly examples discard cleanup errors. Report or join those errors in production code.

## Connect the core to zlogger globals

Once you have the three-sink core, replace zap.New and its cleanup defer with this fragment,
import errors alongside the existing packages, and put the code in a function that returns error.

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

Successful configuration takes responsibility for the callback.
Cleanup syncs before closing sinks. Remove the earlier close defer.
For Write/Sync failures, wrap each sink before building NewSplitCore.
See [output modes](../output-modes.md).
