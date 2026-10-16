package zlogger_test

import (
	"bytes"
	"fmt"

	"github.com/vincent119/zlogger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func ExampleNewRedactingCore() {
	var output bytes.Buffer
	core, err := zlogger.NewRedactingCore(
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{MessageKey: "msg"}),
		zapcore.AddSync(&output), zap.InfoLevel,
		zlogger.RedactionConfig{Enabled: true, Keys: []string{"token", "password"}},
	)
	if err != nil {
		panic(err)
	}
	// Bind fields only after constructing the redacting core. The zero-value configuration leaves redaction disabled.
	zap.New(core).With(zlogger.String("token", "example value")).Info("authentication request", zlogger.String("request_id", "r1"))
	fmt.Print(output.String())
	// Output:
	// {"msg":"authentication request","token":"[REDACTED]","request_id":"r1"}
}
