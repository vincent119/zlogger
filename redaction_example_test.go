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
	// 必須在建構遮罩 core 後才綁定欄位；零值設定不會啟用遮罩。
	zap.New(core).With(zlogger.String("token", "示範用值")).Info("驗證請求", zlogger.String("request_id", "r1"))
	fmt.Print(output.String())
	// Output:
	// {"msg":"驗證請求","token":"[REDACTED]","request_id":"r1"}
}
