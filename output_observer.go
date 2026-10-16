package zlogger

import (
	"fmt"
	"sync/atomic"
	"time"

	"go.uber.org/zap/zapcore"
)

// OutputFailure 描述一次失敗操作，不保存日誌本文或 fields。
// Err 保留原錯誤供 errors.Is／As 判斷，但其文字仍可能含敏感資訊。
type OutputFailure struct {
	Operation  string    `json:"operation" yaml:"operation" mapstructure:"operation"`
	Output     string    `json:"output" yaml:"output" mapstructure:"output"`
	OccurredAt time.Time `json:"occurred_at" yaml:"occurred_at" mapstructure:"occurred_at"`
	Err        error     `json:"-" yaml:"-" mapstructure:"-"`
}

// FailureObserver 以有界通道保存故障；不可複製已使用的 observer。
// 通道不會關閉，消費者須以自己的 context 停止；零值只可查詢，不能登記。
type FailureObserver struct {
	events  chan OutputFailure
	dropped atomic.Uint64
}

// NewFailureObserver 建立容量為 1 至 1024 的 observer，不啟動背景 goroutine。
func NewFailureObserver(capacity int) (*FailureObserver, error) {
	if capacity < 1 || capacity > 1024 {
		return nil, fmt.Errorf("%w: observer 容量須為 1 至 1024", ErrInvalidConfig)
	}
	return &FailureObserver{events: make(chan OutputFailure, capacity)}, nil
}

// Events 回傳唯讀事件通道；nil 或零值回傳 nil。
func (o *FailureObserver) Events() <-chan OutputFailure {
	if o == nil {
		return nil
	}
	return o.events
}

// Dropped 回傳因滿載而丟棄的新事件總數。
func (o *FailureObserver) Dropped() uint64 {
	if o == nil {
		return 0
	}
	return o.dropped.Load()
}

func (o *FailureObserver) report(operation, output string, err error) {
	if o == nil || err == nil {
		return
	}
	event := OutputFailure{Operation: operation, Output: output, OccurredAt: time.Now().UTC(), Err: err}
	select {
	case o.events <- event:
	default:
		o.dropped.Add(1)
	}
}

// NewObservedWriteSyncer 觀察 sink 的 Write／Sync 錯誤，不介入 core 的 Check 或 hooks。
// wrapper 不提供 Close，也不增加 sink 的並行保證；必要時呼叫端先使用 zapcore.Lock。
// 不涵蓋編碼錯誤；tee 的每個已包裝 sink 各自回報，不做跨 wrapper 去重。
func NewObservedWriteSyncer(sink zapcore.WriteSyncer, observer *FailureObserver) (zapcore.WriteSyncer, error) {
	if sink == nil || observer == nil || observer.events == nil {
		return nil, fmt.Errorf("%w: sink 與已初始化的 observer 不可為 nil", ErrInvalidConfig)
	}
	return &observedWriteSyncer{sink: sink, observer: observer}, nil
}

type observedWriteSyncer struct {
	sink     zapcore.WriteSyncer
	observer *FailureObserver
}

func (s *observedWriteSyncer) Write(p []byte) (int, error) {
	n, err := s.sink.Write(p)
	s.observer.report("write", "sink", err)
	return n, err
}

func (s *observedWriteSyncer) Sync() error {
	err := s.sink.Sync()
	s.observer.report("sync", "sink", err)
	return err
}
