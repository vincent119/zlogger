package zlogger

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// ErrAlreadyConfigured 表示全域 logger 已完成一次成功設定。
	ErrAlreadyConfigured = errors.New("全域 logger 已完成設定")

	globalLogger   *zap.Logger
	zapGlobalLevel = zap.NewAtomicLevel()
	globalConfig   *Config

	configureMu   sync.Mutex
	configured    bool
	globalCleanup func() error
)

// Field 是 zap.Field 的別名。
type Field = zap.Field

// Instance 持有非全域 logger 與其擁有的資源。
type Instance struct {
	logger  *zap.Logger
	level   zap.AtomicLevel
	closers []io.Closer

	mu        sync.RWMutex
	closed    bool
	closeOnce sync.Once
	closeErr  error
}

// Logger 回傳底層 zap logger。
// 呼叫端不得在 Close 後繼續使用回傳值。
func (i *Instance) Logger() *zap.Logger {
	if i == nil {
		return nil
	}
	return i.logger
}

// Sync 將目前 Instance 的緩衝資料同步至輸出。
func (i *Instance) Sync() error {
	if i == nil {
		return nil
	}

	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.closed {
		return fmt.Errorf("同步 logger instance: %w", os.ErrClosed)
	}
	return i.logger.Sync()
}

// Close 關閉 Instance 擁有的資源，且可安全重複及並行呼叫。
func (i *Instance) Close() error {
	if i == nil {
		return nil
	}

	i.closeOnce.Do(func() {
		i.mu.Lock()
		i.closed = true
		closers := slices.Clone(i.closers)
		i.mu.Unlock()

		i.closeErr = closeOwnedResources(closers, "關閉 logger 資源")
	})

	return i.closeErr
}

// sqlProcessingCore 會處理 SQL 欄位中的跳脫字元。
type sqlProcessingCore struct {
	zapcore.Core
}

// With 實作 zapcore.Core。
func (c *sqlProcessingCore) With(fields []zapcore.Field) zapcore.Core {
	for i := range fields {
		if fields[i].Key == "sql" && fields[i].Type == zapcore.StringType {
			fields[i].String = processSQLString(fields[i].String)
		}
	}
	return &sqlProcessingCore{Core: c.Core.With(fields)}
}

// Check 實作 zapcore.Core。
func (c *sqlProcessingCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

// Write 實作 zapcore.Core。
func (c *sqlProcessingCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	ent.Message = strings.ReplaceAll(ent.Message, "\\", "")

	for i := range fields {
		if fields[i].Key == "sql" && fields[i].Type == zapcore.StringType {
			fields[i].String = processSQLString(fields[i].String)
		}
	}

	return c.Core.Write(ent, fields)
}

// New 建立不修改全域狀態的 logger Instance。
// nil Config 會使用 DefaultConfig。
func New(cfg *Config) (*Instance, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	} else {
		cfg = cfg.normalizedCopy()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	level := zap.NewAtomicLevelAt(parseLevel(cfg.Level))
	encoderConfig := buildEncoderConfig(cfg)
	cores := make([]zapcore.Core, 0, len(cfg.Outputs))
	closers := make([]io.Closer, 0, 1)

	rollback := func(buildErr error) error {
		return errors.Join(buildErr, closeOwnedResources(closers, "回收 logger 資源"))
	}

	for _, output := range cfg.Outputs {
		switch output {
		case "console":
			cores = append(cores, newConsoleCore(cfg, encoderConfig, level))
		case "file":
			core, file, err := newFileCore(cfg, encoderConfig, level)
			if err != nil {
				return nil, rollback(err)
			}
			closers = append(closers, file)
			cores = append(cores, core)
		}
	}

	logger := zap.New(zapcore.NewTee(cores...))
	options := make([]zap.Option, 0, 3)
	if cfg.AddCaller {
		options = append(options, zap.AddCaller(), zap.AddCallerSkip(1))
	}
	if cfg.AddStacktrace {
		options = append(options, zap.AddStacktrace(zapcore.ErrorLevel))
	}
	if cfg.Development {
		options = append(options, zap.Development())
	}
	if len(options) > 0 {
		logger = logger.WithOptions(options...)
	}

	instance := &Instance{
		logger:  logger,
		level:   level,
		closers: closers,
	}
	instance.logger.Info("logger initialized",
		zap.String("level", cfg.Level),
		zap.String("format", cfg.Format),
		zap.Strings("outputs", slices.Clone(cfg.Outputs)),
		zap.String("path", cfg.LogPath),
		zap.String("file", cfg.FileName),
	)

	return instance, nil
}

// Configure 建立並發布全域 logger，失敗時不修改既有全域狀態。
func Configure(patch *ConfigPatch) (func() error, error) {
	cfg, err := patch.Resolve()
	if err != nil {
		return nil, err
	}
	return configureResolved(cfg)
}

// Init 保留既有初始化入口。
//
// Deprecated: 新程式應使用 Configure 取得可處理的錯誤與 cleanup。
func Init(cfg *Config) {
	merged := DefaultConfig().Merge(cfg).normalizedCopy()
	_, err := configureResolved(merged)
	if err == nil || errors.Is(err, ErrAlreadyConfigured) {
		return
	}
	panic(err)
}

// initLogger 保留既有 package-private 測試入口。
func initLogger(cfg *Config) {
	Init(cfg)
}

func configureResolved(cfg *Config) (func() error, error) {
	configureMu.Lock()
	defer configureMu.Unlock()
	if configured {
		return nil, ErrAlreadyConfigured
	}

	instance, err := New(cfg)
	if err != nil {
		return nil, err
	}

	previousLogger := globalLogger
	previousConfig := globalConfig
	previousLevel := zapGlobalLevel
	restoreZapGlobals := zap.ReplaceGlobals(instance.logger)

	globalLogger = instance.logger
	globalConfig = cfg.normalizedCopy()
	zapGlobalLevel = instance.level
	configured = true

	var cleanupOnce sync.Once
	var cleanupErr error
	cleanup := func() error {
		cleanupOnce.Do(func() {
			configureMu.Lock()
			globalLogger = previousLogger
			globalConfig = previousConfig
			zapGlobalLevel = previousLevel
			restoreZapGlobals()
			configureMu.Unlock()

			cleanupErr = instance.Close()
		})
		return cleanupErr
	}
	globalCleanup = cleanup

	return cleanup, nil
}

func buildEncoderConfig(cfg *Config) zapcore.EncoderConfig {
	var levelEncoder zapcore.LevelEncoder
	if cfg.ColorEnabled {
		levelEncoder = zapcore.CapitalColorLevelEncoder
	} else {
		levelEncoder = zapcore.CapitalLevelEncoder
	}

	return zapcore.EncoderConfig{
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
}

func newConsoleCore(cfg *Config, encoderConfig zapcore.EncoderConfig, level zap.AtomicLevel) zapcore.Core {
	encoder := newEncoder(cfg.Format, encoderConfig)
	return zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), level)
}

