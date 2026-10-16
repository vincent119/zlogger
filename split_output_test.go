package zlogger

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type recordingWriteSyncCloser struct {
	mu         sync.Mutex
	closeCalls int
	syncCalls  int
	closeErr   error
	syncErr    error
}

func TestSplitObserverCloseOnce(t *testing.T) {
	o := newTestObserver(t, 8)
	want := errors.New("file close failed")
	sink := &recordingWriteSyncCloser{closeErr: want}
	s := &SplitOutput{infoOut: sink, settings: fileOutputSettings{observer: o}}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if err := s.Close(); !errors.Is(err, want) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if len(o.Events()) != 1 || (<-o.Events()).Operation != "close" {
		t.Fatal("Close must emit only one event")
	}
	_, closes := sink.counts()
	if closes != 1 {
		t.Fatal(closes)
	}
}

func TestSplitObserverOperations(t *testing.T) {
	o := newTestObserver(t, 16)
	want := errors.New("sync failed")
	s := &SplitOutput{infoOut: &recordingWriteSyncCloser{syncErr: want}, settings: fileOutputSettings{observer: o}}
	if err := s.Sync(); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if err := s.syncLevel(zap.InfoLevel); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if err := s.syncLevel(zap.WarnLevel); !errors.Is(err, os.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Write(zap.ErrorLevel, nil); !errors.Is(err, os.ErrInvalid) {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write(zap.InfoLevel, nil); !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
	if err := s.Sync(); !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
	if err := s.syncLevel(zap.InfoLevel); !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
	if len(o.Events()) != 7 {
		t.Fatalf("event count=%d", len(o.Events()))
	}
	for range 7 {
		if event := <-o.Events(); event.Output != "split" {
			t.Fatal(event)
		}
	}
}

func TestSplitObserverWriteFailure(t *testing.T) {
	o := newTestObserver(t, 1)
	want := errors.New("disk write failed")
	s := &SplitOutput{infoOut: failingObserverSink{writeErr: want}, settings: fileOutputSettings{observer: o}}
	if n, err := s.Write(zap.InfoLevel, []byte("private message")); n != 0 || !errors.Is(err, want) {
		t.Fatalf("Write=(%d,%v)", n, err)
	}
	event := <-o.Events()
	if event.Operation != "write" || event.Output != "split" || !errors.Is(event.Err, want) {
		t.Fatal(event)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSplitObserverRotationFailure(t *testing.T) {
	o := newTestObserver(t, 1)
	now := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	clock := newManualRotationClock(now)
	want := errors.New("open new log failed")
	calls := 0
	opener := func(directory, prefix, date string, _ os.FileMode) (splitFileSet, error) {
		calls++
		if calls > 1 {
			return splitFileSet{}, want
		}
		return openSplitFiles(directory, prefix, date)
	}
	settings, err := resolveFileOutputOptions(WithOutputObserver(o))
	if err != nil {
		t.Fatal(err)
	}
	s, err := newSplitOutputWithSettings(t.TempDir(), "app", clock, opener, settings)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	timer := clock.nextTimer(t)
	clock.setNow(now.AddDate(0, 0, 1))
	timer.fire(clock.Now())
	select {
	case event := <-o.Events():
		if event.Operation != "rotate" || !errors.Is(event.Err, want) {
			t.Fatal(event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for rotation event")
	}
	if _, err := s.Write(zap.InfoLevel, []byte("old file remains writable")); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.done:
	default:
		t.Fatal("worker did not stop")
	}
}

func TestGetSplitCoreWithCleanupConcurrent(t *testing.T) {
	core, cleanup, err := GetSplitCoreWithCleanup(t.TempDir(), "app", zap.NewProductionEncoderConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := core.Sync(); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if err := cleanup(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if err := core.Sync(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("Sync after close = %v", err)
	}
}

func TestGetSplitCoreWithCleanupReturnsCloseErrors(t *testing.T) {
	core, cleanup, err := GetSplitCoreWithCleanup(t.TempDir(), "app", zap.NewProductionEncoderConfig())
	if err != nil {
		t.Fatal(err)
	}
	// The public constructor uses real files. Error injection separately verifies the delegated SplitOutput.Close callback.
	if err := core.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	closeErr := errors.New("file close failed")
	output := &SplitOutput{infoOut: &recordingWriteSyncCloser{closeErr: closeErr}}
	errorCleanup := output.Close
	for range 2 {
		if err := errorCleanup(); !errors.Is(err, closeErr) {
			t.Fatalf("cleanup = %v", err)
		}
	}
}

type recordingSplitSink struct {
	mu         sync.Mutex
	buffer     bytes.Buffer
	syncCalls  int
	closeCalls int
}

func (s *recordingSplitSink) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buffer.Write(data)
}

func (s *recordingSplitSink) Sync() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncCalls++
	return nil
}

func (s *recordingSplitSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeCalls++
	return nil
}

func (s *recordingSplitSink) snapshot() (content string, syncCalls, closeCalls int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buffer.String(), s.syncCalls, s.closeCalls
}

type manualRotationTimer struct {
	ch      chan time.Time
	mu      sync.Mutex
	stopped bool
}

func newManualRotationTimer() *manualRotationTimer {
	return &manualRotationTimer{ch: make(chan time.Time, 1)}
}

func (t *manualRotationTimer) C() <-chan time.Time {
	return t.ch
}

func (t *manualRotationTimer) Stop() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	wasActive := !t.stopped
	t.stopped = true
	return wasActive
}

func (t *manualRotationTimer) fire(at time.Time) {
	t.ch <- at
}

func (t *manualRotationTimer) isStopped() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stopped
}

type manualRotationClock struct {
	mu     sync.Mutex
	now    time.Time
	timers chan *manualRotationTimer
}

func newManualRotationClock(now time.Time) *manualRotationClock {
	return &manualRotationClock{
		now:    now,
		timers: make(chan *manualRotationTimer, 4),
	}
}

func (c *manualRotationClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *manualRotationClock) NewTimer(time.Duration) rotationTimer {
	timer := newManualRotationTimer()
	c.timers <- timer
	return timer
}

func (c *manualRotationClock) setNow(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = now
}

func (c *manualRotationClock) nextTimer(t *testing.T) *manualRotationTimer {
	t.Helper()
	select {
	case timer := <-c.timers:
		return timer
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for rotation timer")
		return nil
	}
}

func (r *recordingWriteSyncCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (r *recordingWriteSyncCloser) Sync() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.syncCalls++
	return r.syncErr
}

func (r *recordingWriteSyncCloser) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeCalls++
	return r.closeErr
}

func (r *recordingWriteSyncCloser) counts() (syncCalls, closeCalls int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.syncCalls, r.closeCalls
}

func TestNewSplitCoreRoutesLevels(t *testing.T) {
	infoSink := &recordingSplitSink{}
	warnSink := &recordingSplitSink{}
	errorSink := &recordingSplitSink{}
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{MessageKey: "message"})

	core, err := NewSplitCore(encoder, SplitSinks{
		Info:  infoSink,
		Warn:  warnSink,
		Error: errorSink,
	})
	if err != nil {
		t.Fatalf("create generic split core: %v", err)
	}

	tests := []struct {
		name    string
		level   zapcore.Level
		message string
		target  string
	}{
		{name: "debug", level: zapcore.DebugLevel, message: "debug-message", target: "info"},
		{name: "info", level: zapcore.InfoLevel, message: "info-message", target: "info"},
		{name: "warn", level: zapcore.WarnLevel, message: "warn-message", target: "warn"},
		{name: "error", level: zapcore.ErrorLevel, message: "error-message", target: "error"},
		{name: "dpanic", level: zapcore.DPanicLevel, message: "dpanic-message", target: "error"},
		{name: "panic", level: zapcore.PanicLevel, message: "panic-message", target: "error"},
		{name: "fatal", level: zapcore.FatalLevel, message: "fatal-message", target: "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checked := core.Check(zapcore.Entry{Level: tt.level, Message: tt.message}, nil)
			if checked == nil {
				t.Fatalf("level %s must be enabled", tt.level)
			}
			checked.Write()
		})
	}

	infoContent, _, _ := infoSink.snapshot()
	warnContent, _, _ := warnSink.snapshot()
	errorContent, _, _ := errorSink.snapshot()
	contents := map[string]string{
		"info":  infoContent,
		"warn":  warnContent,
		"error": errorContent,
	}
	for _, tt := range tests {
		for sinkName, content := range contents {
			contains := strings.Contains(content, tt.message)
			if sinkName == tt.target && !contains {
				t.Errorf("%s must be written to %s sink", tt.message, tt.target)
			}
			if sinkName != tt.target && contains {
				t.Errorf("%s must not be written to %s sink", tt.message, sinkName)
			}
		}
	}
}

