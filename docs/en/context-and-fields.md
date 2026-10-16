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

fields.go defines 47 helpers. This table covers the original 27; the following section adds 20 structured-field and typed-slice helpers.
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

## Structured fields and typed slices

These 20 helpers delegate to the corresponding zap functions. Together with the table above,
fields.go provides 47 field helpers. The complete example below defines profile and codes;
at refers to the time value in the earlier example.
Any already recognizes ObjectMarshaler and ArrayMarshaler. These explicit helpers make the
intent clearer rather than adding a new object encoder.

| Function | Behavior | Example |
| --- | --- | --- |
| `Object` | Encodes a nested ObjectMarshaler without reflection. | `zlogger.Object("profile", profile{"reader"})` |
| `Array` | Encodes elements in ArrayMarshaler order. | `zlogger.Array("items", codes{3, 1})` |
| `Inline` | Expands object fields at the current level without deduplicating keys. | `zlogger.Inline(profile{"inline"})` |
| `Namespace` | Places subsequent fields in the named child object. | `zlogger.Namespace("request")` |
| `Bools` | An ordered bool slice. | `zlogger.Bools("values", []bool{true, false})` |
| `Ints` | An ordered int slice. | `zlogger.Ints("values", []int{-2, 1})` |
| `Int8s` | An ordered int8 slice. | `zlogger.Int8s("values", []int8{-2, 1})` |
| `Int16s` | An ordered int16 slice. | `zlogger.Int16s("values", []int16{-2, 1})` |
| `Int32s` | An ordered int32 slice. | `zlogger.Int32s("values", []int32{-2, 1})` |
| `Int64s` | An ordered int64 slice. | `zlogger.Int64s("values", []int64{-2, 1})` |
| `Uints` | An ordered uint slice. | `zlogger.Uints("values", []uint{2, 1})` |
| `Uint8s` | A numeric uint8 array, distinct from ByteStrings and Binary. | `zlogger.Uint8s("values", []uint8{2, 1})` |
| `Uint16s` | An ordered uint16 slice. | `zlogger.Uint16s("values", []uint16{2, 1})` |
| `Uint32s` | An ordered uint32 slice. | `zlogger.Uint32s("values", []uint32{2, 1})` |
| `Uint64s` | An ordered uint64 slice. | `zlogger.Uint64s("values", []uint64{2, 1})` |
| `Float32s` | An ordered float32 slice. | `zlogger.Float32s("values", []float32{1.5, -2.25})` |
| `Float64s` | An ordered float64 slice. | `zlogger.Float64s("values", []float64{1.5, -2.25})` |
| `Durations` | Durations formatted by EncodeDuration. | `zlogger.Durations("values", []time.Duration{time.Second, 1500 * time.Millisecond})` |
| `Times` | Times formatted by EncodeTime. | `zlogger.Times("values", []time.Time{at})` |
| `ByteStrings` | An array of UTF-8 strings, not Base64 or numeric bytes. | `zlogger.ByteStrings("values", [][]byte{[]byte("alpha"), nil, {}})` |

### Marshaler and encoder aliases

These are true Go type aliases, identical to the zapcore interfaces rather than new types.

| Alias | Upstream interface and methods |
| --- | --- |
| `ObjectMarshaler` | `zapcore.ObjectMarshaler`, with `MarshalLogObject(ObjectEncoder) error` |
| `ArrayMarshaler` | `zapcore.ArrayMarshaler`, with `MarshalLogArray(ArrayEncoder) error` |
| `ObjectEncoder` | `zapcore.ObjectEncoder`, retaining all methods such as AddString, AddArray, and AddObject |
| `ArrayEncoder` | `zapcore.ArrayEncoder`, retaining all methods such as AppendInt, AppendArray, and AppendObject |

Applications can construct these fields and implement marshalers using only zlogger imports.
The underlying zap dependency remains. Custom core and encoder construction can still require
direct zap imports.

### Evaluation, nil values, and scope