func newFileCore(
	cfg *Config,
	encoderConfig zapcore.EncoderConfig,
	level zap.AtomicLevel,
) (zapcore.Core, *os.File, error) {
	if err := os.MkdirAll(cfg.LogPath, defaultLogDirMode); err != nil {
		return nil, nil, fmt.Errorf("建立日誌目錄 %q: %w", cfg.LogPath, err)
	}

	logFileName := cfg.FileName
	if logFileName == "" {
		logFileName = time.Now().Format("2006-01-02") + ".log"
	}
	logFile, err := openSecureLogFile(cfg.LogPath, logFileName)
	if err != nil {
		return nil, nil, err
	}

	encoder := newEncoder(cfg.Format, encoderConfig)
	return zapcore.NewCore(encoder, zapcore.Lock(logFile), level), logFile, nil
}

func newEncoder(format string, encoderConfig zapcore.EncoderConfig) zapcore.Encoder {
	if format == "json" {
		jsonEncoderConfig := encoderConfig
		jsonEncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format(time.RFC3339))
		}
		return zapcore.NewJSONEncoder(jsonEncoderConfig)
	}
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func closeOwnedResources(closers []io.Closer, operation string) error {
	closeErrs := make([]error, 0, len(closers))
	for index := len(closers) - 1; index >= 0; index-- {
		if err := closers[index].Close(); err != nil {
			closeErrs = append(closeErrs, fmt.Errorf("%s: %w", operation, err))
		}
	}
	return errors.Join(closeErrs...)
}

// buildConsoleCore 保留既有 package-private 測試入口。
func buildConsoleCore(encoderConfig zapcore.EncoderConfig) zapcore.Core {
	cfg := globalConfig
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return newConsoleCore(cfg, encoderConfig, zapGlobalLevel)
}

// buildFileCore 保留既有 package-private 測試入口，並回傳由呼叫端關閉的檔案。
func buildFileCore(encoderConfig zapcore.EncoderConfig) (zapcore.Core, *os.File) {
	cfg := globalConfig
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.LogPath == "" {
		cfg = cfg.normalizedCopy()
		cfg.LogPath = "./logs"
	}
	core, file, err := newFileCore(cfg, encoderConfig, zapGlobalLevel)
	if err != nil {
		panic(err)
	}
	return core, file
}

// parseLevel 解析既有 level 字串；未知值維持回退 info 的 legacy 行為。
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

// Debug 記錄 debug 訊息。
func Debug(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Debug(msg, fields...)
	}
}

// Info 記錄 info 訊息。
func Info(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Info(msg, fields...)
	}
}

// Warn 記錄 warn 訊息。
func Warn(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Warn(msg, fields...)
	}
}

// Error 記錄 error 訊息。
func Error(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Error(msg, fields...)
	}
}

// Fatal 記錄 fatal 訊息並由 zap 結束程序。
func Fatal(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Fatal(msg, fields...)
	}
}

// SetLevel 動態調整全域 logger level。
func SetLevel(level string) {
	zapGlobalLevel.SetLevel(parseLevel(level))
	Info("log level changed", String("level", level))
}

// GetLogger 回傳目前全域 zap logger。
func GetLogger() *zap.Logger {
	return globalLogger
}

// Sync 將全域 logger 的緩衝資料同步至輸出。
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// processSQLString 處理 SQL 字串中的跳脫字元。
func processSQLString(sql string) string {
	sql = strings.ReplaceAll(sql, "\\\\", "\\")
	sql = strings.ReplaceAll(sql, "\\\"", "\"")
	sql = strings.ReplaceAll(sql, "\\'", "'")
	return sql
}
