// Package zlogger provides separate outputs for log severity levels.
//
// DEBUG and INFO go to the info file, WARN to warn, and ERROR or above to error.
// The daily rotation worker stops before Close returns, preventing files from reopening after shutdown.
package zlogger

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type writeSyncCloser interface {
	io.Writer
	Sync() error
	Close() error
}

// ErrInvalidSplitCore indicates a missing encoder or sink.
var ErrInvalidSplitCore = errors.New("invalid split core configuration")

// SplitSinks defines three outputs for standard log severity levels.
//
// Info receives DEBUG and INFO, Warn receives WARN, and Error receives ERROR or above.
// Use distinct sinks for the three fields. Resource ownership remains with the caller.
type SplitSinks struct {
	Info  zapcore.WriteSyncer
	Warn  zapcore.WriteSyncer
	Error zapcore.WriteSyncer
}

// NewSplitCore creates a severity-routing core with the caller's encoder and sinks.
//
// It clones the encoder for each output and neither takes ownership of nor closes the sinks.
// Stop using the logger, call Sync, and then close the underlying resources yourself.
func NewSplitCore(encoder zapcore.Encoder, sinks SplitSinks) (zapcore.Core, error) {
	if encoder == nil {
		return nil, fmt.Errorf("%w: encoder must not be nil", ErrInvalidSplitCore)
	}
	if sinks.Info == nil {
		return nil, fmt.Errorf("%w: info sink must not be nil", ErrInvalidSplitCore)
	}
	if sinks.Warn == nil {
		return nil, fmt.Errorf("%w: warn sink must not be nil", ErrInvalidSplitCore)
	}
	if sinks.Error == nil {
		return nil, fmt.Errorf("%w: error sink must not be nil", ErrInvalidSplitCore)
	}
	return buildSplitCore(encoder, sinks), nil
}

func buildSplitCore(encoder zapcore.Encoder, sinks SplitSinks) zapcore.Core {
	return buildSplitCoreWithRedaction(encoder, sinks, RedactionConfig{})
}

// The high-level caller validates the policy before opening any split output.
func buildSplitCoreWithRedaction(encoder zapcore.Encoder, sinks SplitSinks, cfg RedactionConfig) zapcore.Core {
	infoLevel := zap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level == zapcore.DebugLevel || level == zapcore.InfoLevel
	})
	warnLevel := zap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level == zapcore.WarnLevel
	})
	errorLevel := zap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level >= zapcore.ErrorLevel
	})

	return zapcore.NewTee(
		newValidatedRedactingCore(encoder.Clone(), sinks.Info, infoLevel, cfg),
		newValidatedRedactingCore(encoder.Clone(), sinks.Warn, warnLevel, cfg),
		newValidatedRedactingCore(encoder.Clone(), sinks.Error, errorLevel, cfg),
	)
}

type splitFileSet struct {
	info  writeSyncCloser
	warn  writeSyncCloser
	error writeSyncCloser
}

func (f splitFileSet) close() error {
	var closeErrs []error
	if f.info != nil {
		closeErrs = append(closeErrs, f.info.Close())
	}
	if f.warn != nil {
		closeErrs = append(closeErrs, f.warn.Close())
	}
	if f.error != nil {
		closeErrs = append(closeErrs, f.error.Close())
	}
	return errors.Join(closeErrs...)
}

func (f splitFileSet) sync() error {
	var syncErrs []error
	if f.info != nil {
		syncErrs = append(syncErrs, f.info.Sync())
	}
	if f.warn != nil {
		syncErrs = append(syncErrs, f.warn.Sync())
	}
	if f.error != nil {
		syncErrs = append(syncErrs, f.error.Sync())
	}
	return errors.Join(syncErrs...)
}

type splitFileOpener func(directory, filePrefix, date string) (splitFileSet, error)

type splitFilePermissionOpener func(
	directory string,
	filePrefix string,
	date string,
	filePerm os.FileMode,
) (splitFileSet, error)

type rotationTimer interface {
	C() <-chan time.Time
	Stop() bool
}

