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
	logger.Info("服務啟動")
	if err := logger.Sync(); err != nil {
		panic(err)
	}
	if err := cleanup(); err != nil {
		panic(err)
	}
	fmt.Println("已同步並關閉")
	// Output: 已同步並關閉
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
	instance.Logger().Info("獨立日誌")
	if err := instance.Close(); err != nil {
		panic(err)
	}
	fmt.Println("Instance 已同步並關閉")
	// Output: Instance 已同步並關閉
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
	// 未設定 Output，避免預設 console 初始化訊息造成不穩定輸出。
}

func ExampleConfigureCore() {
	var buffer bytes.Buffer
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zapcore.EncoderConfig{MessageKey: "msg"}), zapcore.AddSync(&buffer), zap.DebugLevel)
	cleanup, err := zlogger.ConfigureCore(nil, core)
	if err != nil {
		panic(err)
	}
	zlogger.Info("服務啟動")
	if err := cleanup(); err != nil {
		panic(err)
	}
	// buffer 屬於呼叫端；只有明確 WithCoreCleanup 登記的 callback 才會執行。
	fmt.Print(buffer.String())
	// 未設定 Output，避免範例測試消耗 process 中只能成功一次的 global 設定資格。
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
	zlogger.Info("服務啟動")
	if err := cleanup(); err != nil {
		panic(err)
	}
	// cleanup 自動 Sync 並停止 worker；本範例只編譯，不消耗 global 設定資格。
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

	// sink 由呼叫端建立，因此也由呼叫端在 logger 停止使用後關閉。
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
