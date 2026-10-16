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
	// ErrAlreadyConfigured indicates that global configuration has already succeeded.
	ErrAlreadyConfigured = errors.New("global logger is already configured")

	globalLogger   *zap.Logger
	zapGlobalLevel = zap.NewAtomicLevel()
	globalConfig   *Config

	configureMu   sync.Mutex
	configured    bool
	globalCleanup func() error
)

// Field is an alias for zap.Field.
type Field = zap.Field

// Instance owns a non-global logger and its resources.
type Instance struct {
	logger  *zap.Logger
	level   zap.AtomicLevel
	closers []io.Closer

	mu        sync.RWMutex
	closed    bool
	closeOnce sync.Once
	closeErr  error
}

// Logger returns the underlying zap logger.
// The returned logger must not be used after Close.
func (i *Instance) Logger() *zap.Logger {
	if i == nil {
		return nil
	}
	return i.logger
}

// Sync flushes the Instance's buffered data to its outputs.
func (i *Instance) Sync() error {
	if i == nil {
		return nil
	}

	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.closed {
		return fmt.Errorf("sync logger instance: %w", os.ErrClosed)
	}
	return i.logger.Sync()
}

// SetLevel changes only this Instance's minimum level. Invalid input leaves it unchanged.
func (i *Instance) SetLevel(level string) error {
	if i == nil {
		return fmt.Errorf("set instance level: %w", os.ErrClosed)
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.closed {
		return fmt.Errorf("set instance level: %w", os.ErrClosed)
	}
	cfg := DefaultConfig()
	cfg.Level = level
	if err := cfg.Validate(); err != nil {
		return err
	}
	i.level.SetLevel(parseLevel(level))
	return nil
}

// NewSplit creates a non-global Instance owning three daily files. A nil cfg uses defaults.
// Outputs does not select sinks but must pass Validate. LogPath/FileName specify directory/prefix.
// Close syncs before closing owned outputs. Use a distinct directory or prefix for each Instance.
func NewSplit(cfg *Config, opts ...FileOutputOption) (*Instance, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	} else {
		cfg = cfg.normalizedCopy()
	}
	return newSplitInstance(cfg, opts...)
}

func newSplitInstance(cfg *Config, opts ...FileOutputOption) (*Instance, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.LogPath == "" {
		return nil, fmt.Errorf("%w: split LogPath must not be empty", ErrInvalidConfig)
	}
	output, err := NewSplitOutputWithOptions(cfg.LogPath, cfg.FileName, opts...)
	if err != nil {
		return nil, err
	}
	level := zap.NewAtomicLevelAt(parseLevel(cfg.Level))
	core := splitCoreForOutput(newEncoder(cfg.Format, buildEncoderConfig(cfg)), output)
	logger := configuredCoreLogger(core, cfg, level, 0)
	return &Instance{
		logger:  logger,
		level:   level,
		closers: []io.Closer{&splitInstanceResource{output: output}},
	}, nil
}

type splitInstanceResource struct {
	output *SplitOutput
}

func (r *splitInstanceResource) Close() error {
	syncErr := r.output.Sync()
	return errors.Join(syncErr, r.output.Close())
}

type levelGateCore struct {
	zapcore.Core
	level zap.AtomicLevel
}

func (c *levelGateCore) Enabled(level zapcore.Level) bool {
	return c.level.Enabled(level) && c.Core.Enabled(level)
}

func (c *levelGateCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if !c.Enabled(entry.Level) {
		return checked
	}
	return c.Core.Check(entry, checked)
}

func (c *levelGateCore) With(fields []zapcore.Field) zapcore.Core {
	return &levelGateCore{Core: c.Core.With(fields), level: c.level}
}

func configuredCoreLogger(core zapcore.Core, cfg *Config, level zap.AtomicLevel, callerSkip int) *zap.Logger {
	logger := zap.New(&levelGateCore{Core: core, level: level})
	if cfg.AddCaller {
		logger = logger.WithOptions(zap.AddCaller(), zap.AddCallerSkip(callerSkip))
	}
	if cfg.AddStacktrace {
		logger = logger.WithOptions(zap.AddStacktrace(zapcore.ErrorLevel))
	}
	if cfg.Development {
		logger = logger.WithOptions(zap.Development())
	}
	return logger
}

// Close releases owned resources and supports repeated and concurrent calls.
func (i *Instance) Close() error {
	if i == nil {
		return nil
	}

	i.closeOnce.Do(func() {
		i.mu.Lock()
		i.closed = true
		closers := slices.Clone(i.closers)
		i.mu.Unlock()

		i.closeErr = closeOwnedResources(closers, "close logger resources")
	})

	return i.closeErr
}

