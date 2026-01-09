# zlogger

[![GitHub](https://img.shields.io/badge/github-vincent119/zlogger-blue?logo=github)](https://github.com/vincent119/zlogger)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.19+-00ADD8?logo=go)](https://go.dev/)
[![CI](https://github.com/vincent119/zlogger/actions/workflows/ci.yml/badge.svg)](https://github.com/vincent119/zlogger/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/vincent119/zlogger/branch/main/graph/badge.svg)](https://codecov.io/gh/vincent119/zlogger)
[![Go Report Card](https://goreportcard.com/badge/github.com/vincent119/zlogger)](https://goreportcard.com/report/github.com/vincent119/zlogger)

A structured logging library based on [zap](https://github.com/uber-go/zap), providing a simple API and rich features.

**[繁體中文](README.md)**

## Installation

```bash
go get github.com/vincent119/zlogger
```

## Basic Usage

```go
import "github.com/vincent119/zlogger"

func main() {
    // Initialize with default config
    zlogger.Init(nil)
    defer zlogger.Sync()

    // Log messages
    zlogger.Info("Hello", zlogger.String("key", "value"))
    zlogger.Debug("Debug message", zlogger.Int("count", 42))
    zlogger.Error("Error occurred", zlogger.Err(err))
}
```

## Custom Configuration

```go
cfg := &zlogger.Config{
    Level:       "debug",
    Format:      "json",
    Outputs:     []string{"console", "file"},
    LogPath:     "./logs",
    FileName:    "app.log",
    AddCaller:   true,
    Development: true,
}
zlogger.Init(cfg)
```

## Load from Config File

### YAML Example

```yaml
# config.yaml
log:
  level: debug
  format: json
  outputs:
    - console
    - file
  log_path: ./logs
  file_name: app.log
  add_caller: true
  add_stacktrace: false
  development: false
  color_enabled: true
```

```go
type AppConfig struct {
    Log zlogger.Config `yaml:"log"`
}

// After loading config
var appConfig AppConfig
// ... load YAML ...
zlogger.Init(&appConfig.Log)
```

### JSON Example

```json
{
  "log": {
    "level": "info",
    "format": "json",
    "outputs": ["console"],
    "log_path": "./logs",
    "add_caller": true,
    "color_enabled": true
  }
}
```

## Context Support

```go
// Create context with tracing info
ctx := zlogger.WithRequestID(context.Background(), "req-123")
ctx = zlogger.WithUserID(ctx, 12345)
ctx = zlogger.WithTraceID(ctx, "trace-abc")

// Log with context (automatically includes tracing info)
zlogger.InfoContext(ctx, "Processing request", zlogger.String("action", "login"))
zlogger.ErrorContext(ctx, "Request failed", zlogger.Err(err))
```

## Field Helper Functions

```go
zlogger.String("key", "value")
zlogger.Int("count", 42)
zlogger.Int64("id", 123456789)
zlogger.Float64("price", 99.99)
zlogger.Bool("active", true)
zlogger.Err(err)
zlogger.Any("data", someStruct)
zlogger.Duration("latency", time.Second)
zlogger.Time("timestamp", time.Now())
```

## Dynamic Log Level

```go
zlogger.SetLevel("debug")  // Change level at runtime
```

## Split Log Files by Level

```go
import "go.uber.org/zap/zapcore"

// Create split output core
core, cleanup, err := zlogger.GetSplitCore("./logs", "app", zapcore.EncoderConfig{
    TimeKey:        "ts",
    LevelKey:       "level",
    MessageKey:     "msg",
    EncodeTime:     zapcore.ISO8601TimeEncoder,
    EncodeLevel:    zapcore.CapitalLevelEncoder,
})
if err != nil {
    panic(err)
}
defer cleanup()

// Creates the following files:
// - logs/app-info-2024-01-01.log
// - logs/app-warn-2024-01-01.log
// - logs/app-error-2024-01-01.log
```

## Log Rotation (using timberjack)

zlogger does not include log rotation. We recommend using [timberjack](https://github.com/DeRuina/timberjack):

```bash
go get github.com/DeRuina/timberjack
```

```go
package main

import (
    "github.com/DeRuina/timberjack"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

func main() {
    // Configure timberjack log rotation
    tjLogger := &timberjack.Logger{
        Filename:   "./logs/app.log",
        MaxSize:    100,   // Max size per file (MB)
        MaxBackups: 10,    // Max number of backups
        MaxAge:     30,    // Days to keep
        Compress:   true,  // Compress old logs (gzip)
    }

    // Create encoder config
    encoderConfig := zapcore.EncoderConfig{
        TimeKey:        "ts",
        LevelKey:       "level",
        NameKey:        "logger",
        CallerKey:      "caller",
        MessageKey:     "msg",
        StacktraceKey:  "stacktrace",
        LineEnding:     zapcore.DefaultLineEnding,
        EncodeLevel:    zapcore.CapitalLevelEncoder,
        EncodeTime:     zapcore.ISO8601TimeEncoder,
        EncodeDuration: zapcore.StringDurationEncoder,
        EncodeCaller:   zapcore.ShortCallerEncoder,
    }

    // Create core
    core := zapcore.NewCore(
        zapcore.NewJSONEncoder(encoderConfig),
        zapcore.AddSync(tjLogger),
        zap.InfoLevel,
    )

    // Create logger
    logger := zap.New(core, zap.AddCaller())
    zap.ReplaceGlobals(logger)

    logger.Info("Server started", zap.String("port", "8080"))
}
```

## Configuration Options

| Option           | Type     | Default       | Description                               |
| ---------------- | -------- | ------------- | ----------------------------------------- |
| `level`          | string   | `"info"`      | Log level: debug, info, warn, error, fatal |
| `format`         | string   | `"console"`   | Output format: json, console              |
| `outputs`        | []string | `["console"]` | Output targets: console, file             |
| `log_path`       | string   | `"./logs"`    | Log file directory                        |
| `file_name`      | string   | `""`          | Log file name (uses date if empty)        |
| `add_caller`     | bool     | `true`        | Show caller location                      |
| `add_stacktrace` | bool     | `false`       | Show stack trace                          |
| `development`    | bool     | `false`       | Development mode                          |
| `color_enabled`  | bool     | `true`        | Enable colored output (console only)      |

### Color Output

When `color_enabled` is `true` and `format` is `console`, different log levels are displayed in different colors:

| Level       | Color   |
| ----------- | ------- |
| DEBUG       | Magenta |
| INFO        | Blue    |
| WARN        | Yellow  |
| ERROR/FATAL | Red     |

```go
// Enable colors (default)
zlogger.Init(nil)

// Disable colors (recommended for files or CI environments)
zlogger.Init(&zlogger.Config{
    ColorEnabled: false,
})
```

> **Note:** For log rotation (file size limits, backups, compression), use timberjack as shown above.

## License

MIT