func TestNewSplitCoreRejectsInvalidInputs(t *testing.T) {
	newEncoder := func() zapcore.Encoder {
		return zapcore.NewJSONEncoder(zapcore.EncoderConfig{MessageKey: "message"})
	}
	newSinks := func() SplitSinks {
		return SplitSinks{
			Info:  &recordingSplitSink{},
			Warn:  &recordingSplitSink{},
			Error: &recordingSplitSink{},
		}
	}

	tests := []struct {
		name    string
		encoder zapcore.Encoder
		sinks   SplitSinks
	}{
		{name: "missing encoder", sinks: newSinks()},
		{name: "missing info", encoder: newEncoder(), sinks: func() SplitSinks {
			sinks := newSinks()
			sinks.Info = nil
			return sinks
		}()},
		{name: "missing warn", encoder: newEncoder(), sinks: func() SplitSinks {
			sinks := newSinks()
			sinks.Warn = nil
			return sinks
		}()},
		{name: "missing error", encoder: newEncoder(), sinks: func() SplitSinks {
			sinks := newSinks()
			sinks.Error = nil
			return sinks
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, err := NewSplitCore(tt.encoder, tt.sinks)
			if core != nil {
				t.Fatal("invalid input must not return a partial core")
			}
			if !errors.Is(err, ErrInvalidSplitCore) {
				t.Fatalf("error = %v, want ErrInvalidSplitCore", err)
			}
		})
	}
}

