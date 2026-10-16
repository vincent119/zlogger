package zlogger_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/vincent119/zlogger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type exampleSplitSink struct {
	bytes.Buffer
}

func ExampleGetSplitCoreWithCleanup() {
	directory, err := os.MkdirTemp("", "zlogger-example-")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := os.RemoveAll(directory); err != nil {
			panic(err)
		}
	}()
	core, cleanup, err := zlogger.GetSplitCoreWithCleanup(directory, "app", zap.NewProductionEncoderConfig())
	if err != nil {
		panic(err)
	}
	logger := zap.New(core)
	logger.Info("service started")
	if err := logger.Sync(); err != nil {
		panic(err)
	}
	if err := cleanup(); err != nil {
		panic(err)
	}
	fmt.Println("synced and closed")
	// Output: synced and closed
}

func ExampleNewSplit() {
	directory, err := os.MkdirTemp("", "zlogger-instance-")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := os.RemoveAll(directory); err != nil {
			panic(err)
		}
	}()
	cfg := zlogger.DefaultConfig()
	cfg.LogPath, cfg.FileName = directory, "app"
	instance, err := zlogger.NewSplit(cfg)
	if err != nil {
		panic(err)
	}
	instance.Logger().Info("independent logger")
	if err := instance.Close(); err != nil {
		panic(err)
	}
	fmt.Println("instance synced and closed")
	// Output: instance synced and closed
}

func ExampleInstance_SetLevel() {
	instance, err := zlogger.New(zlogger.DefaultConfig())
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := instance.Close(); err != nil {
			panic(err)
		}
	}()
	if err := instance.SetLevel("debug"); err != nil {
		panic(err)
	}
	fmt.Println(instance.Logger().Core().Enabled(zlogger.DebugLevel))
	// No Output is specified because the default console initialization message would make output unstable.
}

func ExampleConfigureCore() {
	var buffer bytes.Buffer
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zapcore.EncoderConfig{MessageKey: "msg"}), zapcore.AddSync(&buffer), zap.DebugLevel)
	cleanup, err := zlogger.ConfigureCore(nil, core)
	if err != nil {
		panic(err)
	}
	zlogger.Info("service started")
	if err := cleanup(); err != nil {
		panic(err)
	}
	// The buffer belongs to the caller. Only an explicitly registered WithCoreCleanup callback runs.
	fmt.Print(buffer.String())
	// No Output is specified to avoid consuming the process-wide one-time global configuration.
}

func ExampleConfigureSplit() {
	directory, err := os.MkdirTemp("", "zlogger-global-")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := os.RemoveAll(directory); err != nil {
			panic(err)
		}
	}()
	cleanup, err := zlogger.ConfigureSplit(&zlogger.ConfigPatch{LogPath: &directory})
	if err != nil {
		panic(err)
	}
	zlogger.Info("service started")
	if err := cleanup(); err != nil {
		panic(err)
	}
	// Cleanup syncs and stops the worker. This example is compiled without consuming global configuration.
}

func (*exampleSplitSink) Sync() error {
	return nil
}

func (*exampleSplitSink) Close() error {
	return nil
}

func ExampleNewSplitCore() {
	infoSink := &exampleSplitSink{}
	warnSink := &exampleSplitSink{}
	errorSink := &exampleSplitSink{}

	core, err := zlogger.NewSplitCore(
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{MessageKey: "message"}),
		zlogger.SplitSinks{
			Info:  infoSink,
			Warn:  warnSink,
			Error: errorSink,
		},
	)
	if err != nil {
		panic(err)
	}

	logger := zap.New(core)
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")
	if err := logger.Sync(); err != nil {
		panic(err)
	}

	// The caller creates and owns the sink, and must close it after the logger is no longer in use.
	defer func() {
		_ = infoSink.Close()
		_ = warnSink.Close()
		_ = errorSink.Close()
	}()

	fmt.Printf("info: %s\n", strings.TrimSpace(infoSink.String()))
	fmt.Printf("warn: %s\n", strings.TrimSpace(warnSink.String()))
	fmt.Printf("error: %s\n", strings.TrimSpace(errorSink.String()))

	// Output:
	// info: {"message":"info"}
	// warn: {"message":"warn"}
	// error: {"message":"error"}
}
