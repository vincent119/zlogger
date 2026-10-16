# Initialization and cleanup

[繁體中文（台灣）](../zh-TW/lifecycle.md) | **English**

[Documentation index](README.md)

Choose whether you need globals, then decide who syncs and closes the output.
The constructor determines ownership. A returned `*zap.Logger` alone doesn't tell you who
must release the resources behind it.

## Entry points and cleanup

| Entry point | Sets zlogger and zap globals | Cleanup |
| --- | --- | --- |
| `Configure`, `ConfigureWithOptions` | Yes | Call `Sync`, then cleanup. Cleanup only closes owned resources |
| `New`, `NewWithOptions` | No | Call `Instance.Sync`, then `Instance.Close` |
| `ConfigureSplit` | Yes | Cleanup syncs, stops rotation, and closes files |
| `NewSplit` | No | `Instance.Close` syncs, stops rotation, and closes files |
| `ConfigureCore` | Yes | Cleanup syncs, then runs the registered callback |
| `GetSplitCoreWithCleanup` | No | Sync the logger, then call the returned `func() error` |
| `GetSplitCore`, `GetSplitCoreWithOptions` | No | Sync first. The legacy `func()` discards close errors |
| `zap.New(core)` | No | The caller syncs and closes sinks according to their ownership |

`Init(*Config)` remains for compatibility and may panic if initialization fails.
Use `Configure` in new code.

## Global configuration succeeds once

`Configure`, `ConfigureWithOptions`, `ConfigureSplit`, and `ConfigureCore` share one guard.
After the first success, another configuration attempt returns `ErrAlreadyConfigured`.
A failed attempt publishes no partial logger. Fix the cause and retry.

Cleanup is safe to call repeatedly or concurrently. It restores the previous globals
without resetting the guard. Stop all users of the logger first.
Don't publish or clean up global loggers while other goroutines are writing through them.

Global `SetLevel("debug")` changes the threshold.
Unknown strings fall back to `info` without an error, preserving the existing API behavior.

## Give a component its own Instance

Use `New` for regular outputs or `NewSplit` for daily files.
This snippet belongs inside a function returning `error`. Handle cleanup errors in your
application rather than discarding them as this short example does.

```go
cfg := zlogger.DefaultConfig()
cfg.LogPath = "./logs/worker"
cfg.FileName = "worker"

instance, err := zlogger.NewSplit(cfg)
if err != nil {
	return err
}
defer func() { _ = instance.Close() }()

if err := instance.SetLevel("debug"); err != nil {
	return err
}
instance.Logger().Info("worker started")
```

Use a different directory or prefix for each daily Instance.
`Instance.SetLevel` affects that Instance only. Invalid input returns `ErrInvalidConfig`
and leaves the current threshold unchanged.

`Instance.Close` accepts repeated and concurrent calls. Stop writing first.
After Close, don't use a logger you previously got from `Logger()`.
`Instance.Sync` and `Instance.SetLevel` return errors wrapping `os.ErrClosed`.
Daily instances close files even if Sync fails and join both errors.
Regular instances require a separate Sync call.

## Custom cores and ownership

`ConfigureCore(patch, core, opts...)` connects a custom core to the global API.
The core chooses encoding and output. Only Level, AddCaller, AddStacktrace, and Development
affect its logger, though every patch field still goes through Resolve validation.

External sinks remain yours.
If ConfigureCore's cleanup should also release those resources, register a
`WithCoreCleanup(func() error)` callback and let the same shutdown path handle both sync
and close errors.
Cleanup runs Sync first and still calls the callback if Sync fails, then joins the errors.
Nil or duplicate callbacks are rejected. Failed configuration doesn't run the callback
or take ownership, so the caller must release resources from a failed setup.

Global SetLevel intersects the core's own filter. It can't enable levels that core rejects.
For a non-global custom logger, use `zap.New(core)`.
There is no Instance constructor for an arbitrary custom core.

`NewSplit` and `ConfigureSplit` use LogPath as the directory and FileName as the prefix.
Outputs is validated but doesn't select the output destinations.

## Redaction and failure observers

`NewRedactingCore` needs an encoder with no pre-bound fields.
It builds its own core, keeps sink ownership with the caller, and starts no background work.
Pass the result to `zap.New` or `ConfigureCore`.

To observe sink errors as well, wrap the sink with `NewObservedWriteSyncer` before passing
it to `NewRedactingCore`. An entry rejected by redaction never reaches the sink and
doesn't produce a sink-failure event.

`FailureObserver` starts no goroutine and never closes its Events channel.
Give the consumer its own stop condition.

```go
for {
	select {
	case <-ctx.Done():
		return
	case event := <-observer.Events():
		// Use operation metadata for health checks. Keep raw errors out of external alerts.
		_ = event.Operation
	}
}
```

## Stop writers before cleanup

Stop accepting work, wait for logger users to finish, then sync and close according to
the table above. If you have an event consumer, finish output cleanup and drain any events
you need before stopping it. Notifications don't replace returned errors or zap ErrorOutput.
Delivery can fail when the channel is full.

By default, Fatal exits the process without running your shutdown sequence or deferred cleanup.
If the service needs graceful shutdown, return the error to the application and let it stop
work, sync logs, and release resources.
