package zlogger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestNewSplitOutput(t *testing.T) {
	// 建立臨時目錄
	tmpDir := t.TempDir()

	so, err := NewSplitOutput(tmpDir, "test")
	if err != nil {
		t.Fatalf("NewSplitOutput failed: %v", err)
	}
	defer so.Close()

	if so.directory != tmpDir {
		t.Errorf("expected directory %s, got %s", tmpDir, so.directory)
	}
	if so.filePrefix != "test" {
		t.Errorf("expected filePrefix 'test', got %s", so.filePrefix)
	}
}

func TestNewSplitOutput_InvalidDirectory(t *testing.T) {
	// 嘗試在無效路徑建立
	// 注意：這個測試依賴於作業系統權限
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
	defer so.Close()

	testData := []byte("INFO test log message\n")
	n, err := so.Write(zapcore.InfoLevel, testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
	}

	// 驗證檔案存在
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
	defer so.Close()

	testData := []byte("WARN test log message\n")
	n, err := so.Write(zapcore.WarnLevel, testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
	}

	// 驗證檔案存在
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
	defer so.Close()

	testData := []byte("ERROR test log message\n")
	n, err := so.Write(zapcore.ErrorLevel, testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
	}

	// 驗證檔案存在
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
	defer so.Close()

	// Debug 級別應該寫入 info 檔案
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
	defer so.Close()

	// Fatal 級別應該寫入 error 檔案
	testData := []byte("FATAL test log message\n")
	n, err := so.Write(zapcore.FatalLevel, testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
	}

	// 驗證寫入 error 檔案
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
	defer so.Close()

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
	defer so.Close()

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

	// 驗證檔案已建立
	files, _ := os.ReadDir(tmpDir)
	if len(files) < 3 {
		t.Errorf("expected at least 3 log files, got %d", len(files))
	}

	// 檢查檔案名稱
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
