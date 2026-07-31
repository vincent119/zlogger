# Initialization and Lifecycle

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

1. Stop accepting new work.
2. Wait for in-flight work that uses the logger.
3. Call global `Sync` or `Instance.Sync`.
4. Run Configure cleanup or `Instance.Close`.

`SetLevel` changes the global level at runtime. Unknown strings preserve legacy behavior and fall
back to `info` without returning an error.
