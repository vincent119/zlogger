package zlogger

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrInvalidConfig 表示 logger 設定不符合公開契約。
var ErrInvalidConfig = errors.New("logger 設定無效")

// Config 是 logger 完整執行期設定。
//
// Config 保留既有欄位型別以維持來源碼相容。需要表達「未提供」與明確零值時，
// 應改用 ConfigPatch.Resolve 建立完整設定。
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

// ConfigPatch 表示可區分未提供與明確零值的部分設定。
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

// DefaultConfig 回傳可直接使用的完整預設設定。
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

// Resolve 將部分設定套用至預設值，並回傳獨立且已驗證的完整設定。
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

// Validate 檢查完整設定，不修改呼叫端提供的物件。
// file output 的 FileName 必須是安全 leaf name。
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("%w: Config 不可為 nil", ErrInvalidConfig)
	}

	switch strings.ToLower(c.Level) {
	case "debug", "info", "warn", "error", "fatal":
	default:
		return fmt.Errorf("%w: Level %q 不受支援", ErrInvalidConfig, c.Level)
	}

	switch strings.ToLower(c.Format) {
	case "console", "json":
	default:
		return fmt.Errorf("%w: Format %q 不受支援", ErrInvalidConfig, c.Format)
	}

	if len(c.Outputs) == 0 {
		return fmt.Errorf("%w: Outputs 不可為空", ErrInvalidConfig)
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
			return fmt.Errorf("%w: Output %q 不受支援", ErrInvalidConfig, output)
		}
		if _, exists := seen[normalized]; exists {
			return fmt.Errorf("%w: Output %q 重複", ErrInvalidConfig, output)
		}
		seen[normalized] = struct{}{}
	}

	if fileEnabled && c.LogPath == "" {
		return fmt.Errorf("%w: file output 的 LogPath 不可為空", ErrInvalidConfig)
	}
	if fileEnabled {
		if err := validateLogLeaf(c.FileName, true); err != nil {
			return fmt.Errorf("%w: FileName: %w", ErrInvalidConfig, err)
		}
	}

	return nil
}

// Merge 保留既有的部分合併行為以維持來源碼相容。
//
// Deprecated: Config 的 bool 無法區分未提供與明確 false，新程式應使用
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

	copy := *c
	copy.Level = strings.ToLower(c.Level)
	copy.Format = strings.ToLower(c.Format)
	copy.Outputs = slices.Clone(c.Outputs)
	for i := range copy.Outputs {
		copy.Outputs[i] = strings.ToLower(copy.Outputs[i])
	}

	return &copy
}
