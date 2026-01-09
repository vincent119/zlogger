// Package zlogger - split_output.go
//
// This file provides functionality for splitting log output by level.
//
// Features:
//   - Writes INFO, WARN, ERROR level logs to separate files
//   - Automatically rotates to new date files at midnight
//   - Thread-safe, supports concurrent writes
//
// Notes:
//   - This is "split by level", not "log rotation"
//   - For size limits, compression, etc., use with timberjack
//   - See README.md "Log Rotation" section for details
//
// Example:
//
//	core, cleanup, err := zlogger.GetSplitCore("./logs", "app", encoderConfig)
//	if err != nil {
//	    panic(err)
//	}
//	defer cleanup()
//
// Output files:
//
//	logs/
//	├── app-info-2024-01-01.log
//	├── app-warn-2024-01-01.log
//	└── app-error-2024-01-01.log
package zlogger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// SplitOutput writes different log levels to different files
//
// Mapping:
//   - INFO level → {prefix}-info-{date}.log
//   - WARN level → {prefix}-warn-{date}.log
//   - ERROR/DPANIC/PANIC/FATAL level → {prefix}-error-{date}.log
//   - DEBUG level → goes to info file
type SplitOutput struct {
	directory  string
	filePrefix string
	infoOut    io.Writer
	warnOut    io.Writer
	errorOut   io.Writer
	mutex      sync.Mutex
}

// NewSplitOutput creates a split log output
func NewSplitOutput(directory, filePrefix string) (*SplitOutput, error) {
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}

	so := &SplitOutput{
		directory:  directory,
		filePrefix: filePrefix,
	}

	if err := so.openFiles(); err != nil {
		return nil, err
	}

	// Schedule daily log file rotation (at midnight)
	go so.rotateDaily()

	return so, nil
}

// openFiles opens log files for each level
func (s *SplitOutput) openFiles() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.closeFiles()

	date := time.Now().Format("2006-01-02")

	// Open INFO level log file
	infoFile, err := os.OpenFile(
		filepath.Join(s.directory, s.filePrefix+"-info-"+date+".log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return err
	}
	s.infoOut = infoFile

	// Open WARN level log file
	warnFile, err := os.OpenFile(
		filepath.Join(s.directory, s.filePrefix+"-warn-"+date+".log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		_ = infoFile.Close()
		return err
	}
	s.warnOut = warnFile

	// Open ERROR level log file
	errorFile, err := os.OpenFile(
		filepath.Join(s.directory, s.filePrefix+"-error-"+date+".log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		_ = infoFile.Close()
		_ = warnFile.Close()
		return err
	}
	s.errorOut = errorFile

	return nil
}

// closeFiles closes all files
func (s *SplitOutput) closeFiles() {
	if closer, ok := s.infoOut.(io.Closer); ok && closer != nil {
		_ = closer.Close()
	}
	if closer, ok := s.warnOut.(io.Closer); ok && closer != nil {
		_ = closer.Close()
	}
	if closer, ok := s.errorOut.(io.Closer); ok && closer != nil {
		_ = closer.Close()
	}
}

// rotateDaily rotates log files at midnight
func (s *SplitOutput) rotateDaily() {
	for {
		now := time.Now()
		next := now.Add(24 * time.Hour)
		next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
		duration := next.Sub(now)

		time.Sleep(duration)

		if err := s.openFiles(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to rotate log files: %v\n", err)
		}
	}
}

// Write implements level-based log writing
func (s *SplitOutput) Write(lvl zapcore.Level, p []byte) (n int, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	switch lvl {
	case zapcore.InfoLevel:
		return s.infoOut.Write(p)
	case zapcore.WarnLevel:
		return s.warnOut.Write(p)
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		return s.errorOut.Write(p)
	default:
		return s.infoOut.Write(p)
	}
}

// Close closes the split output
func (s *SplitOutput) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.closeFiles()
	return nil
}

// splitOutputWrapper wraps SplitOutput to support zapcore.WriteSyncer interface
type splitOutputWrapper struct {
	so  *SplitOutput
	lvl zapcore.Level
}

func (w *splitOutputWrapper) Write(p []byte) (n int, err error) {
	return w.so.Write(w.lvl, p)
}

func (w *splitOutputWrapper) Sync() error {
	return nil
}

// GetSplitCore creates a level-separated log core
//
// Parameters:
//   - directory: log file directory
//   - filePrefix: file name prefix
//   - encoderConfig: zap encoder configuration
//
// Returns:
//   - zapcore.Core: core usable with zap.New()
//   - func(): cleanup function to close files when program ends
//   - error: error information
//
// Note: This does not include log rotation (size limits/compression), use timberjack if needed
func GetSplitCore(directory, filePrefix string, encoderConfig zapcore.EncoderConfig) (zapcore.Core, func(), error) {
	splitOut, err := NewSplitOutput(directory, filePrefix)
	if err != nil {
		return nil, nil, err
	}

	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// Create WriteSyncer for each level
	infoOut := zapcore.AddSync(&splitOutputWrapper{so: splitOut, lvl: zapcore.InfoLevel})
	warnOut := zapcore.AddSync(&splitOutputWrapper{so: splitOut, lvl: zapcore.WarnLevel})
	errorOut := zapcore.AddSync(&splitOutputWrapper{so: splitOut, lvl: zapcore.ErrorLevel})

	// Set level filters
	infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl == zapcore.InfoLevel
	})
	warnLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl == zapcore.WarnLevel
	})
	errorLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})

	// Create three cores
	infoCore := zapcore.NewCore(encoder, infoOut, infoLevel)
	warnCore := zapcore.NewCore(encoder, warnOut, warnLevel)
	errorCore := zapcore.NewCore(encoder, errorOut, errorLevel)

	// Combine all cores
	core := zapcore.NewTee(infoCore, warnCore, errorCore)

	return core, func() { _ = splitOut.Close() }, nil
}
