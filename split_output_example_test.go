package zlogger_test

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/vincent119/zlogger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type exampleSplitSink struct {
	bytes.Buffer
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
