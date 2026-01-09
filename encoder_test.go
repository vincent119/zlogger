package zlogger

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewNoEscapeJSONEncoder(t *testing.T) {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}

	encoder := NewNoEscapeJSONEncoder(encoderConfig)
	if encoder == nil {
		t.Error("expected non-nil encoder")
	}
}

func TestDisableHTMLEscaping(t *testing.T) {
	// 建立測試用 logger
	logger := zap.NewNop()

	result := DisableHTMLEscaping(logger)
	if result == nil {
		t.Error("expected non-nil logger")
	}
}