func TestNewSplitCoreSyncsSinks(t *testing.T) {
	infoSink := &recordingSplitSink{}
	warnSink := &recordingSplitSink{}
	errorSink := &recordingSplitSink{}
	core, err := NewSplitCore(
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{MessageKey: "message"}),
		SplitSinks{Info: infoSink, Warn: warnSink, Error: errorSink},
	)
	if err != nil {
		t.Fatalf("create generic split core: %v", err)
	}

	if err := core.Sync(); err != nil {
		t.Fatalf("sync generic split core: %v", err)
	}
	for name, sink := range map[string]*recordingSplitSink{
		"info": infoSink, "warn": warnSink, "error": errorSink,
	} {
		_, syncCalls, _ := sink.snapshot()
		if syncCalls != 1 {
			t.Errorf("%s sink Sync calls = %d, want 1", name, syncCalls)
		}
	}
}

func TestNewSplitCoreDoesNotCloseSinks(t *testing.T) {
	infoSink := &recordingSplitSink{}
	warnSink := &recordingSplitSink{}
	errorSink := &recordingSplitSink{}
	core, err := NewSplitCore(
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{MessageKey: "message"}),
		SplitSinks{Info: infoSink, Warn: warnSink, Error: errorSink},
	)
	if err != nil {
		t.Fatalf("create generic split core: %v", err)
	}

	if err := core.Sync(); err != nil {
		t.Fatalf("sync generic split core: %v", err)
	}
	for name, sink := range map[string]*recordingSplitSink{
		"info": infoSink, "warn": warnSink, "error": errorSink,
	} {
		_, _, closeCalls := sink.snapshot()
		if closeCalls != 0 {
			t.Errorf("%s sink Close calls = %d, want 0", name, closeCalls)
		}
	}
}

func TestNewSplitOutput(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	so, err := NewSplitOutput(tmpDir, "test")
	if err != nil {
		t.Fatalf("NewSplitOutput failed: %v", err)
	}
	defer func() { _ = so.Close() }()

	if so.directory != tmpDir {
		t.Errorf("expected directory %s, got %s", tmpDir, so.directory)
	}
	if so.filePrefix != "test" {
		t.Errorf("expected filePrefix 'test', got %s", so.filePrefix)
	}
}

func TestNewSplitOutputRejectsUnsafePrefix(t *testing.T) {
	unsafePrefixes := []string{
		"../outside",
		"/tmp/outside",
		"sub/app",
		`sub\app`,
		".",
		"..",
		"app\x00name",
		`C:app`,
		`C:\logs\app`,
	}

	for _, prefix := range unsafePrefixes {
		t.Run(prefix, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "must-not-exist")
			output, err := NewSplitOutput(base, prefix)
			if output != nil {
				_ = output.Close()
			}
			if !errors.Is(err, ErrUnsafeLogPath) {
				t.Fatalf("error = %v, want ErrUnsafeLogPath", err)
			}
			if _, statErr := os.Stat(base); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("unsafe prefix must not create directory, Stat error = %v", statErr)
			}
		})
	}

	output, err := NewSplitOutput(t.TempDir(), "")
	if err != nil {
		t.Fatalf("empty prefix must retain legacy behavior: %v", err)
	}
	if err := output.Close(); err != nil {
		t.Fatalf("close SplitOutput with empty prefix: %v", err)
	}
}

