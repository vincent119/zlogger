# Using zlogger

[繁體中文（台灣）](../zh-TW/README.md) | **English**

[Project home](../../README.en.md) · [GoDoc](https://pkg.go.dev/github.com/vincent119/zlogger)

Start with [configuration](configuration.md) and [lifecycle](lifecycle.md).
For a first log line, use the example on the project home page.

| What you're working on | Guide |
| --- | --- |
| Choosing globals or an independent logger, and deciding who closes it | [Lifecycle](lifecycle.md) |
| Defaults, configuration loading, and console colors | [Configuration](configuration.md) |
| Daily files, custom sinks, and output failures | [Output modes](output-modes.md) |
| Your own field names, context, and duplicate keys | [Context and fields](context-and-fields.md) |
| Redaction, paths, and file permissions | [Security](security.md) |
| HTTP request logs in your application | [Gin integration](integrations/gin.md) |
| Size-based rotation, retention, and compression | [timberjack integration](integrations/timberjack.md) |

These guides follow the current branch.
Gin and timberjack examples belong in your application, so using zlogger alone doesn't add
those dependencies or decide your business field names.
