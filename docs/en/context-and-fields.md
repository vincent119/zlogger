# Context and Fields

[繁體中文](../zh-TW/context-and-fields.md) | **English**

[Documentation index](README.md)

## Request-Scoped Fields

```go
ctx := zlogger.WithRequestID(context.Background(), "req-123")
ctx = zlogger.WithUserID(ctx, 12345)
ctx = zlogger.WithTraceID(ctx, "trace-abc")
ctx = zlogger.WithOperation(ctx, "login")
ctx = zlogger.WithComponent(ctx, "auth")

zlogger.InfoContext(ctx, "process request", zlogger.String("action", "login"))
fields := zlogger.FromContext(ctx)
```

Empty strings do not add request ID, trace ID, operation, or component fields. A nil user ID is
also ignored. Use `WithContext` for arbitrary fields.

## Copy and Merge Contract

`WithContext` copies its input slice, and `FromContext` returns a defensive copy. Callers cannot
mutate context fields through a shared backing array.

`DebugContext`, `InfoContext`, `WarnContext`, `ErrorContext`, and `FatalContext` place context
fields first and append call-site fields. Duplicate-key behavior depends on the zap encoder or
consumer; avoid intentionally creating duplicate keys.

## Field Helpers

| Category | Helpers |
| --- | --- |
| Strings | `String`, `Strings`, `ByteString` |
| Integers | `Int`, `Int8`, `Int16`, `Int32`, `Int64` |
| Unsigned integers | `Uint`, `Uint8`, `Uint16`, `Uint32`, `Uint64` |
| Float and bool | `Float32`, `Float64`, `Bool` |
| Time | `Duration`, `Time` |
| Errors | `Err`, `NamedError` |
| Other | `Any`, `Binary`, `Reflect`, `Stringer`, `Stack`, `StackSkip` |

Before logging an arbitrary struct, confirm that it contains no secrets. See
[Security](security.md) for sensitive-data rules.