func TestSplitOutput_Write_InfoLevel(t *testing.T) {
	tmpDir := t.TempDir()

	so, err := NewSplitOutput(tmpDir, "app")
	if err != nil {
		t.Fatalf("NewSplitOutput failed: %v", err)
	}
	defer func() { _ = so.Close() }()

	testData := []byte("INFO test log message\n")
	n, err := so.Write(zapcore.InfoLevel, testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
	}

	// Verify file exists
	files, _ := filepath.Glob(filepath.Join(tmpDir, "app-info-*.log"))
	if len(files) == 0 {
		t.Error("expected info log file to be created")
	}
}

func TestSplitOutput_Write_WarnLevel(t *testing.T) {
	tmpDir := t.TempDir()

	so, err := NewSplitOutput(tmpDir, "app")
	if err != nil {
		t.Fatalf("NewSplitOutput failed: %v", err)
	}
	defer func() { _ = so.Close() }()

	testData := []byte("WARN test log message\n")
	n, err := so.Write(zapcore.WarnLevel, testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
	}

	// Verify file exists
	files, _ := filepath.Glob(filepath.Join(tmpDir, "app-warn-*.log"))
	if len(files) == 0 {
		t.Error("expected warn log file to be created")
	}
}

func TestSplitOutput_Write_ErrorLevel(t *testing.T) {
	tmpDir := t.TempDir()

	so, err := NewSplitOutput(tmpDir, "app")
	if err != nil {
		t.Fatalf("NewSplitOutput failed: %v", err)
	}
	defer func() { _ = so.Close() }()

	testData := []byte("ERROR test log message\n")
	n, err := so.Write(zapcore.ErrorLevel, testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
	}

	// Verify file exists
	files, _ := filepath.Glob(filepath.Join(tmpDir, "app-error-*.log"))
	if len(files) == 0 {
		t.Error("expected error log file to be created")
	}
}

func TestSplitOutput_Write_DebugLevel(t *testing.T) {
	tmpDir := t.TempDir()

	so, err := NewSplitOutput(tmpDir, "app")
	if err != nil {
		t.Fatalf("NewSplitOutput failed: %v", err)
	}
	defer func() { _ = so.Close() }()

	// Debug level should write to info file
	testData := []byte("DEBUG test log message\n")
	n, err := so.Write(zapcore.DebugLevel, testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
	}
}

func TestSplitOutput_Write_FatalLevel(t *testing.T) {
	tmpDir := t.TempDir()

	so, err := NewSplitOutput(tmpDir, "app")
	if err != nil {
		t.Fatalf("NewSplitOutput failed: %v", err)
	}
	defer func() { _ = so.Close() }()

	// Fatal level should write to error file
	testData := []byte("FATAL test log message\n")
	n, err := so.Write(zapcore.FatalLevel, testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
	}

	// Verify written to error file
	files, _ := filepath.Glob(filepath.Join(tmpDir, "app-error-*.log"))
	if len(files) == 0 {
		t.Error("expected error log file to be created for Fatal level")
	}
}

