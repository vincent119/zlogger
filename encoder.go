package zlogger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewNoEscapeJSONEncoder 回傳 zap JSON encoder；zap 本身不進行 HTML browser escaping。
//
// Deprecated: 請直接使用 zapcore.NewJSONEncoder。
func NewNoEscapeJSONEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return zapcore.NewJSONEncoder(cfg)
}

// DisableHTMLEscaping 為 v1 source compatibility 保留，原樣回傳 logger。
// 此函式無法修改已建立 logger 的 encoder。
//
// Deprecated: 請在建立 core 時直接選擇符合需求的 encoder。
func DisableHTMLEscaping(log *zap.Logger) *zap.Logger {
	return log
}
