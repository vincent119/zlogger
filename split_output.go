// Package zlogger 提供依日誌級別分檔的輸出功能。
//
// DEBUG 與 INFO 寫入 info 檔，WARN 寫入 warn 檔，ERROR 以上寫入 error 檔。
// 每日換檔 worker 會在 Close 回傳前停止，避免關閉後重新開啟檔案。
package zlogger

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// SplitOutput 將不同日誌級別寫入不同檔案。
//
// 級別對應如下：
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
	opener    splitFileOpener
}

// NewSplitOutput 建立分級日誌輸出，並啟動每日換檔 worker。
func NewSplitOutput(directory, filePrefix string) (*SplitOutput, error) {
	return newSplitOutput(directory, filePrefix, systemRotationClock{}, openSplitFiles)
}

func newSplitOutput(
	directory string,
	filePrefix string,
	clock rotationClock,
	opener splitFileOpener,
) (*SplitOutput, error) {
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, fmt.Errorf("建立日誌目錄失敗：%w", err)
	}

	output := &SplitOutput{
		directory:  directory,
		filePrefix: filePrefix,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
		clock:      clock,
		opener:     opener,
	}
	if err := output.openFiles(); err != nil {
		return nil, err
	}

	go output.rotateDaily()
	return output, nil
}

func openSplitFiles(directory, filePrefix, date string) (splitFileSet, error) {
	files := splitFileSet{}

	infoFile, err := os.OpenFile(
		filepath.Join(directory, filePrefix+"-info-"+date+".log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return splitFileSet{}, fmt.Errorf("開啟 info 日誌檔失敗：%w", err)
	}
	files.info = infoFile

	warnFile, err := os.OpenFile(
		filepath.Join(directory, filePrefix+"-warn-"+date+".log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return splitFileSet{}, errors.Join(
			fmt.Errorf("開啟 warn 日誌檔失敗：%w", err),
			files.close(),
		)
	}
	files.warn = warnFile

	errorFile, err := os.OpenFile(
		filepath.Join(directory, filePrefix+"-error-"+date+".log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return splitFileSet{}, errors.Join(
			fmt.Errorf("開啟 error 日誌檔失敗：%w", err),
			files.close(),
		)
	}
	files.error = errorFile

	return files, nil
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
		return fmt.Errorf("分級輸出已關閉：%w", os.ErrClosed)
	}
	clock := s.clock
	opener := s.opener
	s.mutex.Unlock()

	if clock == nil || opener == nil {
		return fmt.Errorf("分級輸出尚未初始化：%w", os.ErrInvalid)
	}

	date := clock.Now().Format("2006-01-02")
	newFiles, err := opener(s.directory, s.filePrefix, date)
	if err != nil {
		return err
	}

	s.mutex.Lock()
	if s.closed {
		s.mutex.Unlock()
		return errors.Join(
			fmt.Errorf("分級輸出已關閉：%w", os.ErrClosed),
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
				fmt.Fprintf(os.Stderr, "每日換檔失敗：%v\n", err)
			}
		}
	}
}

// Write 依日誌級別寫入對應檔案。
func (s *SplitOutput) Write(level zapcore.Level, data []byte) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.closed {
		return 0, fmt.Errorf("分級輸出已關閉：%w", os.ErrClosed)
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
		return 0, fmt.Errorf("分級輸出尚未初始化：%w", os.ErrInvalid)
	}
	return output.Write(data)
}

// Sync 將所有分級日誌檔同步至儲存裝置。
func (s *SplitOutput) Sync() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.closed {
		return fmt.Errorf("分級輸出已關閉：%w", os.ErrClosed)
	}
	return s.currentFiles().sync()
}

func (s *SplitOutput) syncLevel(level zapcore.Level) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.closed {
		return fmt.Errorf("分級輸出已關閉：%w", os.ErrClosed)
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
		return fmt.Errorf("分級輸出尚未初始化：%w", os.ErrInvalid)
	}
	return output.Sync()
}

// Close 停止每日換檔 worker，並關閉所有分級日誌檔。
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

// GetSplitCore 建立依級別分檔的 zap core。
//
// cleanup 會同步停止換檔 worker 並關閉所有檔案。呼叫端應在 logger 不再使用後執行 cleanup。
func GetSplitCore(
	directory string,
	filePrefix string,
	encoderConfig zapcore.EncoderConfig,
) (zapcore.Core, func(), error) {
	splitOut, err := NewSplitOutput(directory, filePrefix)
	if err != nil {
		return nil, nil, err
	}

	encoder := zapcore.NewJSONEncoder(encoderConfig)
	infoOut := zapcore.AddSync(&splitOutputWrapper{so: splitOut, lvl: zapcore.InfoLevel})
	warnOut := zapcore.AddSync(&splitOutputWrapper{so: splitOut, lvl: zapcore.WarnLevel})
	errorOut := zapcore.AddSync(&splitOutputWrapper{so: splitOut, lvl: zapcore.ErrorLevel})

	infoLevel := zap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level == zapcore.DebugLevel || level == zapcore.InfoLevel
	})
	warnLevel := zap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level == zapcore.WarnLevel
	})
	errorLevel := zap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level >= zapcore.ErrorLevel
	})

	core := zapcore.NewTee(
		zapcore.NewCore(encoder, infoOut, infoLevel),
		zapcore.NewCore(encoder, warnOut, warnLevel),
		zapcore.NewCore(encoder, errorOut, errorLevel),
	)
	return core, func() { _ = splitOut.Close() }, nil
}
