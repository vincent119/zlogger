package zlogger

// Config 日誌配置結構
// 可從外部應用程式的設定檔（如 YAML、JSON、TOML）直接綁定
type Config struct {
	Level         string   `json:"level" yaml:"level" toml:"level"`                            // 日誌級別: debug, info, warn, error, fatal
	Format        string   `json:"format" yaml:"format" toml:"format"`                         // 輸出格式: json 或 console
	Outputs       []string `json:"outputs" yaml:"outputs" toml:"outputs"`                      // 輸出目標: console, file
	LogPath       string   `json:"log_path" yaml:"log_path" toml:"log_path"`                   // 日誌檔案路徑
	FileName      string   `json:"file_name" yaml:"file_name" toml:"file_name"`                // 日誌檔案名稱
	MaxSize       int      `json:"max_size" yaml:"max_size" toml:"max_size"`                   // 單個日誌檔案最大大小（MB）
	MaxAge        int      `json:"max_age" yaml:"max_age" toml:"max_age"`                      // 日誌檔案保存天數
	MaxBackups    int      `json:"max_backups" yaml:"max_backups" toml:"max_backups"`          // 最大備份數
	Compress      bool     `json:"compress" yaml:"compress" toml:"compress"`                   // 是否壓縮舊日誌
	AddCaller     bool     `json:"add_caller" yaml:"add_caller" toml:"add_caller"`             // 是否添加調用者信息
	AddStacktrace bool     `json:"add_stacktrace" yaml:"add_stacktrace" toml:"add_stacktrace"` // 是否添加堆疊追蹤
	Development   bool     `json:"development" yaml:"development" toml:"development"`          // 是否為開發模式
}

// DefaultConfig 返回預設配置
func DefaultConfig() *Config {
	return &Config{
		Level:         "info",
		Format:        "console",
		Outputs:       []string{"console"},
		LogPath:       "./logs",
		MaxSize:       100,
		MaxAge:        30,
		MaxBackups:    10,
		Compress:      true,
		AddCaller:     true,
		AddStacktrace: false,
		Development:   false,
	}
}

// Merge 將傳入的配置合併到預設配置（零值不覆蓋）
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
	if other.MaxSize > 0 {
		c.MaxSize = other.MaxSize
	}
	if other.MaxAge > 0 {
		c.MaxAge = other.MaxAge
	}
	if other.MaxBackups > 0 {
		c.MaxBackups = other.MaxBackups
	}
	// bool 類型直接覆蓋
	c.Compress = other.Compress
	c.AddCaller = other.AddCaller
	c.AddStacktrace = other.AddStacktrace
	c.Development = other.Development

	return c
}
