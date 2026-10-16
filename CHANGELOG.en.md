# Changelog

[繁體中文](CHANGELOG.md) | **English**

This document records user-visible changes in each zlogger release. Its format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

<!-- markdownlint-disable MD024 -->

## [Unreleased]

There are no recorded changes yet.

## [1.2.0] - 2026-10-16

### Added

- Added `GetSplitCoreWithCleanup` so daily three-file core cleanup returns close errors and supports repeated and concurrent calls.
- Added `ConfigureCore`, `WithCoreCleanup`, and `ConfigureSplit` to connect custom cores or daily three-file loggers to the global API with explicit synchronization, worker shutdown, and resource ownership.
- Added `NewSplit` and `Instance.SetLevel` so components can own separate daily three-file loggers and adjust their levels without changing global configuration.
- Added `NewFailureObserver`, `NewObservedWriteSyncer`, and `WithOutputObserver` to observe write, sync, rotation, and close failures. Notifications use a bounded nonblocking channel, with `Dropped()` counting events lost when full. Core encoding errors are not observed.
- Added `NewRedactingCore`, `RedactionConfig`, and `ErrRedactionFailed` for explicit construction-time enablement and configurable redaction Keys. Redaction defaults to disabled. When enabled, it handles nested fields, masks opaque values as a whole, and rejects entries that cannot be encoded safely.
- Added `Object`, `Array`, `Inline`, `Namespace`, 16 typed-slice helpers, and four type aliases: `ObjectMarshaler`, `ArrayMarshaler`, `ObjectEncoder`, and `ArrayEncoder`. There are now 47 field helpers. Applications can implement marshalers using only zlogger imports, while zap remains the underlying dependency.

### Changed

- Go comments, package-owned errors and diagnostics, and example output now use English. Caller messages, field values, and external errors retain their original text.
- Expanded both language guides with lifecycle, failure observation, redaction, and all 47 field helpers, including limitations and executable examples.

### Compatibility notes

- Fixed error messages and the daily rotation diagnostic prefix now use English. Use `errors.Is`/`errors.As` instead of matching complete strings, and check existing alerts and string-based tests. Existing error identities, wrapping chains, and API signatures remain unchanged.
- Redaction does not protect free-text messages. Regular and split logger constructors do not enable it automatically. Original I/O errors and failure event Err values may still contain sensitive information and should not be forwarded without review.
- New helpers retain zap's evaluation, nil, namespace, and duplicate-key behavior without deep-copying input. Custom core and encoder construction can still require direct zap imports.

The following existing fixed messages changed relative to `v1.1.0`:

| Previous fixed text | New fixed text |
| --- | --- |
| logger 設定無效 | `invalid logger configuration` |
| 全域 logger 已完成設定 | `global logger is already configured` |
| 日誌檔案路徑不安全 | `unsafe log file path` |
| 檔案輸出權限無效 | `invalid log file permissions` |
| 分級 core 設定無效 | `invalid split core configuration` |
| 每日換檔失敗： | `daily log rotation failed:` |

### Security

- Moved vulnerability scanning to Go 1.26.9 while retaining Go 1.25.12 compatibility tests. The minimum Go version remains 1.25. Updating this package does not upgrade the caller's toolchain.

## [1.1.0] - 2026-07-31

### Added

- Added `Configure`, `ConfigureWithOptions`, and `ConfigPatch` to separate initialization errors, defaults, and explicit zero values.
- Added `New`, `NewWithOptions`, and `Instance` for non-global loggers with explicit `Sync` and `Close` lifecycles.
- Added `NewSplitCore` and `SplitSinks` for caller-provided info, warn, and error sinks.
- Added `WithDirPerm` and `WithFilePerm` to customize new log directory and file permissions within enforced safety constraints.
- Added `Redacted` to record a sensitive field name with a fixed replacement value without accepting the original secret.
- Added Traditional Chinese and English documentation for configuration, lifecycles, output modes, security, Gin, and timberjack integration.

### Changed

- Raised the minimum Go version to Go 1.25; CI verifies Go 1.25.12 and Go 1.26.5.
- Updated zap to v1.28.0.
- Configuration values are normalized and strictly validated; unknown levels, formats, outputs, and duplicate outputs no longer silently fall back.
- Tightened default permissions for new log directories and files to `0700` and `0600` without changing existing object permissions.
- Limited `ColorEnabled` to the console encoder so JSON output no longer contains ANSI color sequences.
- Added pinned linting, race tests, vulnerability scanning, benchmarks, a 90% coverage gate, Codecov, and Dependabot to CI.

### Fixed

- Stopped the `SplitOutput` daily rotation goroutine during Close so it cannot reopen files after shutdown.
- Made `SplitOutput.Close` and `Instance.Close` safe for repeated and concurrent calls, with aggregated resource cleanup errors.
- Made `SplitOutput.Write` and `SplitOutput.Sync` return an `os.ErrClosed` error detectable with `errors.Is` after shutdown.
- Fixed DEBUG routing so debug records reach the info sink.
- Fixed incomplete file ownership, reverse-order cleanup on failures, and unclosed test files on Windows.
- Copied field slices at `WithContext` and `FromContext` boundaries to prevent callers from mutating context data through aliases.
- Removed the unused SQL processing core that could rewrite original messages and aligned encoder helpers with their actual contracts.

### Security

- Restricted `file_name` and `filePrefix` to a single leaf name, rejecting path separators, NUL, absolute paths, and Windows drive prefixes.
- Used `os.Root` to contain file resolution within the caller-selected base directory and rejected existing symlink leaves.
- Added `ErrUnsafeLogPath`, `ErrInvalidFilePermission`, `ErrInvalidConfig`, and other classifiable sentinel errors.
- Pinned GitHub Actions to full commit SHAs and added a fixed `govulncheck` version with dependency security automation.

### Deprecated

- Retained `Init` for source compatibility; new code should use `Configure` to receive errors and a cleanup function.
- Retained `Config.Merge` for source compatibility; new code should use `ConfigPatch.Resolve` to distinguish omitted values.
- Retained `NewNoEscapeJSONEncoder` and `DisableHTMLEscaping` for source compatibility; new code should use zap encoder APIs directly.

### Compatibility Notes

- Projects using Go 1.24 or earlier must upgrade their toolchain.
- Configurations that previously supplied a subdirectory or full path through `file_name` or `filePrefix` are rejected; provide the directory through `log_path` or the constructor argument instead.
- `NewSplitCore` does not close external sinks; callers remain responsible for the final `Sync` and `Close` operations.

## [1.0.5] - 2026-01-09

### Changed

- Added an English README and standardized Go comments and formatting.

## [1.0.4] - 2026-01-09

### Added

- Added a Makefile, GitHub Actions CI, and project status badges.
- Added a test coverage reporting workflow.

### Changed

- Increased test coverage to 91.5%.

## [1.0.3] - 2025-12-11

### Added

- Configured the console encoder field separator for clearer plain-text log formatting.

## [1.0.2] - 2025-12-10

### Added

- Added `ColorEnabled` to control level colors in console logs.

<!-- markdownlint-enable MD024 -->

[Unreleased]: https://github.com/vincent119/zlogger/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/vincent119/zlogger/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/vincent119/zlogger/compare/v1.0.5...v1.1.0
[1.0.5]: https://github.com/vincent119/zlogger/compare/v1.0.4...v1.0.5
[1.0.4]: https://github.com/vincent119/zlogger/compare/v1.0.3...v1.0.4
[1.0.3]: https://github.com/vincent119/zlogger/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/vincent119/zlogger/releases/tag/v1.0.2
