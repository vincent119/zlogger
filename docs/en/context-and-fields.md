# Context and fields

[繁體中文（台灣）](../zh-TW/context-and-fields.md) | **English**

[Documentation index](README.md)

Choose your own field names.
zlogger doesn't require service, env, or trace_id fields. OpenTelemetry extraction is left to
your application, so using these helpers won't add a tracing dependency or change the field
names your existing dashboards expect.

## Pass your own fields

Configure the global logger first. Here, ctx comes from the request.

```go
ctx = zlogger.WithContext(ctx,
	zlogger.String("tenant", "example"),
	zlogger.String("job", "import"),
)
zlogger.InfoContext(ctx, "import started", zlogger.Int("batch_size", 100))
```

Bind fixed fields with `zlogger.With(fields...)`.
InfoContext uses the global logger. It doesn't select an independent Instance.
To attach context fields to an Instance logger, use
`instance.Logger().With(zlogger.FromContext(ctx)...)`.

## Names used by the helpers

| Helper | Field key | When it adds nothing |
| --- | --- | --- |
| `WithRequestID` | `request_id` | Empty string |
| `WithTraceID` | `trace_id` | Empty string |
| `WithOperation` | `operation` | Empty string |
| `WithComponent` | `component` | Empty string |
| `WithUserID` | `user_id` | nil |

Using req_id already? Call `WithContext(ctx, String("req_id", value))`.
Keep your existing query fields. There's no need to add request_id too.
These helpers don't validate tracing formats.

## Copies and merge order

`WithContext` copies the input field slice, and `FromContext` returns a slice copy.
Both copies are shallow. If a Field references a map, slice, pointer, or object that another
goroutine can change, the caller must still arrange synchronization or provide its own copy.

InfoContext and its related functions place context fields first, then append call-site fields.
zlogger doesn't deduplicate keys or promise that a later key overwrites an earlier one.
The encoder and downstream parser decide. Avoid duplicate keys.

## Field helpers and examples

These are the 27 helpers currently defined in fields.go.
Each returns a Field. Pass it to Info, With, or another logging entry point to use it.
The complete example below defines readErr, cacheErr, at, and endpoint.
JSON descriptions assume a regular zap JSON encoder with automatic redaction disabled.

| Function | What it does | Example |
| --- | --- | --- |
| `String` | A string. JSON keeps it as a string. | `zlogger.String("status", "ready")` |
| `Strings` | A string slice, encoded as a JSON array. | `zlogger.Strings("tags", []string{"api", "worker"})` |
| `Int` | A signed Go int. | `zlogger.Int("attempt", 2)` |
| `Int8` | An 8-bit signed integer. | `zlogger.Int8("delta8", int8(-8))` |
| `Int16` | A 16-bit signed integer. | `zlogger.Int16("delta16", int16(-16))` |
| `Int32` | A 32-bit signed integer. | `zlogger.Int32("delta32", int32(-32))` |
| `Int64` | A 64-bit signed integer. | `zlogger.Int64("delta64", int64(-64))` |
| `Uint` | An unsigned Go uint. | `zlogger.Uint("count", uint(2))` |
| `Uint8` | An 8-bit unsigned integer. | `zlogger.Uint8("count8", uint8(8))` |
| `Uint16` | A 16-bit unsigned integer. | `zlogger.Uint16("count16", uint16(16))` |
| `Uint32` | A 32-bit unsigned integer. | `zlogger.Uint32("count32", uint32(32))` |
| `Uint64` | A 64-bit unsigned integer. | `zlogger.Uint64("count64", uint64(64))` |
| `Float32` | A 32-bit floating-point value. | `zlogger.Float32("ratio32", float32(0.5))` |
| `Float64` | A 64-bit floating-point value. | `zlogger.Float64("ratio64", 0.75)` |
| `Bool` | A boolean, written as true or false in JSON. | `zlogger.Bool("cached", true)` |
| `Err` | An error under the fixed key error. Nil adds no field. | `zlogger.Err(readErr)` |
| `NamedError` | An error under your chosen key. Nil adds no field. | `zlogger.NamedError("cache_error", cacheErr)` |
| `Any` | Selects a zap field from the value's type, falling back to reflection. | `zlogger.Any("payload", map[string]int{"items": 3})` |
| `Duration` | A time.Duration formatted by EncodeDuration. | `zlogger.Duration("elapsed", 1500*time.Millisecond)` |
| `Time` | A time.Time formatted by EncodeTime. | `zlogger.Time("at", at)` |
| `Binary` | Binary data. The zap JSON encoder writes Base64. | `zlogger.Binary("blob", []byte{0x01, 0x02, 0x03})` |
| `ByteString` | Bytes treated as UTF-8 text rather than Base64. | `zlogger.ByteString("text", []byte("hello"))` |
| `Stringer` | Calls String() when the field is encoded, not when it's constructed. | `zlogger.Stringer("endpoint", endpoint)` |
| `Reflect` | Uses the encoder's reflected-value path for maps, structs, and other values. | `zlogger.Reflect("options", map[string]bool{"dry_run": true})` |
| `Stack` | Captures the current goroutine's stack immediately as a string field. | `zlogger.Stack("stack")` |
| `StackSkip` | Captures a stack with the requested number of top frames skipped. | `zlogger.StackSkip("caller_stack", 1)` |
| `Redacted` | Writes [REDACTED] without accepting the original secret. | `zlogger.Redacted("authorization")` |