func TestSplitOutput_Close(t *testing.T) {
	tmpDir := t.TempDir()

	so, err := NewSplitOutput(tmpDir, "app")
	if err != nil {
		t.Fatalf("NewSplitOutput failed: %v", err)
	}

	err = so.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestSplitOutputCloseStopsRotation(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Date(2026, time.July, 29, 10, 0, 0, 0, time.Local)
	clock := newManualRotationClock(now)
	var openerMu sync.Mutex
	openerCalls := 0
	opener := func(directory, filePrefix, date string) (splitFileSet, error) {
		openerMu.Lock()
		openerCalls++
		openerMu.Unlock()
		return openSplitFiles(directory, filePrefix, date)
	}

	so, err := newSplitOutput(tmpDir, "app", clock, opener)
	if err != nil {
		t.Fatalf("create split output: %v", err)
	}
	timer := clock.nextTimer(t)

	if err := so.Close(); err != nil {
		t.Fatalf("close split output: %v", err)
	}
	if !timer.isStopped() {
		t.Error("Close must stop the pending rotation timer")
	}

	if err := so.openFiles(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("rotation after close must return os.ErrClosed, got: %v", err)
	}

	clock.setNow(now.AddDate(0, 0, 1))
	timer.fire(clock.Now())
	openerMu.Lock()
	defer openerMu.Unlock()
	if openerCalls != 1 {
		t.Fatalf("must not reopen files after Close, opener calls = %d", openerCalls)
	}
}

func TestSplitOutputCloseIdempotent(t *testing.T) {
	infoOut := &recordingWriteSyncCloser{}
	warnOut := &recordingWriteSyncCloser{}
	errorOut := &recordingWriteSyncCloser{}
	so := &SplitOutput{
		infoOut:  infoOut,
		warnOut:  warnOut,
		errorOut: errorOut,
	}

	const closeWorkers = 8
	var wg sync.WaitGroup
	wg.Add(closeWorkers)
	for i := 0; i < closeWorkers; i++ {
		go func() {
			defer wg.Done()
			if err := so.Close(); err != nil {
				t.Errorf("close split output concurrently: %v", err)
			}
		}()
	}
	wg.Wait()

	for name, output := range map[string]*recordingWriteSyncCloser{
		"info":  infoOut,
		"warn":  warnOut,
		"error": errorOut,
	} {
		_, closeCalls := output.counts()
		if closeCalls != 1 {
			t.Errorf("%s output must close once, got %d calls", name, closeCalls)
		}
	}
}

func TestSplitOutputCloseReturnsAllErrors(t *testing.T) {
	infoErr := errors.New("info close failed")
	warnErr := errors.New("warn close failed")
	errorErr := errors.New("error close failed")
	so := &SplitOutput{
		infoOut:  &recordingWriteSyncCloser{closeErr: infoErr},
		warnOut:  &recordingWriteSyncCloser{closeErr: warnErr},
		errorOut: &recordingWriteSyncCloser{closeErr: errorErr},
	}

	err := so.Close()
	for _, expected := range []error{infoErr, warnErr, errorErr} {
		if !errors.Is(err, expected) {
			t.Errorf("Close must contain error %v, got: %v", expected, err)
		}
	}
}

func TestSplitOutputAfterClose(t *testing.T) {
	so, err := NewSplitOutput(t.TempDir(), "app")
	if err != nil {
		t.Fatalf("create split output: %v", err)
	}
	if err := so.Close(); err != nil {
		t.Fatalf("close split output: %v", err)
	}

	if _, err := so.Write(zapcore.InfoLevel, []byte("must not write after close\n")); !errors.Is(err, os.ErrClosed) {
		t.Errorf("write after close must return os.ErrClosed, got: %v", err)
	}
	if err := so.Sync(); !errors.Is(err, os.ErrClosed) {
		t.Errorf("sync after close must return os.ErrClosed, got: %v", err)
	}
}

func TestSplitOutputRotationFailureKeepsCurrentFiles(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Date(2026, time.July, 29, 10, 0, 0, 0, time.Local)
	clock := newManualRotationClock(now)
	rotationAttempted := make(chan struct{})
	var openerMu sync.Mutex
	openerCalls := 0
	opener := func(directory, filePrefix, date string) (splitFileSet, error) {
		openerMu.Lock()
		openerCalls++
		call := openerCalls
		openerMu.Unlock()
		if call == 2 {
			close(rotationAttempted)
			return splitFileSet{}, errors.New("test rotation failed")
		}
		return openSplitFiles(directory, filePrefix, date)
	}

	so, err := newSplitOutput(tmpDir, "app", clock, opener)
	if err != nil {
		t.Fatalf("create split output: %v", err)
	}
	t.Cleanup(func() { _ = so.Close() })

	timer := clock.nextTimer(t)
	clock.setNow(now.AddDate(0, 0, 1))
	timer.fire(clock.Now())
	select {
	case <-rotationAttempted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for failed rotation")
	}

	message := []byte("still writable after failed rotation\n")
	if _, err := so.Write(zapcore.InfoLevel, message); err != nil {
		t.Fatalf("write old file after failed rotation: %v", err)
	}
	if err := so.Sync(); err != nil {
		t.Fatalf("sync existing files: %v", err)
	}

	infoPath := filepath.Join(tmpDir, "app-info-"+now.Format("2006-01-02")+".log")
	//nolint:gosec // infoPath combines t.TempDir with a fixed split filename.
	data, err := os.ReadFile(infoPath)
	if err != nil {
		t.Fatalf("read existing info log file: %v", err)
	}
	if !strings.Contains(string(data), strings.TrimSpace(string(message))) {
		t.Errorf("entry after failed rotation was not written to existing info log")
	}
}

func TestSplitOutputRotationSwitchesFiles(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Date(2026, time.July, 29, 10, 0, 0, 0, time.Local)
	clock := newManualRotationClock(now)
	so, err := newSplitOutput(tmpDir, "app", clock, openSplitFiles)
	if err != nil {
		t.Fatalf("create split output: %v", err)
	}
	t.Cleanup(func() { _ = so.Close() })

	firstTimer := clock.nextTimer(t)
	nextDay := now.AddDate(0, 0, 1)
	clock.setNow(nextDay)
	firstTimer.fire(nextDay)
	clock.nextTimer(t)

	message := []byte("write new file after successful rotation\n")
	if _, err := so.Write(zapcore.InfoLevel, message); err != nil {
		t.Fatalf("write after successful rotation: %v", err)
	}
	if err := so.Sync(); err != nil {
		t.Fatalf("sync after successful rotation: %v", err)
	}

	newInfoPath := filepath.Join(tmpDir, "app-info-"+nextDay.Format("2006-01-02")+".log")
	//nolint:gosec // newInfoPath combines t.TempDir with a fixed split filename.
	data, err := os.ReadFile(newInfoPath)
	if err != nil {
		t.Fatalf("read rotated info log file: %v", err)
	}
	if !strings.Contains(string(data), strings.TrimSpace(string(message))) {
		t.Error("entry after successful rotation was not written to the new date's info log")
	}
}

func TestSplitOutputWithOptionsPreservesPermissionsAcrossRotation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not provide equivalent POSIX mode semantics")
	}

	parent := t.TempDir()
	referenceDir := filepath.Join(parent, "reference")
	if err := os.Mkdir(referenceDir, 0o750); err != nil {
		t.Fatalf("create reference directory: %v", err)
	}
	referenceFile := filepath.Join(referenceDir, "reference.log")
	//nolint:gosec // Deliberately create a 0640 reference file to verify group-read permissions after rotation.
	if err := os.WriteFile(referenceFile, nil, 0o640); err != nil {
		t.Fatalf("create reference file: %v", err)
	}

	settings, err := resolveFileOutputOptions(WithDirPerm(0o750), WithFilePerm(0o640))
	if err != nil {
		t.Fatalf("resolve custom permissions: %v", err)
	}
	now := time.Date(2026, time.July, 29, 10, 0, 0, 0, time.Local)
	clock := newManualRotationClock(now)
	base := filepath.Join(parent, "logs")
	output, err := newSplitOutputWithSettings(base, "app", clock, openSplitFilesWithPermissions, settings)
	if err != nil {
		t.Fatalf("create SplitOutput with custom permissions: %v", err)
	}
	t.Cleanup(func() {
		if err := output.Close(); err != nil {
			t.Errorf("close SplitOutput: %v", err)
		}
	})

	assertSamePermission(t, base, referenceDir)
	assertSplitFilePermissions(t, base, "app", now, referenceFile)

	firstTimer := clock.nextTimer(t)
	nextDay := now.AddDate(0, 0, 1)
	clock.setNow(nextDay)
	firstTimer.fire(nextDay)
	clock.nextTimer(t)

	assertSplitFilePermissions(t, base, "app", nextDay, referenceFile)
}

