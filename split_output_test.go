package zlogger

import (
	"errors"
	"os"
	"path/filepath"
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
		t.Fatal("等待換檔 timer 逾時")
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
		t.Fatalf("建立分級輸出失敗：%v", err)
	}
	timer := clock.nextTimer(t)

	if err := so.Close(); err != nil {
		t.Fatalf("關閉分級輸出失敗：%v", err)
	}
	if !timer.isStopped() {
		t.Error("Close 應停止尚未觸發的換檔 timer")
	}

	if err := so.openFiles(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("關閉後換檔應回傳 os.ErrClosed，實際為：%v", err)
	}

	clock.setNow(now.AddDate(0, 0, 1))
	timer.fire(clock.Now())
	openerMu.Lock()
	defer openerMu.Unlock()
	if openerCalls != 1 {
		t.Fatalf("Close 後不得再次開檔，opener 實際呼叫 %d 次", openerCalls)
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
				t.Errorf("並行關閉分級輸出失敗：%v", err)
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
			t.Errorf("%s 輸出應只關閉一次，實際關閉 %d 次", name, closeCalls)
		}
	}
}

func TestSplitOutputCloseReturnsAllErrors(t *testing.T) {
	infoErr := errors.New("info 關閉失敗")
	warnErr := errors.New("warn 關閉失敗")
	errorErr := errors.New("error 關閉失敗")
	so := &SplitOutput{
		infoOut:  &recordingWriteSyncCloser{closeErr: infoErr},
		warnOut:  &recordingWriteSyncCloser{closeErr: warnErr},
		errorOut: &recordingWriteSyncCloser{closeErr: errorErr},
	}

	err := so.Close()
	for _, expected := range []error{infoErr, warnErr, errorErr} {
		if !errors.Is(err, expected) {
			t.Errorf("Close 應包含錯誤 %v，實際為：%v", expected, err)
		}
	}
}

func TestSplitOutputAfterClose(t *testing.T) {
	so, err := NewSplitOutput(t.TempDir(), "app")
	if err != nil {
		t.Fatalf("建立分級輸出失敗：%v", err)
	}
	if err := so.Close(); err != nil {
		t.Fatalf("關閉分級輸出失敗：%v", err)
	}

	if _, err := so.Write(zapcore.InfoLevel, []byte("關閉後不得寫入\n")); !errors.Is(err, os.ErrClosed) {
		t.Errorf("關閉後寫入應回傳 os.ErrClosed，實際為：%v", err)
	}
	if err := so.Sync(); !errors.Is(err, os.ErrClosed) {
		t.Errorf("關閉後同步應回傳 os.ErrClosed，實際為：%v", err)
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
			return splitFileSet{}, errors.New("測試換檔失敗")
		}
		return openSplitFiles(directory, filePrefix, date)
	}

	so, err := newSplitOutput(tmpDir, "app", clock, opener)
	if err != nil {
		t.Fatalf("建立分級輸出失敗：%v", err)
	}
	t.Cleanup(func() { _ = so.Close() })

	timer := clock.nextTimer(t)
	clock.setNow(now.AddDate(0, 0, 1))
	timer.fire(clock.Now())
	select {
	case <-rotationAttempted:
	case <-time.After(time.Second):
		t.Fatal("等待換檔失敗情境逾時")
	}

	message := []byte("換檔失敗後仍可寫入\n")
	if _, err := so.Write(zapcore.InfoLevel, message); err != nil {
		t.Fatalf("換檔失敗後寫入舊檔失敗：%v", err)
	}
	if err := so.Sync(); err != nil {
		t.Fatalf("同步既有檔案失敗：%v", err)
	}

	infoPath := filepath.Join(tmpDir, "app-info-"+now.Format("2006-01-02")+".log")
	data, err := os.ReadFile(infoPath)
	if err != nil {
		t.Fatalf("讀取既有 info 日誌檔失敗：%v", err)
	}
	if !strings.Contains(string(data), strings.TrimSpace(string(message))) {
		t.Errorf("換檔失敗後的訊息未寫入既有 info 日誌檔")
	}
}

