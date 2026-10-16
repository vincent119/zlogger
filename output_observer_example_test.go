package zlogger_test

import (
	"errors"
	"fmt"

	"github.com/vincent119/zlogger"
)

type exampleFailureSink struct{}

func (exampleFailureSink) Write([]byte) (int, error) { return 0, errors.New("write failed") }
func (exampleFailureSink) Sync() error               { return nil }

func ExampleFailureObserver() {
	observer, err := zlogger.NewFailureObserver(1)
	if err != nil {
		panic(err)
	}
	sink, err := zlogger.NewObservedWriteSyncer(exampleFailureSink{}, observer)
	if err != nil {
		panic(err)
	}
	for range 2 {
		if _, err := sink.Write([]byte("service message")); err == nil {
			panic("expected the underlying write error")
		}
	}
	// Poll the channel rather than waiting for it to close. Long-lived consumers should select on their own context.Done.
	select {
	case event := <-observer.Events():
		fmt.Println(event.Operation, event.Output)
	default:
		panic("expected a failure event")
	}
	// Print operation metadata only. The original error may contain sensitive information.
	fmt.Println("dropped events:", observer.Dropped())
	// Output:
	// write sink
	// dropped events: 1
}

func ExampleWithOutputObserver() {
	observer, err := zlogger.NewFailureObserver(1)
	if err != nil {
		panic(err)
	}
	_ = zlogger.WithOutputObserver(observer)
	// Pass the option to NewSplit, ConfigureSplit, NewSplitOutputWithOptions,
	// or GetSplitCoreWithCleanup. NewWithOptions does not accept this split-only option.
}