func TestNewSplitOutputWithOptionsUsesConfiguredPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not provide equivalent POSIX mode semantics")
	}

	parent := t.TempDir()
	referenceDir, referenceFile := createPermissionReference(t, parent)
	base := filepath.Join(parent, "logs")
	output, err := NewSplitOutputWithOptions(
		base,
		"app",
		WithDirPerm(0o750),
		WithFilePerm(0o640),
	)
	if err != nil {
		t.Fatalf("create SplitOutput with custom permissions: %v", err)
	}
	if err := output.Close(); err != nil {
		t.Fatalf("close SplitOutput: %v", err)
	}

	assertSamePermission(t, base, referenceDir)
	assertCreatedSplitFilePermissions(t, base, "app", referenceFile)
}

func TestGetSplitCoreWithOptionsUsesConfiguredPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not provide equivalent POSIX mode semantics")
	}

	parent := t.TempDir()
	referenceDir, referenceFile := createPermissionReference(t, parent)
	base := filepath.Join(parent, "logs")
	encoderConfig := zapcore.EncoderConfig{
		MessageKey: "msg",
		EncodeTime: zapcore.ISO8601TimeEncoder,
	}
	core, cleanup, err := GetSplitCoreWithOptions(
		base,
		"app",
		encoderConfig,
		WithDirPerm(0o750),
		WithFilePerm(0o640),
	)
	if err != nil {
		t.Fatalf("create split core with custom permissions: %v", err)
	}
	if core == nil || cleanup == nil {
		t.Fatal("success must return a core and cleanup")
	}
	cleanup()

	assertSamePermission(t, base, referenceDir)
	assertCreatedSplitFilePermissions(t, base, "app", referenceFile)
}