type rotationClock interface {
	Now() time.Time
	NewTimer(time.Duration) rotationTimer
}

type systemRotationClock struct{}

func (systemRotationClock) Now() time.Time {
	return time.Now()
}

func (systemRotationClock) NewTimer(duration time.Duration) rotationTimer {
	return &systemRotationTimer{timer: time.NewTimer(duration)}
}

type systemRotationTimer struct {
	timer *time.Timer
}

func (t *systemRotationTimer) C() <-chan time.Time {
	return t.timer.C
}

func (t *systemRotationTimer) Stop() bool {
	return t.timer.Stop()
}

// SplitOutput writes each log level to its corresponding file.
//
// Level routing:
//   - DEBUG、INFO：{prefix}-info-{date}.log
//   - WARN：{prefix}-warn-{date}.log
//   - ERROR、DPANIC、PANIC、FATAL：{prefix}-error-{date}.log
type SplitOutput struct {
	directory  string
	filePrefix string
	infoOut    writeSyncCloser
	warnOut    writeSyncCloser
	errorOut   writeSyncCloser

	mutex     sync.Mutex
	closed    bool
	closeOnce sync.Once
	closeErr  error
	stop      chan struct{}
	done      chan struct{}
	clock     rotationClock
	opener    splitFilePermissionOpener
	settings  fileOutputSettings
}

// NewSplitOutput creates severity-based file outputs and starts a daily rotation worker.
// filePrefix must be a single leaf name. Unsafe paths return ErrUnsafeLogPath.
func NewSplitOutput(directory, filePrefix string) (*SplitOutput, error) {
	return NewSplitOutputWithOptions(directory, filePrefix)
}

// NewSplitOutputWithOptions configures creation permissions for severity-based outputs.
//
// With no options it behaves like NewSplitOutput. Resolved permissions also apply during daily rotation.
// The process umask can further restrict permissions. Existing permissions are not changed.
func NewSplitOutputWithOptions(
	directory string,
	filePrefix string,
	opts ...FileOutputOption,
) (*SplitOutput, error) {
	settings, err := resolveFileOutputOptions(opts...)
	if err != nil {
		return nil, err
	}
	return newSplitOutputWithSettings(
		directory,
		filePrefix,
		systemRotationClock{},
		openSplitFilesWithPermissions,
		settings,
	)
}

func newSplitOutput(
	directory string,
	filePrefix string,
	clock rotationClock,
	opener splitFileOpener,
) (*SplitOutput, error) {
	settings, err := resolveFileOutputOptions()
	if err != nil {
		return nil, err
	}
	var permissionOpener splitFilePermissionOpener
	if opener != nil {
		permissionOpener = func(
			directory string,
			filePrefix string,
			date string,
			_ os.FileMode,
		) (splitFileSet, error) {
			return opener(directory, filePrefix, date)
		}
	}
	return newSplitOutputWithSettings(
		directory,
		filePrefix,
		clock,
		permissionOpener,
		settings,
	)
}

func newSplitOutputWithSettings(
	directory string,
	filePrefix string,
	clock rotationClock,
	opener splitFilePermissionOpener,
	settings fileOutputSettings,
) (*SplitOutput, error) {
	if err := validateLogLeaf(filePrefix, true); err != nil {
		return nil, fmt.Errorf("validate split log prefix: %w", err)
	}
	if err := validateFileOutputPermission("directory", settings.dirPerm, 0o700); err != nil {
		return nil, err
	}
	if err := validateFileOutputPermission("file", settings.filePerm, 0o600); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(directory, settings.dirPerm); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	output := &SplitOutput{
		directory:  directory,
		filePrefix: filePrefix,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
		clock:      clock,
		opener:     opener,
		settings:   settings,
	}
	if err := output.openFiles(); err != nil {
		return nil, err
	}

	go output.rotateDaily()
	return output, nil
}

func openSplitFiles(directory, filePrefix, date string) (splitFileSet, error) {
	return openSplitFilesWithPermissions(
		directory,
		filePrefix,
		date,
		defaultLogFileMode,
	)
}