// New creates a logger Instance without modifying global state.
// A nil Config uses DefaultConfig.
func New(cfg *Config) (*Instance, error) {
	return NewWithOptions(cfg)
}

// NewWithOptions creates an Instance with configurable permissions for new directories and files.
//
// With no options it behaves like New. Options affect only newly created filesystem objects.
// The process umask can further restrict permissions. Existing permissions are not changed.
func NewWithOptions(cfg *Config, opts ...FileOutputOption) (*Instance, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	} else {
		cfg = cfg.normalizedCopy()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	settings, err := resolveFileOutputOptions(opts...)
	if err != nil {
		return nil, err
	}
	if settings.observer != nil {
		return nil, fmt.Errorf("%w: WithOutputObserver requires split output", ErrInvalidConfig)
	}

	level := zap.NewAtomicLevelAt(parseLevel(cfg.Level))
	encoderConfig := buildEncoderConfig(cfg)
	cores := make([]zapcore.Core, 0, len(cfg.Outputs))
	closers := make([]io.Closer, 0, 1)

	rollback := func(buildErr error) error {
		return errors.Join(buildErr, closeOwnedResources(closers, "roll back logger resources"))
	}

	for _, output := range cfg.Outputs {
		switch output {
		case "console":
			cores = append(cores, newConsoleCore(cfg, encoderConfig, level))
		case "file":
			core, file, err := newFileCoreWithSettings(cfg, encoderConfig, level, settings)
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

// Configure publishes a global logger. Failure leaves existing global state unchanged.
func Configure(patch *ConfigPatch) (func() error, error) {
	return ConfigureWithOptions(patch)
}

// ConfigureWithOptions configures a global logger with file creation permissions.
//
// With no options it behaves like Configure. The returned cleanup owns successful resources.
func ConfigureWithOptions(
	patch *ConfigPatch,
	opts ...FileOutputOption,
) (func() error, error) {
	cfg, err := patch.Resolve()
	if err != nil {
		return nil, err
	}
	return configureResolvedWithOptions(cfg, opts...)
}

// Init retains the legacy initialization entry point.
//
// Deprecated: Use Configure to receive errors and a cleanup function.
func Init(cfg *Config) {
	merged := DefaultConfig().Merge(cfg).normalizedCopy()
	_, err := configureResolved(merged)
	if err == nil || errors.Is(err, ErrAlreadyConfigured) {
		return
	}
	panic(err)
}

// initLogger retains the package-private test entry point.
func initLogger(cfg *Config) {
	Init(cfg)
}

func configureResolved(cfg *Config) (func() error, error) {
	return configureResolvedWithOptions(cfg)
}

func configureResolvedWithOptions(
	cfg *Config,
	opts ...FileOutputOption,
) (func() error, error) {
	configureMu.Lock()
	defer configureMu.Unlock()
	if configured {
		return nil, ErrAlreadyConfigured
	}

	instance, err := NewWithOptions(cfg, opts...)
	if err != nil {
		return nil, err
	}
	return publishGlobal(instance, cfg, instance.Close), nil
}

// CoreOption specifies explicit cleanup ownership for a custom core.
type CoreOption func(*coreSettings)

type coreSettings struct {
	cleanup func() error
	err     error
}

// WithCoreCleanup registers a callback to run once after successful configuration. Nil and duplicates are rejected.
// Failed configuration does not invoke the callback. The caller retains resource ownership.
func WithCoreCleanup(cleanup func() error) CoreOption {
	return func(settings *coreSettings) {
		if cleanup == nil || settings.cleanup != nil {
			settings.err = fmt.Errorf("%w: cleanup must not be nil or registered more than once", ErrInvalidConfig)
			return
		}
		settings.cleanup = cleanup
	}
}

// ConfigureCore publishes a custom core to both zlogger and zap globals.
// The core controls encoding and outputs. Settings are resolved, but only Level and logger options affect the core.
// Cleanup syncs before invoking the registered callback. It does not close external sinks automatically.
// SetLevel intersects with the core's filter and cannot enable levels the core rejects.
func ConfigureCore(patch *ConfigPatch, core zapcore.Core, opts ...CoreOption) (func() error, error) {
	cfg, err := patch.Resolve()
	if err != nil {
		return nil, err
	}
	settings := coreSettings{}
	for _, option := range opts {
		if option == nil {
			return nil, fmt.Errorf("%w: core option must not be nil", ErrInvalidConfig)
		}
		option(&settings)
	}
	if settings.err != nil {
		return nil, settings.err
	}
	if core == nil {
		return nil, fmt.Errorf("%w: core must not be nil", ErrInvalidConfig)
	}
	configureMu.Lock()
	defer configureMu.Unlock()
	if configured {
		return nil, ErrAlreadyConfigured
	}
	level := zap.NewAtomicLevelAt(parseLevel(cfg.Level))
	instance := &Instance{logger: configuredCoreLogger(core, cfg, level, 1), level: level}
	return publishGlobal(instance, cfg, func() error {
		syncErr := instance.Sync()
		if settings.cleanup == nil {
			return syncErr
		}
		return errors.Join(syncErr, settings.cleanup())
	}), nil
}

// ConfigureSplit publishes three daily files globally. Cleanup syncs, stops the worker, and closes files.
// LogPath/FileName specify directory/prefix. Outputs does not select sinks but must pass Resolve.
func ConfigureSplit(patch *ConfigPatch, opts ...FileOutputOption) (func() error, error) {
	cfg, err := patch.Resolve()
	if err != nil {
		return nil, err
	}
	configureMu.Lock()
	defer configureMu.Unlock()
	if configured {
		return nil, ErrAlreadyConfigured
	}
	instance, err := newSplitInstance(cfg, opts...)
	if err != nil {
		return nil, err
	}
	if cfg.AddCaller {
		instance.logger = instance.logger.WithOptions(zap.AddCallerSkip(1))
	}
	return publishGlobal(instance, cfg, instance.Close), nil
}

// The caller holds configureMu so that only a fully constructed logger can claim successful configuration.
func publishGlobal(instance *Instance, cfg *Config, closeLogger func() error) func() error {
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
			cleanupErr = closeLogger()
			configureMu.Lock()
			globalLogger = previousLogger
			globalConfig = previousConfig
			zapGlobalLevel = previousLevel
			restoreZapGlobals()
			configureMu.Unlock()
		})
		return cleanupErr
	}
	globalCleanup = cleanup

	return cleanup
}

