# zlogger

zlogger is a Go structured logging package built on [zap](https://github.com/uber-go/zap).
Use a global logger or give each component its own Instance. It supports regular file output,
three daily files, output-failure notifications, and field redaction that you enable explicitly.

[![CI](https://github.com/vincent119/zlogger/actions/workflows/ci.yml/badge.svg)](https://github.com/vincent119/zlogger/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/gh/vincent119/zlogger/branch/main/graph/badge.svg)](https://codecov.io/gh/vincent119/zlogger)
[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

[繁體中文](README.md) | **English**

## Install

Requires Go 1.25 or later.

```bash
go get github.com/vincent119/zlogger
```

These docs follow the source on this branch. A published release may have fewer APIs.
If your installed version lacks an API shown here, check which version or commit includes it
before updating your dependency.

## Write your first log

This example uses the default console output. After initialization, you'll see
`service started` with a `component=api` field.

```go
package main

import (
	"fmt"
	"os"

	"github.com/vincent119/zlogger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cleanup, err := zlogger.Configure(nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := cleanup(); err != nil {
			fmt.Fprintln(os.Stderr, "close logger:", err)
		}
	}()

	zlogger.Info("service started", zlogger.String("component", "api"))
	if err := zlogger.Sync(); err != nil {
		fmt.Fprintln(os.Stderr, "sync logger:", err)
	}
	return nil
}
```

`Configure` can succeed once per process. Cleanup doesn't reset that limit.
Use `New` or `NewSplit` when you don't need globals.

Some terminals and pipes don't support Sync on stdout and return an operating-system error.
This example reports it without exiting. Handle file-output sync failures according to your
application's reliability requirements.

## Choose an entry point

| What you need | Entry point | How to close it |
| --- | --- | --- |
| Global console or single-file output | `Configure`, `ConfigureWithOptions` | Call `Sync`, then cleanup |
| Independent console or single-file logger | `New`, `NewWithOptions` | Call `Instance.Sync`, then `Instance.Close` |
| Global daily files | `ConfigureSplit` | Cleanup syncs, stops rotation, and closes files |
| Independent daily logger | `NewSplit` | `Instance.Close` syncs, stops rotation, and closes files |
| A daily core with close errors | `GetSplitCoreWithCleanup` | Sync the logger, then call cleanup |
| A custom core behind the global API | `ConfigureCore` | Cleanup syncs, then runs the registered `WithCoreCleanup` |
| Three external sinks routed by level | `NewSplitCore` | The caller syncs and closes each sink |

Daily routing sends DEBUG/INFO, WARN, and ERROR or higher to separate files.
`Config.Outputs=["file"]` creates a regular single file, not daily split output.
The older `GetSplitCore` returns a `func()` that discards close errors.
Use `GetSplitCoreWithCleanup` when you need those errors.

You can also pass a custom core straight to `zap.New(core)` without touching globals.
There is no non-global Instance constructor that accepts an arbitrary custom core.

## Output failures and sensitive data

`NewFailureObserver(capacity)` receives output-failure events. Wrap an external sink with
`NewObservedWriteSyncer`, or pass `WithOutputObserver` to a daily split constructor.
A full channel drops new events and increments `Dropped()` rather than blocking the write.
Delivery is best-effort. Core encoding errors aren't covered.

Redaction is off by default. Build a core with `NewRedactingCore(encoder, sink, level, cfg)`
and `RedactionConfig{Enabled: true, Keys: []string{"token", "password"}}`.
The policy is fixed at construction. Free-text messages stay unchanged.
Reflect, Stringer, and Error values are fully masked, which removes some diagnostic detail.

Regular `New`/`Configure` and daily split constructors don't enable redaction for you.
See [output modes](docs/en/output-modes.md) and [security](docs/en/security.md) for setup and limits.

## You choose configuration sources and field names

`ConfigPatch` distinguishes an omitted setting from an explicit `false`.
Your application loads configuration files or environment variables and decides which source
takes precedence, then zlogger fills defaults and validates the resulting configuration.

Field names are yours too.
Use `With` for fixed fields and `WithContext` for request fields.
Helpers such as `WithRequestID` and `WithTraceID` are conveniences, not a required schema.
zlogger doesn't extract OpenTelemetry context automatically.

## Read more

| What you need | Guide |
| --- | --- |
| Defaults, ConfigPatch, and loading configuration | [Configuration](docs/en/configuration.md) |
| Globals, independent loggers, Sync, Close, and ownership | [Lifecycle](docs/en/lifecycle.md) |
| Daily files, custom sinks, and failure events | [Output modes](docs/en/output-modes.md) |
| Custom fields, context, duplicate keys, and copies | [Context and fields](docs/en/context-and-fields.md) |
| Redaction examples, paths, and file permissions | [Security](docs/en/security.md) |
| HTTP request logging in your application | [Gin integration](docs/en/integrations/gin.md) |
| Size-based rotation, retention, and compression | [timberjack integration](docs/en/integrations/timberjack.md) |

[Documentation index](docs/en/README.md) · [GoDoc](https://pkg.go.dev/github.com/vincent119/zlogger) ·
[Design notes (Chinese)](DESIGN.md) · [Changelog](CHANGELOG.en.md)

## Develop and verify

Run these from the repository root:

```bash
make help
make verify
```

`make verify` runs formatting, vet, lint, race tests, the 90% coverage gate, and logger benchmark
smoke tests. Run `make vuln` separately for reachable vulnerabilities.
The Makefile pins tool versions.

CI runs race tests on Go 1.25.12 and 1.26.9, with compatibility jobs for Linux, macOS 15,
and Windows 2025. File-permission behavior still depends on the operating system.

## License

[MIT](LICENSE)
