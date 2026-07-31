# Security

[繁體中文](../zh-TW/security.md) | **English**

[Documentation index](README.md)

## Path Boundary

`log_path` and the `SplitOutput` directory are trusted base directories selected by the caller.
`file_name` and `filePrefix` must be one leaf name. They cannot be `.`, `..`, contain `/`, `\`,
NUL, be absolute or Windows drive-prefixed, or contain other path semantics.

Unsafe names satisfy `errors.Is(err, zlogger.ErrUnsafeLogPath)`. When Config validation is involved,
the error chain also retains `ErrInvalidConfig`.

## os.Root Containment

Each file batch opens the base directory with `os.OpenRoot`, then uses root-relative `Lstat` and
`OpenFile` calls. Stable final symlinks are rejected. If a leaf is replaced after checking,
`os.Root` still prevents resolution from escaping the root.

This mechanism is not a complete filesystem sandbox:

- `OpenRoot` follows a symlink in the base path itself.
- It does not prevent mount boundaries, bind mounts, special devices, or a malicious filesystem.
- A racing symlink that stays within the root may be followed.
- Go limits `js`, `plan9`, and `wasip1`; validated platforms are Linux, macOS, and Windows.

The caller must still protect the base directory and configuration sources.

## Creation Permissions

New directories and files use `0700` and `0600`. The umask may restrict them further, and existing
objects are never chmodded. To relax new-object permissions:

```go
instance, err := zlogger.NewWithOptions(
	cfg,
	zlogger.WithDirPerm(0o750),
	zlogger.WithFilePerm(0o640),
)
```

Directories must include owner `rwx`; files must include owner `rw`. Neither mode may contain
other-write or non-permission bits. Invalid values return `ErrInvalidFilePermission`. The last
option of the same kind wins.

Options affect only new objects, cannot bypass umask, and never change existing permissions.
Windows accepts the same API but does not guarantee observable POSIX mode semantics. Permissions
are intentionally not configuration-file fields, preventing untrusted external settings from
relaxing filesystem access.

## Sensitive Data

Use a field allowlist and record only the minimum diagnostic data. Do not log tokens, API keys,
passwords, private keys, Authorization headers, cookies, session identifiers, complete personal
data, or full request, response, Config, and arbitrary struct values that may contain secrets.

```go
zlogger.Info("authentication request",
	zlogger.Redacted("authorization"),
	zlogger.String("request_id", requestID),
)
```

`Redacted` writes only the fixed value `[REDACTED]`; it does not scan or mask other fields.
