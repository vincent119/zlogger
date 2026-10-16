package zlogger

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrInvalidConfig indicates that logger settings violate the public contract.
var ErrInvalidConfig = errors.New("invalid logger configuration")

// Config contains the complete runtime logger settings.
//
// Existing field types are retained for source compatibility. To distinguish omitted
// values from explicit zero values, resolve a ConfigPatch instead.
type Config struct {
	Level         string   `json:"level" yaml:"level" toml:"level" mapstructure:"level"`
	Format        string   `json:"format" yaml:"format" toml:"format" mapstructure:"format"`
	Outputs       []string `json:"outputs" yaml:"outputs" toml:"outputs" mapstructure:"outputs"`
	LogPath       string   `json:"log_path" yaml:"log_path" toml:"log_path" mapstructure:"log_path"`
	FileName      string   `json:"file_name" yaml:"file_name" toml:"file_name" mapstructure:"file_name"`
	AddCaller     bool     `json:"add_caller" yaml:"add_caller" toml:"add_caller" mapstructure:"add_caller"`
	AddStacktrace bool     `json:"add_stacktrace" yaml:"add_stacktrace" toml:"add_stacktrace" mapstructure:"add_stacktrace"`
	Development   bool     `json:"development" yaml:"development" toml:"development" mapstructure:"development"`
	ColorEnabled  bool     `json:"color_enabled" yaml:"color_enabled" toml:"color_enabled" mapstructure:"color_enabled"`
}

// ConfigPatch distinguishes omitted settings from explicit zero values.
type ConfigPatch struct {
	Level         *string   `json:"level,omitempty" yaml:"level,omitempty" toml:"level,omitempty" mapstructure:"level"`
	Format        *string   `json:"format,omitempty" yaml:"format,omitempty" toml:"format,omitempty" mapstructure:"format"`
	Outputs       *[]string `json:"outputs,omitempty" yaml:"outputs,omitempty" toml:"outputs,omitempty" mapstructure:"outputs"`
	LogPath       *string   `json:"log_path,omitempty" yaml:"log_path,omitempty" toml:"log_path,omitempty" mapstructure:"log_path"`
	FileName      *string   `json:"file_name,omitempty" yaml:"file_name,omitempty" toml:"file_name,omitempty" mapstructure:"file_name"`
	AddCaller     *bool     `json:"add_caller,omitempty" yaml:"add_caller,omitempty" toml:"add_caller,omitempty" mapstructure:"add_caller"`
	AddStacktrace *bool     `json:"add_stacktrace,omitempty" yaml:"add_stacktrace,omitempty" toml:"add_stacktrace,omitempty" mapstructure:"add_stacktrace"`
	Development   *bool     `json:"development,omitempty" yaml:"development,omitempty" toml:"development,omitempty" mapstructure:"development"`
	ColorEnabled  *bool     `json:"color_enabled,omitempty" yaml:"color_enabled,omitempty" toml:"color_enabled,omitempty" mapstructure:"color_enabled"`
}

// DefaultConfig returns a complete, usable default configuration.
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

// Resolve applies a patch to defaults and returns an independent, validated configuration.
func (p *ConfigPatch) Resolve() (*Config, error) {
	cfg := DefaultConfig()
	if p == nil {
		return cfg, nil
	}

	if p.Level != nil {
		cfg.Level = *p.Level
	}
	if p.Format != nil {
		cfg.Format = *p.Format
	}
	if p.Outputs != nil {
		cfg.Outputs = slices.Clone(*p.Outputs)
	}
	if p.LogPath != nil {
		cfg.LogPath = *p.LogPath
	}
	if p.FileName != nil {
		cfg.FileName = *p.FileName
	}
	if p.AddCaller != nil {
		cfg.AddCaller = *p.AddCaller
	}
	if p.AddStacktrace != nil {
		cfg.AddStacktrace = *p.AddStacktrace
	}
	if p.Development != nil {
		cfg.Development = *p.Development
	}
	if p.ColorEnabled != nil {
		cfg.ColorEnabled = *p.ColorEnabled
	}

	cfg = cfg.normalizedCopy()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks a complete configuration without modifying the caller's object.
// File output requires FileName to be a safe leaf name.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("%w: Config must not be nil", ErrInvalidConfig)
	}

	switch strings.ToLower(c.Level) {
	case "debug", "info", "warn", "error", "fatal":
	default:
		return fmt.Errorf("%w: unsupported Level %q", ErrInvalidConfig, c.Level)
	}

	switch strings.ToLower(c.Format) {
	case "console", "json":
	default:
		return fmt.Errorf("%w: unsupported Format %q", ErrInvalidConfig, c.Format)
	}

	if len(c.Outputs) == 0 {
		return fmt.Errorf("%w: Outputs must not be empty", ErrInvalidConfig)
	}

	seen := make(map[string]struct{}, len(c.Outputs))
	fileEnabled := false
	for _, output := range c.Outputs {
		normalized := strings.ToLower(output)
		switch normalized {
		case "console":
		case "file":
			fileEnabled = true
		default:
			return fmt.Errorf("%w: unsupported Output %q", ErrInvalidConfig, output)
		}
		if _, exists := seen[normalized]; exists {
			return fmt.Errorf("%w: duplicate Output %q", ErrInvalidConfig, output)
		}
		seen[normalized] = struct{}{}
	}

	if fileEnabled && c.LogPath == "" {
		return fmt.Errorf("%w: file output requires a nonempty LogPath", ErrInvalidConfig)
	}
	if fileEnabled {
		if err := validateLogLeaf(c.FileName, true); err != nil {
			return fmt.Errorf("%w: FileName: %w", ErrInvalidConfig, err)
		}
	}

	return nil
}

// Merge retains legacy partial-merge behavior for source compatibility.
//
// Deprecated: Config booleans cannot distinguish omitted values from explicit false. Use
// ConfigPatch.Resolve。
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
		c.Outputs = slices.Clone(other.Outputs)
	}
	if other.LogPath != "" {
		c.LogPath = other.LogPath
	}
	if other.FileName != "" {
		c.FileName = other.FileName
	}
	c.AddCaller = other.AddCaller
	c.AddStacktrace = other.AddStacktrace
	c.Development = other.Development
	c.ColorEnabled = other.ColorEnabled

	return c
}

func (c *Config) normalizedCopy() *Config {
	if c == nil {
		return nil
	}

	cloned := *c
	cloned.Level = strings.ToLower(c.Level)
	cloned.Format = strings.ToLower(c.Format)
	cloned.Outputs = slices.Clone(c.Outputs)
	for i := range cloned.Outputs {
		cloned.Outputs[i] = strings.ToLower(cloned.Outputs[i])
	}

	return &cloned
}
