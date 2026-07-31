# Changelog

[繁體中文](CHANGELOG.md) | **English**

This document records user-visible changes in each zlogger release. Its format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

<!-- markdownlint-disable MD024 -->

## [Unreleased]

There are no recorded changes yet.

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

[Unreleased]: https://github.com/vincent119/zlogger/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/vincent119/zlogger/compare/v1.0.5...v1.1.0
[1.0.5]: https://github.com/vincent119/zlogger/compare/v1.0.4...v1.0.5
[1.0.4]: https://github.com/vincent119/zlogger/compare/v1.0.3...v1.0.4
[1.0.3]: https://github.com/vincent119/zlogger/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/vincent119/zlogger/releases/tag/v1.0.2
