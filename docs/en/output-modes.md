# Output Modes

[繁體中文](../zh-TW/output-modes.md) | **English**

[Documentation index](README.md)

## Choosing a Mode

| Requirement | Entry point | Rotation | Ownership |
| --- | --- | --- | --- |
| Console or one file | `Configure` / `New` | None | Cleanup / Instance |
| Three daily files | `GetSplitCore` | Daily | Returned cleanup |
| Daily files with close errors | `GetSplitCoreWithCleanup` | Daily | Sync first, then `func() error` |
| Independent daily logger | `NewSplit` | Daily | Instance.Close syncs and closes |
| Global daily logger | `ConfigureSplit` | Daily | Cleanup syncs and closes |
| Global custom core | `ConfigureCore` | Core-defined | Explicit WithCoreCleanup |
| Direct daily output | `NewSplitOutput` / `NewSplitOutputWithOptions` | Daily | Caller closes it |
| Three custom sinks | `NewSplitCore` | Sink-defined | Caller manages sinks |

The `file` value in `Config.Outputs` writes one file and never creates `SplitOutput` automatically.

GetSplitCoreWithCleanup accepts the existing first three parameters plus optional FileOutputOption
values and returns `(core, func() error, error)`. It preserves Close errors without automatic
Sync or global publication. NewSplit and ConfigureSplit always create three daily files.

## Daily Level-Based Output

```go
core, cleanup, err := zlogger.GetSplitCore(
	"./logs",
	"app",
	zapcore.EncoderConfig{
		TimeKey:    "ts",
		LevelKey:   "level",
		MessageKey: "msg",
		EncodeTime: zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalLevelEncoder,
	},
)
if err != nil {
	return err
}
logger := zap.New(core)
defer func() {
	_ = logger.Sync()
	cleanup()
}()
```

- info file: DEBUG and INFO
- warn file: WARN
- error file: ERROR, DPANIC, PANIC, and FATAL

Cleanup stops the rotation goroutine and closes files. Stop using the logger, call `Sync`, and then
run cleanup. Use `GetSplitCoreWithOptions` for custom creation permissions.

When holding `SplitOutput` directly, `Close` is safe for repeated and concurrent calls. After
Close, `Write` and `Sync` return errors wrapping `os.ErrClosed`.

## Custom Sinks

### Failure observation

Create an observer with `NewFailureObserver(capacity)` (1–1024) and wrap the external sink using
`NewObservedWriteSyncer(sink, observer)` before passing it to a core. Successful operations emit
no events. Failures preserve the original write count and error. Events contain Operation
(write/sync/rotate/close), Output (sink/split), UTC OccurredAt, and Err. Full buffers drop new
events and increment Dropped. Core encoding errors are not covered; Check and hooks are untouched.
Each wrapped tee sink reports independently, without cross-wrapper deduplication.

Daily split option entry points accept `WithOutputObserver(observer)` and report Write, Sync,
rotation, and the one actual Close natively. Avoid wrapping the same output twice. Standard
NewWithOptions/ConfigureWithOptions reject this split-only option with ErrInvalidConfig.

### Level-based core

```go
core, err := zlogger.NewSplitCore(
	zapcore.NewJSONEncoder(encoderConfig),
	zlogger.SplitSinks{Info: infoSink, Warn: warnSink, Error: errorSink},
)
if err != nil {
	if errors.Is(err, zlogger.ErrInvalidSplitCore) {
		return fmt.Errorf("invalid split output: %w", err)
	}
	return err
}
logger := zap.New(core)
```

Use a different sink for every field. `NewSplitCore` clones the encoder but does not take sink
ownership or call `Close`. The caller must sync the logger and then close external sinks.

See [Security](security.md) for paths and permissions, and
[timberjack integration](integrations/timberjack.md) for size and compression rotation.