### Nil values, types, and encoding

Err(nil) and NamedError(key, nil) omit the field.
Any(key, nil) writes null with a regular JSON encoder, while Strings(key, nil) writes an empty array.
Here, nil means a nil interface. An interface holding a nil pointer is different and needs
handling appropriate to its concrete type.

Use Binary for raw bytes.
ByteString expects UTF-8 text. The blob example becomes `"AQID"`, while text becomes `"hello"`.
Any treats []byte as Binary, so it writes Base64 too.
Choose Binary or ByteString directly when that distinction matters.

Any recognizes common scalars, slices, errors, Stringers, and zap ObjectMarshaler/ArrayMarshaler values.
A plain map or struct without a recognized interface falls back to Reflect.
Reflected encoding can invoke custom serialization methods, so it doesn't make arbitrary data safe to log.
Slices, maps, and objects aren't deep-copied. Don't mutate shared data before it has been encoded.

JSON output doesn't guarantee that a collector preserves the original integer width.
Large Int64/Uint64 identifiers can lose precision when a consumer parses JSON numbers as floating point.
Use a string if an identifier must survive character for character.
Writing Float32 as JSON doesn't add precision either.

Time and Duration follow the encoder.
They aren't always RFC3339 timestamps or milliseconds. The complete example selects RFC3339
and string durations, giving at as `"2026-10-16T09:00:00Z"` and elapsed as `"1.5s"`.

### When errors, Stringers, and stacks are evaluated

A regular core gets error and Stringer text during encoding.
Binding fields with With can trigger that encoding too, so don't assume String or Error waits
for a later log call. Some error types also produce verbose fields.
Check the value for sensitive data first.

Stack and StackSkip capture the stack when the Field is constructed.
That work happens even if the level is later filtered out. Use a non-negative skip.
The first visible frame also depends on wrappers in zlogger or your application.
Check the output as you tune it. This is separate from the logger's AddCallerSkip setting.

For an existing logger, check the level before building a stack field:

```go
if entry := logger.Check(zap.DebugLevel, "diagnostics"); entry != nil {
	entry.Write(zlogger.Stack("stack"))
}
```

This fragment needs zap and zlogger imports.
Stack is constructed only after Check accepts the entry.

### Run the complete example

Save this as main.go in a module that depends on zlogger, then run `go run .`.
It writes to memory and prints JSON. No files or globals.
The first entry uses all 27 helpers. Two more entries show nil errors and empty values.

```go
package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/vincent119/zlogger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	var output bytes.Buffer
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey:     "msg",
		EncodeTime:     zapcore.RFC3339TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	})
	logger := zap.New(zapcore.NewCore(encoder, zapcore.AddSync(&output), zap.DebugLevel))
	readErr := errors.New("read failed")
	cacheErr := errors.New("cache miss")
	at := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	endpoint := &url.URL{Scheme: "https", Host: "example.com"}

	logger.Info("field examples",
		zlogger.String("status", "ready"),
		zlogger.Strings("tags", []string{"api", "worker"}),
		zlogger.Int("attempt", 2),
		zlogger.Int8("delta8", int8(-8)),
		zlogger.Int16("delta16", int16(-16)),
		zlogger.Int32("delta32", int32(-32)),
		zlogger.Int64("delta64", int64(-64)),
		zlogger.Uint("count", uint(2)),
		zlogger.Uint8("count8", uint8(8)),
		zlogger.Uint16("count16", uint16(16)),
		zlogger.Uint32("count32", uint32(32)),
		zlogger.Uint64("count64", uint64(64)),
		zlogger.Float32("ratio32", float32(0.5)),
		zlogger.Float64("ratio64", 0.75),
		zlogger.Bool("cached", true),
		zlogger.Err(readErr),
		zlogger.NamedError("cache_error", cacheErr),
		zlogger.Any("payload", map[string]int{"items": 3}),
		zlogger.Duration("elapsed", 1500*time.Millisecond),
		zlogger.Time("at", at),
		zlogger.Binary("blob", []byte{0x01, 0x02, 0x03}),
		zlogger.ByteString("text", []byte("hello")),
		zlogger.Stringer("endpoint", endpoint),
		zlogger.Reflect("options", map[string]bool{"dry_run": true}),
		zlogger.Stack("stack"),
		zlogger.StackSkip("caller_stack", 1),
		zlogger.Redacted("authorization"),
	)
	logger.Info("no error", zlogger.Err(nil), zlogger.NamedError("cache_error", nil))
	logger.Info("empty values", zlogger.Strings("tags", nil), zlogger.Any("missing", nil))

	if err := logger.Sync(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	fmt.Print(output.String())
}
```

Stack paths, line numbers, and function names depend on where you run it.
Don't compare them byte for byte.
With automatic redaction disabled, the second entry has only msg,
and the third includes `"tags":[],"missing":null`.

## Add redaction before binding fields

Build `NewRedactingCore` before using With, WithLazy, or context fields.
Disabled redaction keeps the regular behavior. Manual `Redacted` fields always contain
the fixed replacement string.

Enabled redaction matches the field name at each nesting level, not a parent.token path.
Reflect, Stringer, and Error values are fully masked even when their keys don't match.
See [security](security.md) for setup and failure handling.
