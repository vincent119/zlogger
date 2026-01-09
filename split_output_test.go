package zlogger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

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

func TestNewSplitOutput_InvalidDirectory(t *testing.T) {
	// Try to create in invalid path
	// Note: this test depends on OS permissions
	invalidPath := "/nonexistent/deeply/nested/path/that/should/not/exist"

	_, err := NewSplitOutput(invalidPath, "test")
	if err == nil {
		t.Error("expected error for invalid directory, got nil")
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

func TestGetSplitCore_InvalidDirectory(t *testing.T) {
	invalidPath := "/nonexistent/deeply/nested/path"

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:    "ts",
		LevelKey:   "level",
		MessageKey: "msg",
	}

	_, cleanup, err := GetSplitCore(invalidPath, "app", encoderConfig)
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Error("expected error for invalid directory, got nil")
	}
}
