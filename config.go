package zlogger

// Config 日誌配置結構
// 可從外部應用程式的設定檔（如 YAML、JSON、TOML）直接綁定
type Config struct {
	Level         string   `json:"level" yaml:"level" toml:"level"`                            // 日誌級別: debug, info, warn, error, fatal
	Format        string   `json:"format" yaml:"format" toml:"format"`                         // 輸出格式: json 或 console
	Outputs       []string `json:"outputs" yaml:"outputs" toml:"outputs"`                      // 輸出目標: console, file
	LogPath       string   `json:"log_path" yaml:"log_path" toml:"log_path"`                   // 日誌檔案路徑
	FileName      string   `json:"file_name" yaml:"file_name" toml:"file_name"`                // 日誌檔案名稱
	AddCaller     bool     `json:"add_caller" yaml:"add_caller" toml:"add_caller"`             // 是否添加調用者信息
	AddStacktrace bool     `json:"add_stacktrace" yaml:"add_stacktrace" toml:"add_stacktrace"` // 是否添加堆疊追蹤
	Development   bool     `json:"development" yaml:"development" toml:"development"`          // 是否為開發模式
	ColorEnabled  bool     `json:"color_enabled" yaml:"color_enabled" toml:"color_enabled"`    // 是否啟用顏色輸出（僅 console 格式有效）
}

// DefaultConfig 返回預設配置
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

// Merge 將傳入的配置合併到預設配置
//
// 合併規則：
//   - string: 空字串不覆蓋
//   - slice: nil/空切片不覆蓋
//   - bool: 直接覆蓋（無法區分「未設置」和「設置為 false」）
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
	// bool 類型直接覆蓋
	c.AddCaller = other.AddCaller
	c.AddStacktrace = other.AddStacktrace
	c.Development = other.Development
	c.ColorEnabled = other.ColorEnabled

	return c
}