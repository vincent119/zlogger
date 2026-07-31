# Configuration

[繁體中文](../zh-TW/configuration.md) | **English**

[Documentation index](README.md)

## Configuration Model

New applications should use `ConfigPatch`. Every field is a pointer: nil means omitted, while a
non-nil pointer is an explicit override, including `false`, an empty string, or an empty slice.
`Resolve` fills omitted fields from `DefaultConfig`, copies `outputs`, normalizes level, format, and
outputs to lowercase, and then validates the result.

`Config` represents a complete runtime configuration. `Config.Merge` is deprecated because bool
fields cannot distinguish an omitted value from `false`.

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

## Configuration Source Responsibility

zlogger does not read YAML, JSON, TOML, or environment variables and does not define source
precedence. The caller decodes external data into `ConfigPatch`; zlogger only resolves, normalizes,
validates, and initializes the logger.

`ConfigPatch` provides `json`, `yaml`, `toml`, and `mapstructure` tags, but no built-in loader.
Unknown keys must be rejected by the decoder's strict mode. Once ignored, `Validate` cannot recover
them.

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

This standard-library JSON loader limits input size and rejects unknown fields:

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

## Field Contract

| Key | Type | Default | Allowed values or conditions |
| --- | --- | --- | --- |
| `level` | string | `info` | `debug`, `info`, `warn`, `error`, `fatal`; case-insensitive |
| `format` | string | `console` | `console`, `json`; case-insensitive |
| `outputs` | []string | `[console]` | `console`, `file`; at least one and unique |
| `log_path` | string | `./logs` | Non-empty when `file` is enabled |
| `file_name` | string | empty | Safe leaf name; empty uses the date |
| `add_caller` | bool | `true` | Add caller information |
| `add_stacktrace` | bool | `false` | Add stack traces at ERROR and above |
| `development` | bool | `false` | zap development mode |
| `color_enabled` | bool | `true` | Emit ANSI colors only for console format |

Invalid values satisfy `errors.Is(err, zlogger.ErrInvalidConfig)`. An unsafe `file_name` with file
output retains both `ErrInvalidConfig` and `ErrUnsafeLogPath`. Decoder and file I/O errors are not
wrapped as `ErrInvalidConfig`.

## Color Contract

ANSI colors are emitted only when `format=console` and `color_enabled=true`. JSON levels never
contain ANSI, even when `color_enabled` remains true.

File creation permissions are not configuration-file fields. See [Security](security.md) and
`ConfigureWithOptions`.