func TestSplitOutputRotationSwitchesFiles(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Date(2026, time.July, 29, 10, 0, 0, 0, time.Local)
	clock := newManualRotationClock(now)
	so, err := newSplitOutput(tmpDir, "app", clock, openSplitFiles)
	if err != nil {
		t.Fatalf("建立分級輸出失敗：%v", err)
	}
	t.Cleanup(func() { _ = so.Close() })

	firstTimer := clock.nextTimer(t)
	nextDay := now.AddDate(0, 0, 1)
	clock.setNow(nextDay)
	firstTimer.fire(nextDay)
	clock.nextTimer(t)

	message := []byte("換檔成功後寫入新檔\n")
	if _, err := so.Write(zapcore.InfoLevel, message); err != nil {
		t.Fatalf("換檔成功後寫入失敗：%v", err)
	}
	if err := so.Sync(); err != nil {
		t.Fatalf("換檔成功後同步失敗：%v", err)
	}

	newInfoPath := filepath.Join(tmpDir, "app-info-"+nextDay.Format("2006-01-02")+".log")
	data, err := os.ReadFile(newInfoPath)
	if err != nil {
		t.Fatalf("讀取換檔後 info 日誌檔失敗：%v", err)
	}
	if !strings.Contains(string(data), strings.TrimSpace(string(message))) {
		t.Error("換檔成功後的訊息未寫入新日期 info 日誌檔")
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
		t.Fatalf("同步全部分級輸出失敗：%v", err)
	}

	infoSyncs, _ := infoOut.counts()
	warnSyncs, _ := warnOut.counts()
	errorSyncs, _ := errorOut.counts()
	if infoSyncs != 1 || warnSyncs != 1 || errorSyncs != 1 {
		t.Fatalf("Sync 應各同步一次，實際次數 info=%d warn=%d error=%d", infoSyncs, warnSyncs, errorSyncs)
	}

	wrapper := &splitOutputWrapper{so: so, lvl: zapcore.InfoLevel}
	if err := wrapper.Sync(); err != nil {
		t.Fatalf("同步 info 輸出失敗：%v", err)
	}

	infoSyncs, _ = infoOut.counts()
	warnSyncs, _ = warnOut.counts()
	errorSyncs, _ = errorOut.counts()
	if infoSyncs != 2 || warnSyncs != 1 || errorSyncs != 1 {
		t.Fatalf("wrapper 應只同步 info 輸出，實際次數 info=%d warn=%d error=%d", infoSyncs, warnSyncs, errorSyncs)
	}
}

func TestSplitOutputSyncReturnsAllErrors(t *testing.T) {
	infoErr := errors.New("info 同步失敗")
	warnErr := errors.New("warn 同步失敗")
	errorErr := errors.New("error 同步失敗")
	so := &SplitOutput{
		infoOut:  &recordingWriteSyncCloser{syncErr: infoErr},
		warnOut:  &recordingWriteSyncCloser{syncErr: warnErr},
		errorOut: &recordingWriteSyncCloser{syncErr: errorErr},
	}

	err := so.Sync()
	for _, expected := range []error{infoErr, warnErr, errorErr} {
		if !errors.Is(err, expected) {
			t.Errorf("Sync 應包含錯誤 %v，實際為：%v", expected, err)
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
		t.Fatalf("建立分級 core 失敗：%v", err)
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
			t.Errorf("級別 %s 應啟用但未取得 CheckedEntry", entry.level)
			continue
		}
		checked.Write()
	}
	if err := core.Sync(); err != nil {
		t.Errorf("同步分級 core 失敗：%v", err)
	}
	cleanup()

	contents := make(map[string]string, 3)
	for _, levelName := range []string{"info", "warn", "error"} {
		matches, err := filepath.Glob(filepath.Join(tmpDir, "app-"+levelName+"-*.log"))
		if err != nil {
			t.Fatalf("搜尋 %s 日誌檔失敗：%v", levelName, err)
		}
		if len(matches) != 1 {
			t.Fatalf("預期一個 %s 日誌檔，實際為 %d 個", levelName, len(matches))
		}
		data, err := os.ReadFile(matches[0])
		if err != nil {
			t.Fatalf("讀取 %s 日誌檔失敗：%v", levelName, err)
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
				t.Errorf("%s 應存在於 %s 日誌檔", message, target)
			}
			if levelName != target && contains {
				t.Errorf("%s 不應存在於 %s 日誌檔", message, levelName)
			}
		}
	}
}

func TestNewSplitOutputInvalidDirectory(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "parent-file")
	if err := os.WriteFile(parentFile, []byte("不是目錄"), 0600); err != nil {
		t.Fatalf("建立測試檔案失敗：%v", err)
	}

	_, err := NewSplitOutput(filepath.Join(parentFile, "child"), "app")
	if err == nil {
		t.Fatal("父路徑為一般檔案時應建立失敗")
	}
}

func TestGetSplitCoreInvalidDirectory(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "parent-file")
	if err := os.WriteFile(parentFile, []byte("不是目錄"), 0600); err != nil {
		t.Fatalf("建立測試檔案失敗：%v", err)
	}

	_, cleanup, err := GetSplitCore(filepath.Join(parentFile, "child"), "app", zap.NewProductionEncoderConfig())
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatal("父路徑為一般檔案時應建立 core 失敗")
	}
}
