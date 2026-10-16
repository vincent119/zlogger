package zlogger_test

import (
	"errors"
	"fmt"

	"github.com/vincent119/zlogger"
)

type exampleFailureSink struct{}

func (exampleFailureSink) Write([]byte) (int, error) { return 0, errors.New("寫入失敗") }
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
		if _, err := sink.Write([]byte("服務訊息")); err == nil {
			panic("預期底層寫入錯誤")
		}
	}
	// 使用輪詢，不等待永不關閉的通道；長期消費者應 select 自己的 context.Done。
	select {
	case event := <-observer.Events():
		fmt.Println(event.Operation, event.Output)
	default:
		panic("預期故障事件")
	}
	// 僅輸出操作 metadata，不直接輸出可能包含敏感資訊的原 error。
	fmt.Println("丟棄事件：", observer.Dropped())
	// Output:
	// write sink
	// 丟棄事件： 1
}

func ExampleWithOutputObserver() {
	observer, err := zlogger.NewFailureObserver(1)
	if err != nil {
		panic(err)
	}
	_ = zlogger.WithOutputObserver(observer)
	// 將 option 傳入 NewSplit、ConfigureSplit、NewSplitOutputWithOptions
	// 或 GetSplitCoreWithCleanup；一般 NewWithOptions 不接受此 split 專屬 option。
}
