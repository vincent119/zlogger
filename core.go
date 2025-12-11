package zlogger

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// globalLogger 全局日誌實例
	globalLogger *zap.Logger
	once         sync.Once
	// zapGlobalLevel 全局日誌級別，支援動態修改
	zapGlobalLevel = zap.NewAtomicLevel()
	// globalConfig 儲存當前配置
	globalConfig *Config
)

// Field 是 zap.Field 的別名，使用時更簡潔
type Field = zap.Field

// sqlProcessingCore 是一個處理 SQL 字段的核心包裝器
type sqlProcessingCore struct {
	zapcore.Core
}

// With 實現 zapcore.Core 接口
func (c *sqlProcessingCore) With(fields []zapcore.Field) zapcore.Core {
	for i := range fields {
		if fields[i].Key == "sql" && fields[i].Type == zapcore.StringType {
			fields[i].String = processSQLString(fields[i].String)
		}
	}
	return &sqlProcessingCore{Core: c.Core.With(fields)}
}

// Check 實現 zapcore.Core 接口
func (c *sqlProcessingCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Core.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

// Write 實現 zapcore.Core 接口
func (c *sqlProcessingCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	ent.Message = strings.ReplaceAll(ent.Message, "\\", "")

	for i := range fields {
		if fields[i].Key == "sql" && fields[i].Type == zapcore.StringType {
			fields[i].String = processSQLString(fields[i].String)
		}
	}

	return c.Core.Write(ent, fields)
}

// Init 使用傳入的配置初始化日誌系統
func Init(cfg *Config) {
	once.Do(func() {
		initLogger(cfg)
	})
}

// initLogger 實際初始化邏輯
func initLogger(cfg *Config) {
	// 合併預設配置
	globalConfig = DefaultConfig().Merge(cfg)

	// 設置日誌級別
	logLevel := parseLevel(globalConfig.Level)
	zapGlobalLevel.SetLevel(logLevel)

	// 配置編碼器 - 根據設定決定是否使用顏色
	var levelEncoder zapcore.LevelEncoder
	if globalConfig.ColorEnabled {
		levelEncoder = zapcore.CapitalColorLevelEncoder // 帶顏色
	} else {
		levelEncoder = zapcore.CapitalLevelEncoder // 不帶顏色
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    levelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		ConsoleSeparator: " ",
	}

	// 定義日誌輸出
	var outputs []zapcore.Core

	// 處理輸出目標
	for _, output := range globalConfig.Outputs {
		switch strings.ToLower(output) {
		case "console":
			core := buildConsoleCore(encoderConfig)
			outputs = append(outputs, core)

		case "file":
			core := buildFileCore(encoderConfig)
			if core != nil {
				outputs = append(outputs, core)
			}
		}
	}

	// 如果沒有指定任何輸出，預設使用控制台輸出
	if len(outputs) == 0 {
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
		consoleOutput := zapcore.Lock(os.Stdout)
		outputs = append(outputs, zapcore.NewCore(consoleEncoder, consoleOutput, zapGlobalLevel))
	}

	// 建立核心日誌
	core := zapcore.NewTee(outputs...)

	// 建立日誌實例
	globalLogger = zap.New(core)

	// 添加處理反斜線的 hook
	globalLogger = globalLogger.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
		return nil
	}))

	var options []zap.Option

	if globalConfig.AddCaller {
		options = append(options, zap.AddCaller())
		// 設置 caller skip 以正確顯示調用位置
		options = append(options, zap.AddCallerSkip(1))
	}

	if globalConfig.AddStacktrace {
		options = append(options, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	if globalConfig.Development {
		options = append(options, zap.Development())
	}

	if len(options) > 0 {
		globalLogger = globalLogger.WithOptions(options...)
	}

	// 替換全局 logger
	zap.ReplaceGlobals(globalLogger)

	// 記錄日誌系統初始化信息
	globalLogger.Info("日誌系統初始化完成",
		zap.String("level", globalConfig.Level),
		zap.String("format", globalConfig.Format),
		zap.Strings("outputs", globalConfig.Outputs),
		zap.String("path", globalConfig.LogPath),
		zap.String("file", globalConfig.FileName),
	)
}

// buildConsoleCore 建立控制台輸出核心
func buildConsoleCore(encoderConfig zapcore.EncoderConfig) zapcore.Core {
	var encoder zapcore.Encoder
	if strings.ToLower(globalConfig.Format) == "json" {
		jsonEncoderConfig := encoderConfig
		jsonEncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format(time.RFC3339))
		}
		encoder = zapcore.NewJSONEncoder(jsonEncoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}
	consoleOutput := zapcore.Lock(os.Stdout)
	return zapcore.NewCore(encoder, consoleOutput, zapGlobalLevel)
}

// buildFileCore 建立檔案輸出核心
func buildFileCore(encoderConfig zapcore.EncoderConfig) zapcore.Core {
	var encoder zapcore.Encoder
	if strings.ToLower(globalConfig.Format) == "json" {
		jsonEncoderConfig := encoderConfig
		jsonEncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format(time.RFC3339))
		}
		encoder = zapcore.NewJSONEncoder(jsonEncoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 確保日誌目錄存在
	logDir := globalConfig.LogPath
	if logDir == "" {
		logDir = "./logs"
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		panic("無法建立日誌目錄: " + err.Error())
	}

	// 確定檔案名稱
	var logFileName string
	if globalConfig.FileName != "" {
		logFileName = globalConfig.FileName
	} else {
		now := time.Now()
		logFileName = now.Format("2006-01-02") + ".log"
	}

	// 開啟日誌檔案
	logFilePath := filepath.Join(logDir, logFileName)
	logFile, err := os.OpenFile(
		logFilePath,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		panic("無法開啟日誌檔案: " + err.Error())
	}

	fileOutput := zapcore.Lock(logFile)
	return zapcore.NewCore(encoder, fileOutput, zapGlobalLevel)
}

// parseLevel 解析日誌級別字串
func parseLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	case "fatal":
		return zap.FatalLevel
	default:
		return zap.InfoLevel
	}
}

// Debug 記錄調試信息
func Debug(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Debug(msg, fields...)
	}
}

// Info 記錄一般信息
func Info(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Info(msg, fields...)
	}
}

// Warn 記錄警告信息
func Warn(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Warn(msg, fields...)
	}
}

// Error 記錄錯誤信息
func Error(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Error(msg, fields...)
	}
}

// Fatal 記錄致命錯誤並退出程式
func Fatal(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Fatal(msg, fields...)
	}
}

// SetLevel 動態設置日誌級別
func SetLevel(level string) {
	zapGlobalLevel.SetLevel(parseLevel(level))
	Info("日誌級別已變更", String("level", level))
}

// GetLogger 返回原始 zap logger
func GetLogger() *zap.Logger {
	return globalLogger
}

// Sync 同步日誌緩衝區
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// processSQLString 處理 SQL 字串中的轉義字符
func processSQLString(sql string) string {
	sql = strings.ReplaceAll(sql, "\\\\", "\\")
	sql = strings.ReplaceAll(sql, "\\\"", "\"")
	sql = strings.ReplaceAll(sql, "\\'", "'")
	return sql
}

