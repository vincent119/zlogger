# zlogger Documentation

[繁體中文](../zh-TW/README.md) | **English**

[Project home](../../README.en.md) | [GoDoc](https://pkg.go.dev/github.com/vincent119/zlogger)

## Suggested Reading Order

1. [Configuration](configuration.md): sources, `ConfigPatch`, defaults, and validation.
2. [Lifecycle](lifecycle.md): globals, instances, cleanup, `Sync`, and `Close`.
3. [Output modes](output-modes.md): console, file, daily routing, and external sinks.
4. [Context and fields](context-and-fields.md): request-scoped fields and merge rules.
5. [Security](security.md): paths, permissions, `os.Root`, and sensitive data.
6. [Gin integration](integrations/gin.md): caller-owned HTTP middleware.
7. [timberjack integration](integrations/timberjack.md): size, retention, and compression rotation.

The root README provides onboarding and a capability summary. This directory contains the complete
usage contracts and advanced integration guides.
