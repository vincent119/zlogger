package zlogger

// Config is the logging configuration structure
// Can be directly bound from external application config files (YAML, JSON, TOML)
type Config struct {
	Level         string   `json:"level" yaml:"level" toml:"level"`                            // Log level: debug, info, warn, error, fatal
	Format        string   `json:"format" yaml:"format" toml:"format"`                         // Output format: json or console
	Outputs       []string `json:"outputs" yaml:"outputs" toml:"outputs"`                      // Output targets: console, file
	LogPath       string   `json:"log_path" yaml:"log_path" toml:"log_path"`                   // Log file path
	FileName      string   `json:"file_name" yaml:"file_name" toml:"file_name"`                // Log file name
	AddCaller     bool     `json:"add_caller" yaml:"add_caller" toml:"add_caller"`             // Whether to add caller info
	AddStacktrace bool     `json:"add_stacktrace" yaml:"add_stacktrace" toml:"add_stacktrace"` // Whether to add stack trace
	Development   bool     `json:"development" yaml:"development" toml:"development"`          // Whether in development mode
	ColorEnabled  bool     `json:"color_enabled" yaml:"color_enabled" toml:"color_enabled"`    // Whether to enable colored output (console format only)
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Level:         "info",
		Format:        "console",
		Outputs:       []string{"console"},
		LogPath:       "./logs",
		AddCaller:     true,
		AddStacktrace: false,
		Development:   false,
		ColorEnabled:  true,
	}
}

// Merge merges the provided configuration into the default configuration
//
// Merge rules:
//   - string: empty string does not override
//   - slice: nil/empty slice does not override
//   - bool: directly overrides (cannot distinguish between "not set" and "set to false")
func (c *Config) Merge(other *Config) *Config {
	if other == nil {
		return c
	}

	if other.Level != "" {
		c.Level = other.Level
	}
	if other.Format != "" {
		c.Format = other.Format
	}
	if len(other.Outputs) > 0 {
		c.Outputs = other.Outputs
	}
	if other.LogPath != "" {
		c.LogPath = other.LogPath
	}
	if other.FileName != "" {
		c.FileName = other.FileName
	}
	// bool types are directly overridden
	c.AddCaller = other.AddCaller
	c.AddStacktrace = other.AddStacktrace
	c.Development = other.Development
	c.ColorEnabled = other.ColorEnabled

	return c
}
