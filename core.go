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
	// globalLogger is the global logger instance
	globalLogger *zap.Logger
	once         sync.Once
	// zapGlobalLevel is the global log level, supports dynamic modification
	zapGlobalLevel = zap.NewAtomicLevel()
	// globalConfig stores the current configuration
	globalConfig *Config
)

// Field is an alias for zap.Field for convenience
type Field = zap.Field

// sqlProcessingCore is a core wrapper that processes SQL fields
type sqlProcessingCore struct {
	zapcore.Core
}

// With implements zapcore.Core interface
func (c *sqlProcessingCore) With(fields []zapcore.Field) zapcore.Core {
	for i := range fields {
		if fields[i].Key == "sql" && fields[i].Type == zapcore.StringType {
			fields[i].String = processSQLString(fields[i].String)
		}
	}
	return &sqlProcessingCore{Core: c.Core.With(fields)}
}

// Check implements zapcore.Core interface
func (c *sqlProcessingCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

// Write implements zapcore.Core interface
func (c *sqlProcessingCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	ent.Message = strings.ReplaceAll(ent.Message, "\\", "")

	for i := range fields {
		if fields[i].Key == "sql" && fields[i].Type == zapcore.StringType {
			fields[i].String = processSQLString(fields[i].String)
		}
	}

	return c.Core.Write(ent, fields)
}

// Init initializes the logging system with the provided configuration
func Init(cfg *Config) {
	once.Do(func() {
		initLogger(cfg)
	})
}

// initLogger is the actual initialization logic
func initLogger(cfg *Config) {
	// Merge with default config
	globalConfig = DefaultConfig().Merge(cfg)

	// Set log level
	logLevel := parseLevel(globalConfig.Level)
	zapGlobalLevel.SetLevel(logLevel)

	// Configure encoder - decide whether to use colors based on settings
	var levelEncoder zapcore.LevelEncoder
	if globalConfig.ColorEnabled {
		levelEncoder = zapcore.CapitalColorLevelEncoder // with color
	} else {
		levelEncoder = zapcore.CapitalLevelEncoder // without color
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:          "ts",
		LevelKey:         "level",
		NameKey:          "logger",
		CallerKey:        "caller",
		FunctionKey:      zapcore.OmitKey,
		MessageKey:       "msg",
		StacktraceKey:    "stacktrace",
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeLevel:      levelEncoder,
		EncodeTime:       zapcore.ISO8601TimeEncoder,
		EncodeDuration:   zapcore.StringDurationEncoder,
		EncodeCaller:     zapcore.ShortCallerEncoder,
		ConsoleSeparator: " ",
	}

	// Define log outputs
	var outputs []zapcore.Core

	// Process output targets
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

	// If no outputs specified, default to console output
	if len(outputs) == 0 {
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
		consoleOutput := zapcore.Lock(os.Stdout)
		outputs = append(outputs, zapcore.NewCore(consoleEncoder, consoleOutput, zapGlobalLevel))
	}

	// Create core logger
	core := zapcore.NewTee(outputs...)

	// Create logger instance
	globalLogger = zap.New(core)

	// Add hook for processing backslashes
	globalLogger = globalLogger.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
		return nil
	}))

	var options []zap.Option

	if globalConfig.AddCaller {
		options = append(options, zap.AddCaller())
		// Set caller skip to correctly display call location
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

	// Replace global logger
	zap.ReplaceGlobals(globalLogger)

	// Log initialization info
	globalLogger.Info("logger initialized",
		zap.String("level", globalConfig.Level),
		zap.String("format", globalConfig.Format),
		zap.Strings("outputs", globalConfig.Outputs),
		zap.String("path", globalConfig.LogPath),
		zap.String("file", globalConfig.FileName),
	)
}

// buildConsoleCore builds the console output core
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

// buildFileCore builds the file output core
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

	// Ensure log directory exists
	logDir := globalConfig.LogPath
	if logDir == "" {
		logDir = "./logs"
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		panic("failed to create log directory: " + err.Error())
	}

	// Determine file name
	var logFileName string
	if globalConfig.FileName != "" {
		logFileName = globalConfig.FileName
	} else {
		now := time.Now()
		logFileName = now.Format("2006-01-02") + ".log"
	}

	// Open log file
	logFilePath := filepath.Join(logDir, logFileName)
	logFile, err := os.OpenFile(
		logFilePath,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		panic("failed to open log file: " + err.Error())
	}

	fileOutput := zapcore.Lock(logFile)
	return zapcore.NewCore(encoder, fileOutput, zapGlobalLevel)
}

// parseLevel parses the log level string
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

// Debug logs a debug message
func Debug(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Debug(msg, fields...)
	}
}

// Info logs an info message
func Info(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Info(msg, fields...)
	}
}

// Warn logs a warning message
func Warn(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Warn(msg, fields...)
	}
}

// Error logs an error message
func Error(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Error(msg, fields...)
	}
}

// Fatal logs a fatal error and exits the program
func Fatal(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Fatal(msg, fields...)
	}
}

// SetLevel dynamically sets the log level
func SetLevel(level string) {
	zapGlobalLevel.SetLevel(parseLevel(level))
	Info("log level changed", String("level", level))
}

// GetLogger returns the raw zap logger
func GetLogger() *zap.Logger {
	return globalLogger
}

// Sync flushes the log buffer
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// processSQLString processes escape characters in SQL strings
func processSQLString(sql string) string {
	sql = strings.ReplaceAll(sql, "\\\\", "\\")
	sql = strings.ReplaceAll(sql, "\\\"", "\"")
	sql = strings.ReplaceAll(sql, "\\'", "'")
	return sql
}
