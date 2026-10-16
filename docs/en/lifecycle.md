# Initialization and Lifecycle

## 遮罩 core 的生命週期

`NewRedactingCore` 不接受任意既有 core，不取得 sink ownership，也不建立 worker。
可使用 ConfigureCore 與 WithCoreCleanup 沿用原清理契約。參見[生命週期](../zh-TW/lifecycle.md)。

[繁體中文](../zh-TW/lifecycle.md) | **English**

[Documentation index](README.md)

## Entry Point Comparison

| Entry point | Error handling | Resource responsibility | Global effect |
| --- | --- | --- | --- |
| `Configure` | Returns an error | Run cleanup | Sets zlogger and zap globals |
| `ConfigureWithOptions` | Returns an error | Run cleanup | Sets zlogger and zap globals |
| `Init` | May panic | Compatibility path | Sets zlogger and zap globals |
| `New` / `NewWithOptions` | Returns an error | `Instance.Close` | None |
| Direct `zap.New` | Caller-defined | Caller manages sinks | None unless `zap.ReplaceGlobals` is called |

## Global Logger

`Configure` can succeed only once in a process. Failure does not publish partial state and may be
retried after correction. Calls after the first success return `ErrAlreadyConfigured`. Cleanup is
idempotent, but it does not permit another successful Configure call.

```go
cleanup, err := zlogger.Configure(nil)
if err != nil {
	return err
}
defer func() { _ = cleanup() }()
```

`Init(*Config)` remains only for source compatibility. It cannot return configuration or I/O
errors and may panic on initialization failure. New applications should use `Configure`.

## Instance Logger

`NewSplit(cfg, opts...)` returns the existing Instance type with independent daily files and
never changes globals. Instance.SetLevel affects only that instance; invalid levels return
ErrInvalidConfig and nil or closed instances return an error wrapping os.ErrClosed. Use distinct
directories or prefixes. Split Instance.Close automatically syncs, stops the worker and closes
files, joining errors even when Sync fails. Standard instances retain explicit Sync before Close.

```go
instance, err := zlogger.New(zlogger.DefaultConfig())
if err != nil {
	return err
}
defer func() { _ = instance.Close() }()

instance.Logger().Info("service started")
if err := instance.Sync(); err != nil {
	return err
}
```

`Instance.Close` is safe for repeated and concurrent calls. Do not use the logger returned by
`Logger()` after Close. `Instance.Sync` then returns an error wrapping `os.ErrClosed`.

## Shutdown Order

FailureObserver starts no goroutine and never closes Events. Use your own context in a select
loop, not an unbounded range waiting for channel closure. Stop logger users, run Sync/cleanup,
drain any required events, then stop the consumer. Shared observers are not owned by logger
cleanup. Notifications do not replace returned errors or zap ErrorOutput. Split notifications
run after releasing the output lock; repeated concurrent Close calls report only the one actual
Close failure while preserving the stored error for every caller.

ConfigureCore and ConfigureSplit share Configure's one-success guard and publish both zlogger
and zap globals. Their concurrent-safe, idempotent cleanup restores previous globals without
resetting the guard. Stop all logger users first. ConfigureCore syncs before an explicitly
registered WithCoreCleanup callback and does not automatically close external sinks. Failed
configuration leaves ownership with the caller; nil or duplicate callbacks are invalid.
ConfigureSplit delegates to split Instance.Close, syncing and closing once. Errors are joined;
existing Configure cleanup remains Close-only. Dynamic levels intersect the core's own filter.

ConfigureCore uses Level, AddCaller, AddStacktrace and Development; the core defines encoding
and outputs. Other fields still undergo Resolve validation. NewSplit and ConfigureSplit use
LogPath as directory and FileName as prefix. Outputs is validated but does not select outputs;
Format and ColorEnabled follow existing encoder behavior.

1. Stop accepting new work.
2. Wait for in-flight work that uses the logger.
3. Call global `Sync` or `Instance.Sync`.
4. Run Configure cleanup or `Instance.Close`.

`SetLevel` changes the global level at runtime. Unknown strings preserve legacy behavior and fall
back to `info` without returning an error.
