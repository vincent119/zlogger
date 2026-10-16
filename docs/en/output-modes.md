# Output modes and failure events

[繁體中文（台灣）](../zh-TW/output-modes.md) | **English**

[Documentation index](README.md)

Regular file output writes one file. Use a split constructor for separate daily info,
warn, and error files. External sinks such as [timberjack](integrations/timberjack.md)
handle size-based rotation, retention, and compression.

## Choose an output

| What you need | Entry point | Ownership |
| --- | --- | --- |
| Console or a regular file | `Configure` / `New` | Sync, then cleanup / Close |
| Global daily files | `ConfigureSplit` | Cleanup syncs and closes |
| Independent daily files | `NewSplit` | Instance.Close syncs and closes |
| A daily core to assemble yourself | `GetSplitCoreWithCleanup` | Sync first, then cleanup with close errors |
| Direct access to daily output | `NewSplitOutput` / `NewSplitOutputWithOptions` | The caller syncs and closes |
| Three external sinks by level | `NewSplitCore` | The caller syncs and closes sinks |
| A global custom core | `ConfigureCore` | Cleanup syncs, then runs registered cleanup |

`GetSplitCore` and `GetSplitCoreWithOptions` remain available, but their legacy `func()`
discards Close errors. `GetSplitCoreWithCleanup` neither syncs automatically nor sets globals.

## Three daily files

The info file receives DEBUG and INFO, warn receives WARN, and error receives ERROR,
DPANIC, PANIC, and FATAL. Configured level thresholds still apply.
The default info threshold filters out DEBUG.

Add this function to your project to write daily files under ./logs.
Its returned error preserves both sync and close failures.

```go
package example

import (
	"errors"

	"github.com/vincent119/zlogger"
	"go.uber.org/zap"
)

func WriteDailyLog() (err error) {
	core, cleanup, err := zlogger.GetSplitCoreWithCleanup(
		"./logs", "app", zap.NewProductionEncoderConfig(),
	)
	if err != nil {
		return err
	}
	logger := zap.New(core)
	defer func() {
		err = errors.Join(err, logger.Sync(), cleanup())
	}()

	logger.Info("service started")
	return nil
}
```

This low-level core has no separate level gate, so every level supported by split routing
can pass through unless you add a gate or use NewSplit or ConfigureSplit to apply Config's threshold.
Those constructors also manage the Instance.

SplitOutput.Close accepts repeated and concurrent calls, stops rotation, and closes every file.
After Close, Write and Sync return errors wrapping `os.ErrClosed`.
Stop all writers before cleanup.

## Three external sinks

`NewSplitCore(encoder, SplitSinks{Info: infoSink, Warn: warnSink, Error: errorSink})`
clones the encoder and routes by the levels above. It neither sets globals nor takes ownership.
Supply three valid sinks backed by different output resources.
Invalid settings return `ErrInvalidSplitCore`.

Pass the core to `zap.New(core)` or `ConfigureCore`.
The caller still owns sync and close. Register `WithCoreCleanup` with ConfigureCore
if its cleanup should release the sinks. See [lifecycle](lifecycle.md).

## Receive output failures

Daily split constructors accept an observer directly.
Use this fragment inside a function returning error:

```go
observer, err := zlogger.NewFailureObserver(64)
if err != nil {
	return err
}
cfg := zlogger.DefaultConfig()
instance, err := zlogger.NewSplit(cfg, zlogger.WithOutputObserver(observer))
if err != nil {
	return err
}
defer func() { _ = instance.Close() }()
instance.Logger().Info("daily output started")
```

For an external sink, call `NewObservedWriteSyncer(sink, observer)` before passing it to a core.
The wrapper observes Write and Sync, preserves the original n and error, and has no Close method.
It doesn't make the sink safe for concurrent calls. Add zapcore.Lock first if needed.

| Event field | Content |
| --- | --- |
| Operation | write and sync, plus rotate and close for native split output |
| Output | sink for an external wrapper, split for native daily output |
| OccurredAt | UTC timestamp |
| Err | Original error, available to errors.Is / As |

Success emits no event. Capacity ranges from 1 to 1024.
A full channel drops new events and increments Dropped without waiting for a consumer.
Events contain no log body, though Err can still include sensitive data.
Each wrapped sink reports independently, with no tee aggregation or cross-wrapper deduplication.
Dropped failure notifications don't mean regular logs are being sampled.

Native split notifications are sent after releasing the output lock.
Repeated Close calls return the stored error, but only the actual Close emits an event.
Avoid observing the same output twice.

Regular `NewWithOptions` and `ConfigureWithOptions` reject WithOutputObserver with ErrInvalidConfig.
Core encoding errors and redaction rejections stay on the original error path or zap ErrorOutput.
They aren't sink failures.

The observer channel never closes.
Consumer shutdown is covered in [lifecycle](lifecycle.md).
Check [security](security.md) before forwarding raw errors.
