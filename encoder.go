package zlogger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewNoEscapeJSONEncoder 創建一個不轉義 HTML 的 JSON 編碼器
func NewNoEscapeJSONEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return zapcore.NewJSONEncoder(cfg)
}

// DisableHTMLEscaping 為 logger 添加 hook（保留擴展用）
func DisableHTMLEscaping(log *zap.Logger) *zap.Logger {
	return log.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
		return nil
	}))
}
