# Security and sensitive data

[繁體中文](../zh-TW/security.md) | **English**

[Documentation index](README.md)

Log only the fields you need to investigate a problem.
A full request, response, Config, or arbitrary struct can carry passwords, tokens, cookies,
or personal data. Redaction can't identify every secret inside free text.

## Mask a field explicitly

Use `Redacted` when you don't need the original value.
It never accepts that value, and automatic redaction settings don't affect it.

```go
zlogger.Info("authentication request",
	zlogger.Redacted("authorization"),
	zlogger.String("request_id", "req-123"),
)
```

The authorization field contains `[REDACTED]`. Other fields aren't scanned by this helper.

## Redact fields by name

`NewRedactingCore(encoder, sink, level, cfg)` builds its own core.
Redaction starts disabled. Set `Enabled: true` and provide `Keys` to enable it.
Regular and daily split constructors don't apply this policy automatically.

This complete example masks token and keeps request_id.

```go
package main

import (
	"fmt"
	"os"

	"github.com/vincent119/zlogger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	core, err := zlogger.NewRedactingCore(
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{MessageKey: "msg"}),
		zapcore.AddSync(os.Stdout),
		zap.InfoLevel,
		zlogger.RedactionConfig{
			Enabled: true,
			Keys:    []string{"token", "password"},
		},
	)
	if err != nil {
		return err
	}

	logger := zap.New(core)
	logger.With(zlogger.String("token", "example value")).Info(
		"authentication request", zlogger.String("request_id", "req-123"),
	)
	return nil
}
```

This writes to stdout and leaves os.Stdout open.
For files or other sinks, [sync and close the resources you own](lifecycle.md).
You can also pass the core to `ConfigureCore` to use zlogger's global API.

### The policy is fixed at construction

| Setting | Behavior |
| --- | --- |
| `Enabled=false` or zero-value config | Builds a regular zap core, ignoring Keys without validation |
| `Enabled=true` | Validates and copies Keys, isolating later edits to the original slice |
| Keys | 1 to 128 items, each 1 to 128 bytes long |
| Matching | ASCII case-insensitive, exact match on each field name |
| Duplicates | Merged after normalization |
| Whitespace, dots, regex | No trimming, path parsing, or regex matching |
| Runtime updates | Unsupported. Build a new core to change the policy |

`token` matches `TOKEN`. It doesn't match a field literally named `user.token`.
A token field inside a nested object still matches because each field name is checked
at its own level, regardless of the path used to reach it.

Nil encoder, sink, or level values, and invalid Keys when enabled, return `ErrInvalidConfig`.
Interface nil checks don't reliably detect typed nil pointers. Supply valid implementations.

### Which values are processed?

Matched fields become a fixed string before their marshalers run.
A matched namespace masks its subtree. Object, Array, and Inline values are processed recursively.

Reflect, Stringer, and Error values are fully masked even if their keys don't match.
Their String, Error, and MarshalJSON methods aren't called. You'll lose error details and some
object content. Any follows the Field type selected by zap, so arbitrary values aren't all
treated as inspectable objects.

Build the redacting core first.
Then bind With or WithLazy fields, or log context fields through it.
The supplied encoder must have no pre-bound fields, since earlier encoded content can't be redacted.

### Unsafe entries never reach the sink

Depth above 32, a recording budget above 1024, unknown Field types, marshaler errors, or
panics reject the entire entry with the fixed `ErrRedactionFailed` error.
The budget includes fields, array elements, and container/namespace operations.
It isn't a count of output JSON properties.

Nothing is partially written. Source errors aren't included.
With stores failures until a later Write because it can't return an error itself.
Direct core.Write returns the error, while logger calls send it to zap ErrorOutput.
A redaction failure doesn't reach the sink or produce a sink-failure event.

Messages, free text, field names, code evaluated before Field construction, external hooks,
pre-bound encoder fields, and original I/O errors are outside the protection boundary.
Marshalers remain trusted code. A marshaler that never returns can't be interrupted.

## Failure events can contain sensitive errors

`OutputFailure` stores no log body or fields, and Err is excluded from JSON/YAML serialization.
The original error can still contain paths or internal details.
Review it before forwarding alerts. Don't send an entire event formatted with `%+v`.

Watch `Dropped()` on the bounded observer channel.
Lost notifications leave gaps, so an empty channel doesn't prove that every write succeeded.

## Paths and file permissions

The caller chooses a trusted directory through log_path or the SplitOutput directory argument.
file_name and filePrefix must be a single filename, with no path components.
Dots used as `.` or `..`, slashes, backslashes, NUL, absolute paths, and Windows drive prefixes
are rejected.

Use `errors.Is(err, zlogger.ErrUnsafeLogPath)` to identify unsafe names.
Config validation also preserves `ErrInvalidConfig` in the error chain.

Files are opened through `os.OpenRoot` and root-relative Lstat/OpenFile calls.
Existing final symlinks are rejected, and os.Root confines path resolution during races.
The base directory itself can still be a symlink, and racing symlinks within the root may be
followed. Mount boundaries, special devices, and malicious filesystems aren't isolated.
Protect the directory and configuration sources yourself.
Go has standard-library limits on js, plan9, and wasip1. This project's compatibility jobs
cover Linux, macOS, and Windows.

New directories default to `0700`. New files default to `0600`.
To change permissions:

```go
instance, err := zlogger.NewWithOptions(
	cfg,
	zlogger.WithDirPerm(0o750),
	zlogger.WithFilePerm(0o640),
)
```

This fragment uses your application's cfg. Handle err and clean up the returned Instance.
Directory modes must include owner rwx, and file modes must include owner rw.
Other-write and non-permission bits are rejected with `ErrInvalidFilePermission`.
The last option of the same kind wins.

Only new objects are affected. The umask can restrict permissions further, and existing
permissions stay unchanged. Windows accepts the API without guaranteeing POSIX mode behavior.
Permission options aren't configuration-file fields.
