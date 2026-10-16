package zlogger

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type failingObserverSink struct {
	writeErr error
	syncErr  error
}

func (s failingObserverSink) Write(p []byte) (int, error) {
	if s.writeErr != nil {
		return 0, s.writeErr
	}
	return len(p), nil
}

func (s failingObserverSink) Sync() error { return s.syncErr }

func (s failingObserverSink) Close() error { return nil }

func newTestObserver(t *testing.T, capacity int) *FailureObserver {
	t.Helper()
	o, err := NewFailureObserver(capacity)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestObservedWriteSyncerPreservesErrors(t *testing.T) {
	writeErr, syncErr := errors.New("write failed"), errors.New("sync failed")
	o := newTestObserver(t, 2)
	sink, err := NewObservedWriteSyncer(failingObserverSink{writeErr, syncErr}, o)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := sink.Write([]byte("private message")); n != 0 || !errors.Is(err, writeErr) {
		t.Fatalf("Write=(%d,%v)", n, err)
	}
	if err := sink.Sync(); !errors.Is(err, syncErr) {
		t.Fatal(err)
	}
	for _, want := range []struct {
		op  string
		err error
	}{{"write", writeErr}, {"sync", syncErr}} {
		event := <-o.Events()
		if event.Operation != want.op || event.Output != "sink" || !errors.Is(event.Err, want.err) || event.OccurredAt.IsZero() || event.OccurredAt.Location().String() != "UTC" {
			t.Fatalf("event=%+v", event)
		}
	}
}

func TestFailureObserverOverflow(t *testing.T) {
	o := newTestObserver(t, 1)
	want := errors.New("write failed")
	sink, err := NewObservedWriteSyncer(failingObserverSink{writeErr: want}, o)
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		if _, err := sink.Write(nil); !errors.Is(err, want) {
			t.Fatal(err)
		}
	}
	if o.Dropped() != 9 {
		t.Fatalf("Dropped=%d", o.Dropped())
	}
	if event := <-o.Events(); !errors.Is(event.Err, want) {
		t.Fatal(event.Err)
	}
}

func TestFailureObserverValidation(t *testing.T) {
	for _, capacity := range []int{-1, 0, 1025} {
		if o, err := NewFailureObserver(capacity); o != nil || !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("capacity=%d: %v", capacity, err)
		}
	}
	var zero FailureObserver
	var nilObserver *FailureObserver
	for _, o := range []*FailureObserver{&zero, nilObserver} {
		if o.Events() != nil || o.Dropped() != 0 {
			t.Fatal("zero value must be safe to query")
		}
		if _, err := NewObservedWriteSyncer(zapcore.AddSync(io.Discard), o); !errors.Is(err, ErrInvalidConfig) {
			t.Fatal(err)
		}
	}
	if _, err := NewObservedWriteSyncer(nil, newTestObserver(t, 1)); !errors.Is(err, ErrInvalidConfig) {
		t.Fatal(err)
	}
}

func TestObservedWriteSyncerPreservesContracts(t *testing.T) {
	o := newTestObserver(t, 2)
	sink := &recordingSplitSink{}
	wrapped, err := NewObservedWriteSyncer(sink, o)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := wrapped.(io.Closer); ok {
		t.Fatal("wrapper must not take ownership of Close")
	}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), wrapped, zap.WarnLevel)
	hooks := 0
	logger := zap.New(zapcore.RegisterHooks(core, func(zapcore.Entry) error { hooks++; return nil })).Named("child").With(zap.String("key", "value"))
	logger.Info("must not be written")
	logger.Warn("written once")
	if err := logger.Sync(); err != nil {
		t.Fatal(err)
	}
	content, _, closes := sink.snapshot()
	if hooks != 1 || closes != 0 || bytes.Count([]byte(content), []byte("written once")) != 1 {
		t.Fatalf("hooks=%d closes=%d content=%s", hooks, closes, content)
	}
	select {
	case event := <-o.Events():
		t.Fatalf("success must not emit an event: %+v", event)
	default:
	}
}

func TestFailureObserverConcurrent(t *testing.T) {
	o := newTestObserver(t, 1)
	want := errors.New("shared failure")
	sink, err := NewObservedWriteSyncer(failingObserverSink{writeErr: want, syncErr: want}, o)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				_, _ = sink.Write(nil)
				_ = sink.Sync()
			}
		})
	}
	wg.Wait()
	if got := uint64(len(o.Events())) + o.Dropped(); got != 1600 {
		t.Fatalf("total events=%d", got)
	}
}

func TestObservedWriteSyncerTeeReportsPerSink(t *testing.T) {
	o := newTestObserver(t, 2)
	want := errors.New("output failed")
	cores := make([]zapcore.Core, 0, 2)
	for range 2 {
		sink, err := NewObservedWriteSyncer(failingObserverSink{writeErr: want}, o)
		if err != nil {
			t.Fatal(err)
		}
		cores = append(cores, zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), sink, zap.DebugLevel))
	}
	var diagnostic bytes.Buffer
	logger := zap.New(zapcore.NewTee(cores...), zap.ErrorOutput(zapcore.AddSync(&diagnostic)))
	logger.Info("private message")
	if len(o.Events()) != 2 {
		t.Fatalf("each sink must report independently, got=%d", len(o.Events()))
	}
	if !bytes.Contains(diagnostic.Bytes(), []byte(want.Error())) {
		t.Fatal("zap must retain its original error diagnostic")
	}
}

func TestFailureObserverConcurrentConsumer(t *testing.T) {
	o := newTestObserver(t, 1)
	want := errors.New("failure")
	sink, err := NewObservedWriteSyncer(failingObserverSink{writeErr: want}, o)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	counted := make(chan uint64, 1)
	go func() {
		var count uint64
		for {
			select {
			case <-o.Events():
				count++
			case <-done:
				counted <- count
				return
			}
		}
	}()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				_, _ = sink.Write(nil)
			}
		})
	}
	wg.Wait()
	close(done)
	consumed := <-counted
	if got := consumed + uint64(len(o.Events())) + o.Dropped(); got != 800 {
		t.Fatalf("total events=%d", got)
	}
}
