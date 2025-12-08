// Package zlogger - split_output.go
//
// 此檔案提供按日誌級別分離輸出的功能。
//
// 功能說明：
//   - 將 INFO、WARN、ERROR 級別的日誌分別寫入不同檔案
//   - 每天零點自動切換到新日期的檔案
//   - 線程安全，支援併發寫入
//
// 注意事項：
//   - 此功能是「按級別分離」，不是「log rotation」
//   - 如需大小限制、壓縮備份等功能，請搭配 timberjack 使用
//   - 詳細說明請參考 README.md 的「Log Rotation」章節
//
// 使用範例：
//
//	core, cleanup, err := zlogger.GetSplitCore("./logs", "app", encoderConfig)
//	if err != nil {
//	    panic(err)
//	}
//	defer cleanup()
//
// 輸出檔案：
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

// SplitOutput 將不同級別的日誌寫入不同的檔案
//
// 功能：
//   - INFO 級別 → {prefix}-info-{date}.log
//   - WARN 級別 → {prefix}-warn-{date}.log
//   - ERROR/DPANIC/PANIC/FATAL 級別 → {prefix}-error-{date}.log
//   - DEBUG 級別 → 歸類到 info 檔案
type SplitOutput struct {
	directory  string
	filePrefix string
	infoOut    io.Writer
	warnOut    io.Writer
	errorOut   io.Writer
	mutex      sync.Mutex
}

// NewSplitOutput 創建分離日誌輸出
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

	// 定期切換日誌檔案（每天零點）
	go so.rotateDaily()

	return so, nil
}

// openFiles 開啟各級別日誌檔案
func (s *SplitOutput) openFiles() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.closeFiles()

	date := time.Now().Format("2006-01-02")

	// 開啟 INFO 級別日誌檔案
	infoFile, err := os.OpenFile(
		filepath.Join(s.directory, s.filePrefix+"-info-"+date+".log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return err
	}
	s.infoOut = infoFile

	// 開啟 WARN 級別日誌檔案
	warnFile, err := os.OpenFile(
		filepath.Join(s.directory, s.filePrefix+"-warn-"+date+".log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		infoFile.Close()
		return err
	}
	s.warnOut = warnFile

	// 開啟 ERROR 級別日誌檔案
	errorFile, err := os.OpenFile(
		filepath.Join(s.directory, s.filePrefix+"-error-"+date+".log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		infoFile.Close()
		warnFile.Close()
		return err
	}
	s.errorOut = errorFile

	return nil
}

// closeFiles 關閉所有檔案
func (s *SplitOutput) closeFiles() {
	if closer, ok := s.infoOut.(io.Closer); ok && closer != nil {
		closer.Close()
	}
	if closer, ok := s.warnOut.(io.Closer); ok && closer != nil {
		closer.Close()
	}
	if closer, ok := s.errorOut.(io.Closer); ok && closer != nil {
		closer.Close()
	}
}

// rotateDaily 每天零點切換日誌檔案
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

// Write 實現按級別寫入日誌
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

// Close 關閉分離輸出
func (s *SplitOutput) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.closeFiles()
	return nil
}

// splitOutputWrapper 支持 zapcore.WriteSyncer 接口的包裝器
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

// GetSplitCore 創建按級別分離的日誌核心
//
// 參數：
//   - directory: 日誌檔案目錄
//   - filePrefix: 檔案名稱前綴
//   - encoderConfig: zap 編碼器配置
//
// 返回：
//   - zapcore.Core: 可用於 zap.New() 的核心
//   - func(): 清理函數，程式結束時調用以關閉檔案
//   - error: 錯誤信息
//
// 注意：此功能不包含 log rotation（大小限制/壓縮），如需請搭配 timberjack
func GetSplitCore(directory, filePrefix string, encoderConfig zapcore.EncoderConfig) (zapcore.Core, func(), error) {
	splitOut, err := NewSplitOutput(directory, filePrefix)
	if err != nil {
		return nil, nil, err
	}

	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// 創建各級別的 WriteSyncer
	infoOut := zapcore.AddSync(&splitOutputWrapper{so: splitOut, lvl: zapcore.InfoLevel})
	warnOut := zapcore.AddSync(&splitOutputWrapper{so: splitOut, lvl: zapcore.WarnLevel})
	errorOut := zapcore.AddSync(&splitOutputWrapper{so: splitOut, lvl: zapcore.ErrorLevel})

	// 設置級別過濾
	infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl == zapcore.InfoLevel
	})
	warnLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl == zapcore.WarnLevel
	})
	errorLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})

	// 創建三個核心
	infoCore := zapcore.NewCore(encoder, infoOut, infoLevel)
	warnCore := zapcore.NewCore(encoder, warnOut, warnLevel)
	errorCore := zapcore.NewCore(encoder, errorOut, errorLevel)

	// 組合所有核心
	core := zapcore.NewTee(infoCore, warnCore, errorCore)

	return core, func() { splitOut.Close() }, nil
}