func openSplitFilesWithPermissions(
	directory string,
	filePrefix string,
	date string,
	filePerm os.FileMode,
) (splitFileSet, error) {
	opened, err := openRootedLogFilesWithPermissions(
		directory,
		filePerm,
		filePrefix+"-info-"+date+".log",
		filePrefix+"-warn-"+date+".log",
		filePrefix+"-error-"+date+".log",
	)
	if err != nil {
		return splitFileSet{}, fmt.Errorf("open split log files: %w", err)
	}
	if len(opened) != 3 {
		return splitFileSet{}, errors.Join(
			fmt.Errorf("got %d split log files, expected 3: %w", len(opened), os.ErrInvalid),
			closeRootedLogFiles(opened),
		)
	}

	return splitFileSet{
		info:  opened[0],
		warn:  opened[1],
		error: opened[2],
	}, nil
}

func (s *SplitOutput) currentFiles() splitFileSet {
	return splitFileSet{
		info:  s.infoOut,
		warn:  s.warnOut,
		error: s.errorOut,
	}
}

func (s *SplitOutput) replaceFiles(files splitFileSet) splitFileSet {
	previous := s.currentFiles()
	s.infoOut = files.info
	s.warnOut = files.warn
	s.errorOut = files.error
	return previous
}

func (s *SplitOutput) openFiles() error {
	s.mutex.Lock()
	if s.closed {
		s.mutex.Unlock()
		return fmt.Errorf("split output is closed: %w", os.ErrClosed)
	}
	clock := s.clock
	opener := s.opener
	filePerm := s.settings.filePerm
	s.mutex.Unlock()

	if clock == nil || opener == nil {
		return fmt.Errorf("split output is not initialized: %w", os.ErrInvalid)
	}

	date := clock.Now().Format("2006-01-02")
	newFiles, err := opener(s.directory, s.filePrefix, date, filePerm)
	if err != nil {
		return err
	}

	s.mutex.Lock()
	if s.closed {
		s.mutex.Unlock()
		return errors.Join(
			fmt.Errorf("split output is closed: %w", os.ErrClosed),
			newFiles.close(),
		)
	}
	previous := s.replaceFiles(newFiles)
	s.mutex.Unlock()

	return previous.close()
}

func (s *SplitOutput) rotateDaily() {
	defer close(s.done)

	for {
		now := s.clock.Now()
		next := now.Add(24 * time.Hour)
		next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
		timer := s.clock.NewTimer(next.Sub(now))

		select {
		case <-s.stop:
			timer.Stop()
			return
		case <-timer.C():
			if err := s.openFiles(); err != nil {
				if errors.Is(err, os.ErrClosed) {
					return
				}
				s.settings.observer.report("rotate", "split", err)
				fmt.Fprintf(os.Stderr, "daily log rotation failed: %v\n", err)
			}
		}
	}
}

// Write sends data to the file for the supplied log level.
func (s *SplitOutput) Write(level zapcore.Level, data []byte) (n int, err error) {
	// Defers run in reverse order: release the output lock before reporting to keep notification outside the critical section.
	defer func() { s.settings.observer.report("write", "split", err) }()
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.closed {
		return 0, fmt.Errorf("split output is closed: %w", os.ErrClosed)
	}

	var output io.Writer
	switch level {
	case zapcore.InfoLevel:
		output = s.infoOut
	case zapcore.WarnLevel:
		output = s.warnOut
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		output = s.errorOut
	default:
		output = s.infoOut
	}
	if output == nil {
		return 0, fmt.Errorf("split output is not initialized: %w", os.ErrInvalid)
	}
	return output.Write(data)
}

// Sync flushes all severity-based log files to storage.
func (s *SplitOutput) Sync() (err error) {
	defer func() { s.settings.observer.report("sync", "split", err) }()
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.closed {
		return fmt.Errorf("split output is closed: %w", os.ErrClosed)
	}
	return s.currentFiles().sync()
}