Constructing an Object, Array, or Inline Field does not invoke its marshaler.
The core evaluates it during encoding, which can happen when With binds fields.
WithLazy follows zap's evaluation timing. Helpers do not copy objects or slices.
Keep data stable until encoding and synchronize concurrent access yourself.

With a regular JSON encoder, nil and empty typed slices both produce `[]`.
A nil element in ByteStrings becomes an empty string; Uint8s keeps numeric values.
There is no extra nil-marshaler handling. An interface holding a nil pointer follows its
concrete method and core behavior, not the typed-slice nil policy.

Namespace affects subsequent fields, including fields added when logging through a derived
logger. A With child does not change its parent.
Inline can introduce duplicate keys. These wrappers neither deduplicate nor promise
replacement. A downstream JSON parser may retain only the last value.

### A complete example using only zlogger

Save this as main.go and run `go run .`. It creates an independent Instance, writes to a
temporary file, syncs and closes it, then prints the contents. It removes its own temporary
directory and does not configure a global logger.

```go
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vincent119/zlogger"
)

type profile struct{ name string }

func (p profile) MarshalLogObject(encoder zlogger.ObjectEncoder) error {
	encoder.AddString("name", p.name)
	return encoder.AddArray("codes", codes{2, 1})
}

type codes []int

func (values codes) MarshalLogArray(encoder zlogger.ArrayEncoder) error {
	for _, value := range values {
		encoder.AppendInt(value)
	}
	return nil
}

var (
	_ zlogger.ObjectMarshaler = profile{}
	_ zlogger.ArrayMarshaler  = codes{}
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func run() (result error) {
	directory, err := os.MkdirTemp("", "zlogger-structured-fields-")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, os.RemoveAll(directory)) }()
	cfg := zlogger.DefaultConfig()
	cfg.Format, cfg.Outputs = "json", []string{"file"}
	cfg.LogPath, cfg.FileName, cfg.AddCaller = directory, "fields.log", false
	instance, err := zlogger.New(cfg)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, instance.Close()) }()
	instance.Logger().Info("structured fields",
		zlogger.Object("profile", profile{"reader"}),
		zlogger.Array("items", codes{3, 1}),
		zlogger.Inline(profile{"inline"}),
		zlogger.Ints("attempts", []int{1, 2}),
		zlogger.Uint8s("numbers", []uint8{65, 66}),
		zlogger.ByteStrings("texts", [][]byte{[]byte("AB")}),
		zlogger.Durations("elapsed", []time.Duration{1500 * time.Millisecond}),
		zlogger.Times("started_at", []time.Time{time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)}),
		zlogger.Namespace("request"),
		zlogger.String("id", "r1"),
	)
	if err := instance.Sync(); err != nil {
		return err
	}
	if err := instance.Close(); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(directory, cfg.FileName))
	if err != nil {
		return err
	}
	_, err = fmt.Print(string(data))
	return err
}
```

The output contains an initialization entry and an application entry; ts varies per run.
In the application entry, profile is nested, items preserves `[3,1]` order, and Inline places
name and codes at the top level. request.id belongs to a child object.
numbers is `[65,66]`, while texts is `["AB"]`. With the default encoder, elapsed is
`["1.5s"]` and started_at contains a time string. The helpers do not hard-code those formats.

## Add redaction before binding fields

Build `NewRedactingCore` before using With, WithLazy, or context fields.
Disabled redaction keeps the regular behavior. Manual `Redacted` fields always contain
the fixed replacement string.

Enabled redaction matches the field name at each nesting level, not a parent.token path.
Reflect, Stringer, and Error values are fully masked even when their keys don't match.
The new helpers follow the same policy. Matched Object or Array containers are not evaluated,
and a matched Namespace suppresses its subtree. Inline fields are checked in their expanded
scope. Encoding errors, panics, and depth or budget overruns reject the entire entry without
partial output. Opaque maps and structs still use the existing whole-field policy.
Marshalers remain caller code, so these wrappers do not isolate arbitrary side effects or
stop a marshaler that never returns.
See [security](security.md) for setup and failure handling.
