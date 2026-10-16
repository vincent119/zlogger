package zlogger

import (
	"fmt"
	"sync/atomic"
	"time"

	"go.uber.org/zap/zapcore"
)

// OutputFailure describes one failed operation without retaining the log message or fields.
// Err preserves the original error for errors.Is/As, but its text may contain sensitive information.
type OutputFailure struct {
	Operation  string    `json:"operation" yaml:"operation" mapstructure:"operation"`
	Output     string    `json:"output" yaml:"output" mapstructure:"output"`
	OccurredAt time.Time `json:"occurred_at" yaml:"occurred_at" mapstructure:"occurred_at"`
	Err        error     `json:"-" yaml:"-" mapstructure:"-"`
}

// FailureObserver stores failures in a bounded channel. Do not copy it after use.
// The channel never closes. Consumers must stop using their own context. The zero value is query-only.
type FailureObserver struct {
	events  chan OutputFailure
	dropped atomic.Uint64
}

// NewFailureObserver creates an observer with capacity 1 to 1024 and starts no goroutines.
func NewFailureObserver(capacity int) (*FailureObserver, error) {
	if capacity < 1 || capacity > 1024 {
		return nil, fmt.Errorf("%w: observer capacity must be between 1 and 1024", ErrInvalidConfig)
	}
	return &FailureObserver{events: make(chan OutputFailure, capacity)}, nil
}

// Events returns a receive-only channel, or nil for a nil or zero-value observer.
func (o *FailureObserver) Events() <-chan OutputFailure {
	if o == nil {
		return nil
	}
	return o.events
}

// Dropped returns the total number of new events dropped because the channel was full.
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

// NewObservedWriteSyncer observes sink Write/Sync failures without changing core Check or hooks.
// The wrapper provides no Close or additional concurrency guarantees. Apply zapcore.Lock first if needed.
// Encoding errors are not observed. Each wrapped tee sink reports independently without deduplication.
func NewObservedWriteSyncer(sink zapcore.WriteSyncer, observer *FailureObserver) (zapcore.WriteSyncer, error) {
	if sink == nil || observer == nil || observer.events == nil {
		return nil, fmt.Errorf("%w: sink and initialized observer must not be nil", ErrInvalidConfig)
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