func assertCreatedSplitFilePermissions(
	t *testing.T,
	base string,
	prefix string,
	referenceFile string,
) {
	t.Helper()
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatalf("read split output directory: %v", err)
	}
	matched := 0
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix+"-") || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		matched++
		assertSamePermission(t, filepath.Join(base, entry.Name()), referenceFile)
	}
	if matched != 3 {
		t.Fatalf("split file count matching prefix = %d, want 3", matched)
	}
}

func assertSplitFilePermissions(
	t *testing.T,
	base string,
	prefix string,
	date time.Time,
	referenceFile string,
) {
	t.Helper()
	for _, level := range []string{"info", "warn", "error"} {
		path := filepath.Join(base, prefix+"-"+level+"-"+date.Format("2006-01-02")+".log")
		assertSamePermission(t, path, referenceFile)
	}
}

func TestSplitOutputWrapper_Write(t *testing.T) {
	tmpDir := t.TempDir()

	so, err := NewSplitOutput(tmpDir, "app")
	if err != nil {
		t.Fatalf("NewSplitOutput failed: %v", err)
	}
	defer func() { _ = so.Close() }()

	wrapper := &splitOutputWrapper{so: so, lvl: zapcore.InfoLevel}
	testData := []byte("wrapper test\n")
	n, err := wrapper.Write(testData)
	if err != nil {
		t.Errorf("wrapper.Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
	}
}

func TestSplitOutputWrapper_Sync(t *testing.T) {
	tmpDir := t.TempDir()

	so, err := NewSplitOutput(tmpDir, "app")
	if err != nil {
		t.Fatalf("NewSplitOutput failed: %v", err)
	}
	defer func() { _ = so.Close() }()

	wrapper := &splitOutputWrapper{so: so, lvl: zapcore.InfoLevel}
	err = wrapper.Sync()
	if err != nil {
		t.Errorf("wrapper.Sync failed: %v", err)
	}
}

func TestSplitOutputSync(t *testing.T) {
	infoOut := &recordingWriteSyncCloser{}
	warnOut := &recordingWriteSyncCloser{}
	errorOut := &recordingWriteSyncCloser{}
	so := &SplitOutput{
		infoOut:  infoOut,
		warnOut:  warnOut,
		errorOut: errorOut,
	}

	if err := so.Sync(); err != nil {
		t.Fatalf("sync all split outputs: %v", err)
	}

	infoSyncs, _ := infoOut.counts()
	warnSyncs, _ := warnOut.counts()
	errorSyncs, _ := errorOut.counts()
	if infoSyncs != 1 || warnSyncs != 1 || errorSyncs != 1 {
		t.Fatalf("Sync must flush each output once, calls info=%d warn=%d error=%d", infoSyncs, warnSyncs, errorSyncs)
	}

	wrapper := &splitOutputWrapper{so: so, lvl: zapcore.InfoLevel}
	if err := wrapper.Sync(); err != nil {
		t.Fatalf("sync info output: %v", err)
	}

	infoSyncs, _ = infoOut.counts()
	warnSyncs, _ = warnOut.counts()
	errorSyncs, _ = errorOut.counts()
	if infoSyncs != 2 || warnSyncs != 1 || errorSyncs != 1 {
		t.Fatalf("wrapper must sync only info output, calls info=%d warn=%d error=%d", infoSyncs, warnSyncs, errorSyncs)
	}
}

func TestSplitOutputSyncReturnsAllErrors(t *testing.T) {
	infoErr := errors.New("info sync failed")
	warnErr := errors.New("warn sync failed")
	errorErr := errors.New("error sync failed")
	so := &SplitOutput{
		infoOut:  &recordingWriteSyncCloser{syncErr: infoErr},
		warnOut:  &recordingWriteSyncCloser{syncErr: warnErr},
		errorOut: &recordingWriteSyncCloser{syncErr: errorErr},
	}

	err := so.Sync()
	for _, expected := range []error{infoErr, warnErr, errorErr} {
		if !errors.Is(err, expected) {
			t.Errorf("Sync must contain error %v, got: %v", expected, err)
		}
	}
}

func TestGetSplitCore(t *testing.T) {
	tmpDir := t.TempDir()

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "ts",
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}

	core, cleanup, err := GetSplitCore(tmpDir, "app", encoderConfig)
	if err != nil {
		t.Fatalf("GetSplitCore failed: %v", err)
	}
	defer cleanup()

	if core == nil {
		t.Error("expected non-nil core")
	}

	// Verify files created
	files, _ := os.ReadDir(tmpDir)
	if len(files) < 3 {
		t.Errorf("expected at least 3 log files, got %d", len(files))
	}

	// Check file names
	foundInfo, foundWarn, foundError := false, false, false
	for _, f := range files {
		if strings.Contains(f.Name(), "-info-") {
			foundInfo = true
		}
		if strings.Contains(f.Name(), "-warn-") {
			foundWarn = true
		}
		if strings.Contains(f.Name(), "-error-") {
			foundError = true
		}
	}

	if !foundInfo {
		t.Error("expected info log file")
	}
	if !foundWarn {
		t.Error("expected warn log file")
	}
	if !foundError {
		t.Error("expected error log file")
	}
}