func (s *SplitOutput) syncLevel(level zapcore.Level) (err error) {
	defer func() { s.settings.observer.report("sync", "split", err) }()
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.closed {
		return fmt.Errorf("split output is closed: %w", os.ErrClosed)
	}

	var output writeSyncCloser
	switch level {
	case zapcore.WarnLevel:
		output = s.warnOut
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		output = s.errorOut
	default:
		output = s.infoOut
	}
	if output == nil {
		return fmt.Errorf("split output is not initialized: %w", os.ErrInvalid)
	}
	return output.Sync()
}

// Close stops the daily rotation worker and closes all severity-based log files.
func (s *SplitOutput) Close() error {
	s.closeOnce.Do(func() {
		s.mutex.Lock()
		s.closed = true
		files := s.replaceFiles(splitFileSet{})
		stop := s.stop
		done := s.done
		if stop != nil {
			close(stop)
		}
		s.mutex.Unlock()

		if done != nil {
			<-done
		}
		s.closeErr = files.close()
		s.settings.observer.report("close", "split", s.closeErr)
	})
	return s.closeErr
}

type splitOutputWrapper struct {
	so  *SplitOutput
	lvl zapcore.Level
}

func (w *splitOutputWrapper) Write(data []byte) (int, error) {
	return w.so.Write(w.lvl, data)
}

func (w *splitOutputWrapper) Sync() error {
	return w.so.syncLevel(w.lvl)
}

// GetSplitCore creates a zap core with separate files by severity.
//
// Cleanup synchronously stops rotation and closes all files. Stop using the logger before calling it.
func GetSplitCore(
	directory string,
	filePrefix string,
	encoderConfig zapcore.EncoderConfig,
) (zapcore.Core, func(), error) {
	return GetSplitCoreWithOptions(directory, filePrefix, encoderConfig)
}

// GetSplitCoreWithOptions configures file creation permissions for a severity-routing zap core.
//
// With no options it behaves like GetSplitCore. The cleanup contract is unchanged.
func GetSplitCoreWithOptions(
	directory string,
	filePrefix string,
	encoderConfig zapcore.EncoderConfig,
	opts ...FileOutputOption,
) (zapcore.Core, func(), error) {
	core, cleanup, err := GetSplitCoreWithCleanup(directory, filePrefix, encoderConfig, opts...)
	if err != nil {
		return nil, nil, err
	}
	return core, func() { _ = cleanup() }, nil
}

// GetSplitCoreWithCleanup creates a daily three-file core and preserves cleanup close errors.
// Stop using the logger and call Sync before cleanup. Cleanup supports repeated and concurrent calls.
func GetSplitCoreWithCleanup(
	directory, filePrefix string,
	encoderConfig zapcore.EncoderConfig,
	opts ...FileOutputOption,
) (zapcore.Core, func() error, error) {
	splitOut, err := NewSplitOutputWithOptions(directory, filePrefix, opts...)
	if err != nil {
		return nil, nil, err
	}

	return splitCoreForOutput(zapcore.NewJSONEncoder(encoderConfig), splitOut), splitOut.Close, nil
}

func splitCoreForOutput(encoder zapcore.Encoder, splitOut *SplitOutput) zapcore.Core {
	return splitCoreForOutputWithRedaction(encoder, splitOut, RedactionConfig{})
}

func splitCoreForOutputWithRedaction(encoder zapcore.Encoder, splitOut *SplitOutput, cfg RedactionConfig) zapcore.Core {
	infoOut := zapcore.AddSync(&splitOutputWrapper{so: splitOut, lvl: zapcore.InfoLevel})
	warnOut := zapcore.AddSync(&splitOutputWrapper{so: splitOut, lvl: zapcore.WarnLevel})
	errorOut := zapcore.AddSync(&splitOutputWrapper{so: splitOut, lvl: zapcore.ErrorLevel})
	return buildSplitCoreWithRedaction(encoder, SplitSinks{
		Info:  infoOut,
		Warn:  warnOut,
		Error: errorOut,
	}, cfg)
}
