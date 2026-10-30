# Configuration

[繁體中文（台灣）](../zh-TW/configuration.md) | **English**

[Documentation index](README.md)

## Use ConfigPatch for partial overrides

Start with `ConfigPatch`. Each field is a pointer.
Nil means omitted. A non-nil pointer supplies the value, including false, an empty string,
or an empty slice, so you can turn a feature off without accidentally asking for its default.

`Resolve` fills missing values from `DefaultConfig`, copies outputs and redaction.keys, lowercases level,
format, and outputs, then validates the result.

`Config` holds a complete configuration. Start with `DefaultConfig()` and edit what you need.
The older `Config.Merge` is deprecated because its bool fields can't distinguish omission from false.

This fragment selects debug-level JSON and enables console and file output.
Once you have cleanup, call Sync before cleanup during shutdown. See [lifecycle](lifecycle.md).

```go
level := "debug"
format := "json"
outputs := []string{"console", "file"}
colorEnabled := false

cleanup, err := zlogger.Configure(&zlogger.ConfigPatch{
	Level:        &level,
	Format:       &format,
	Outputs:      &outputs,
	ColorEnabled: &colorEnabled,
})
```

## Load configuration in your application

Load settings in your application. Choose the source order there too.
Then pass the decoded `ConfigPatch` to zlogger.
It fills defaults, validates settings, and builds the logger.

`ConfigPatch` has json, yaml, toml, and mapstructure tags for your chosen decoder.
Enable strict decoding to reject unknown keys. Validate can't recover a typo that the decoder
has already discarded.

```yaml
log:
  level: debug
  format: json
  outputs: [console, file]
  log_path: ./logs
  file_name: app.log
  add_caller: true
  add_stacktrace: false
  development: false
  color_enabled: false
```

This standard-library JSON fragment reads at most 1 MiB and rejects unknown fields.
It doesn't fully validate file size or trailing data, so add those checks if your input requires them.
Place it inside a function returning error and import encoding/json, fmt, io, os, and zlogger.

```go
type AppConfig struct {
	Log zlogger.ConfigPatch `json:"log"`
}

file, err := os.Open("config.json")
if err != nil {
	return fmt.Errorf("open configuration file: %w", err)
}
defer func() { _ = file.Close() }()

var appConfig AppConfig
decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
decoder.DisallowUnknownFields()
if err := decoder.Decode(&appConfig); err != nil {
	return fmt.Errorf("decode configuration file: %w", err)
}

cleanup, err := zlogger.Configure(&appConfig.Log)
if err != nil {
	return fmt.Errorf("configure logger: %w", err)
}
defer func() { _ = cleanup() }()
```

## Defaults and accepted values

| Key | Type | Default | Allowed values or conditions |
| --- | --- | --- | --- |
| `level` | string | `info` | `debug`, `info`, `warn`, `error`, `fatal` (case-insensitive) |
| `format` | string | `console` | `console`, `json` (case-insensitive) |
| `outputs` | []string | `[console]` | One or both of `console`, `file`, with no duplicates |
| `log_path` | string | `./logs` | Non-empty when `file` is enabled |
| `file_name` | string | empty | Single safe filename. Empty uses the date |
| `add_caller` | bool | `true` | Add caller information |
| `add_stacktrace` | bool | `false` | Add stack traces at ERROR and above |
| `development` | bool | `false` | zap development mode |
| `color_enabled` | bool | `true` | Emit ANSI colors only for console format |
| `redaction.enabled` | bool | `false` | Enable structured-field redaction at construction |
| `redaction.keys` | []string | nil | When enabled, 1 to 128 keys of 1 to 128 bytes each; ignored when disabled |

Check invalid settings with `errors.Is(err, zlogger.ErrInvalidConfig)`.
With file output enabled, an unsafe file_name also preserves `ErrUnsafeLogPath`.
Decoder failures and file I/O errors keep their own error types.

## Colors affect console format only

ANSI colors are emitted only when `format=console` and `color_enabled=true`. JSON levels never
contain ANSI, even when `color_enabled` remains true.

File creation permissions are not configuration-file fields. See [Security](security.md) and
`ConfigureWithOptions`.

## Redaction settings

`Config.Redaction` holds a `RedactionConfig`; `ConfigPatch.Redaction` is a pointer to one.
Nil keeps the default disabled policy. A non-nil pointer replaces the whole policy;
keys aren't merged individually.

```go
patch := &zlogger.ConfigPatch{
	Redaction: &zlogger.RedactionConfig{
		Enabled: true,
		Keys:    []string{"token", "password"},
	},
}
```

Use this patch with `Configure`, `ConfigureWithOptions`, or `ConfigureSplit`.
For `New`, `NewWithOptions`, and `NewSplit`, set `cfg.Redaction` instead.
Invalid enabled policies return `ErrInvalidConfig` before opening files or starting workers.
Later edits to the original Enabled or Keys don't affect a constructed logger.

Legacy `Config.Merge` replaces the whole Redaction policy for a non-nil other, including
Enabled=false, and copies Keys. It can't distinguish omission from an explicit disable;
prefer ConfigPatch. New struct fields require callers using unkeyed Config or ConfigPatch
literals to switch to named fields.

`ConfigureCore` doesn't apply redaction to an external core and rejects Enabled=true with
`ErrInvalidConfig`. Build the core with `NewRedactingCore` first, then pass a nil or disabled
high-level policy to ConfigureCore. See [security](security.md) for matching, opaque values,
and free-text limits.