func buildEncoderConfig(cfg *Config) zapcore.EncoderConfig {
	var levelEncoder zapcore.LevelEncoder
	if cfg.Format == "console" && cfg.ColorEnabled {
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
	settings, err := resolveFileOutputOptions()
	if err != nil {
		return nil, nil, err
	}
	return newFileCoreWithSettings(cfg, encoderConfig, level, settings)
}

func newFileCoreWithSettings(
	cfg *Config,
	encoderConfig zapcore.EncoderConfig,
	level zap.AtomicLevel,
	settings fileOutputSettings,
) (zapcore.Core, *os.File, error) {
	if err := os.MkdirAll(cfg.LogPath, settings.dirPerm); err != nil {
		return nil, nil, fmt.Errorf("create log directory %q: %w", cfg.LogPath, err)
	}

	logFileName := cfg.FileName
	if logFileName == "" {
		logFileName = time.Now().Format("2006-01-02") + ".log"
	}
	logFiles, err := openRootedLogFilesWithPermissions(cfg.LogPath, settings.filePerm, logFileName)
	if err != nil {
		return nil, nil, err
	}
	if len(logFiles) != 1 {
		return nil, nil, errors.Join(
			fmt.Errorf("got %d log files, expected 1: %w", len(logFiles), os.ErrInvalid),
			closeRootedLogFiles(logFiles),
		)
	}
	logFile := logFiles[0]

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

// buildConsoleCore retains the package-private test entry point.
func buildConsoleCore(encoderConfig zapcore.EncoderConfig) zapcore.Core {
	cfg := globalConfig
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return newConsoleCore(cfg, encoderConfig, zapGlobalLevel)
}

// buildFileCore retains the package-private test entry point and returns a file for the caller to close.
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

// parseLevel retains the legacy info fallback for unknown level strings.
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

// Debug logs a debug message.
func Debug(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Debug(msg, fields...)
	}
}

// Info logs an info message.
func Info(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Info(msg, fields...)
	}
}

// Warn logs a warning message.
func Warn(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Warn(msg, fields...)
	}
}

// Error logs an error message.
func Error(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Error(msg, fields...)
	}
}

// Fatal logs a fatal message and lets zap terminate the process.
func Fatal(msg string, fields ...Field) {
	if globalLogger != nil {
		globalLogger.Fatal(msg, fields...)
	}
}

// SetLevel changes the global logger's level dynamically.
func SetLevel(level string) {
	zapGlobalLevel.SetLevel(parseLevel(level))
	Info("log level changed", String("level", level))
}

// GetLogger returns the current global zap logger.
func GetLogger() *zap.Logger {
	return globalLogger
}

// Sync flushes the global logger's buffered data to its outputs.
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}