func TestGetSplitCoreRoutesLevels(t *testing.T) {
	tmpDir := t.TempDir()
	encoderConfig := zapcore.EncoderConfig{
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}

	core, cleanup, err := GetSplitCore(tmpDir, "app", encoderConfig)
	if err != nil {
		t.Fatalf("create split core: %v", err)
	}

	entries := []struct {
		level   zapcore.Level
		message string
	}{
		{level: zapcore.DebugLevel, message: "debug-message"},
		{level: zapcore.InfoLevel, message: "info-message"},
		{level: zapcore.WarnLevel, message: "warn-message"},
		{level: zapcore.ErrorLevel, message: "error-message"},
		{level: zapcore.DPanicLevel, message: "dpanic-message"},
		{level: zapcore.PanicLevel, message: "panic-message"},
		{level: zapcore.FatalLevel, message: "fatal-message"},
	}

	for _, entry := range entries {
		checked := core.Check(zapcore.Entry{Level: entry.level, Message: entry.message}, nil)
		if checked == nil {
			t.Errorf("enabled level %s did not produce a CheckedEntry", entry.level)
			continue
		}
		checked.Write()
	}
	if err := core.Sync(); err != nil {
		t.Errorf("sync split core: %v", err)
	}
	cleanup()

	contents := make(map[string]string, 3)
	for _, levelName := range []string{"info", "warn", "error"} {
		matches, err := filepath.Glob(filepath.Join(tmpDir, "app-"+levelName+"-*.log"))
		if err != nil {
			t.Fatalf("find %s log file: %v", levelName, err)
		}
		if len(matches) != 1 {
			t.Fatalf("expected one %s log file, got %d", levelName, len(matches))
		}
		data, err := os.ReadFile(matches[0])
		if err != nil {
			t.Fatalf("read %s log file: %v", levelName, err)
		}
		contents[levelName] = string(data)
	}

	expectedFile := map[string]string{
		"debug-message":  "info",
		"info-message":   "info",
		"warn-message":   "warn",
		"error-message":  "error",
		"dpanic-message": "error",
		"panic-message":  "error",
		"fatal-message":  "error",
	}
	for message, target := range expectedFile {
		for levelName, content := range contents {
			contains := strings.Contains(content, message)
			if levelName == target && !contains {
				t.Errorf("%s must be present in %s log file", message, target)
			}
			if levelName != target && contains {
				t.Errorf("%s must not be present in %s log file", message, levelName)
			}
		}
	}
}

func TestNewSplitOutputInvalidDirectory(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "parent-file")
	if err := os.WriteFile(parentFile, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	_, err := NewSplitOutput(filepath.Join(parentFile, "child"), "app")
	if err == nil {
		t.Fatal("construction must fail when the parent path is a regular file")
	}
}

func TestGetSplitCoreInvalidDirectory(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "parent-file")
	if err := os.WriteFile(parentFile, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	_, cleanup, err := GetSplitCore(filepath.Join(parentFile, "child"), "app", zap.NewProductionEncoderConfig())
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatal("core construction must fail when the parent path is a regular file")
	}
}
