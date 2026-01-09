package zlogger

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Level != "info" {
		t.Errorf("expected Level 'info', got '%s'", cfg.Level)
	}
	if cfg.Format != "console" {
		t.Errorf("expected Format 'console', got '%s'", cfg.Format)
	}
	if len(cfg.Outputs) != 1 || cfg.Outputs[0] != "console" {
		t.Errorf("expected Outputs ['console'], got %v", cfg.Outputs)
	}
	if cfg.LogPath != "./logs" {
		t.Errorf("expected LogPath './logs', got '%s'", cfg.LogPath)
	}
	if !cfg.AddCaller {
		t.Error("expected AddCaller true, got false")
	}
	if cfg.AddStacktrace {
		t.Error("expected AddStacktrace false, got true")
	}
	if cfg.Development {
		t.Error("expected Development false, got true")
	}
}

func TestConfigMerge_NilOther(t *testing.T) {
	cfg := DefaultConfig()
	result := cfg.Merge(nil)

	if result != cfg {
		t.Error("Merge(nil) should return original config")
	}
}

func TestConfigMerge_OverrideLevel(t *testing.T) {
	cfg := DefaultConfig()
	other := &Config{Level: "debug"}

	result := cfg.Merge(other)

	if result.Level != "debug" {
		t.Errorf("expected Level 'debug', got '%s'", result.Level)
	}
	// Other values should remain default
	if result.Format != "console" {
		t.Errorf("expected Format 'console', got '%s'", result.Format)
	}
}

func TestConfigMerge_OverrideFormat(t *testing.T) {
	cfg := DefaultConfig()
	other := &Config{Format: "json"}

	result := cfg.Merge(other)

	if result.Format != "json" {
		t.Errorf("expected Format 'json', got '%s'", result.Format)
	}
}

func TestConfigMerge_OverrideOutputs(t *testing.T) {
	cfg := DefaultConfig()
	other := &Config{Outputs: []string{"console", "file"}}

	result := cfg.Merge(other)

	if len(result.Outputs) != 2 {
		t.Errorf("expected 2 outputs, got %d", len(result.Outputs))
	}
	if result.Outputs[0] != "console" || result.Outputs[1] != "file" {
		t.Errorf("expected ['console', 'file'], got %v", result.Outputs)
	}
}

func TestConfigMerge_EmptyStringNotOverride(t *testing.T) {
	cfg := DefaultConfig()
	other := &Config{Level: ""} // empty string should not override

	result := cfg.Merge(other)

	if result.Level != "info" {
		t.Errorf("empty string should not override, expected 'info', got '%s'", result.Level)
	}
}

func TestConfigMerge_EmptySliceNotOverride(t *testing.T) {
	cfg := DefaultConfig()
	other := &Config{Outputs: nil} // nil slice should not override

	result := cfg.Merge(other)

	if len(result.Outputs) != 1 || result.Outputs[0] != "console" {
		t.Errorf("nil slice should not override, expected ['console'], got %v", result.Outputs)
	}
}

func TestConfigMerge_BoolOverride(t *testing.T) {
	cfg := DefaultConfig()
	// AddCaller defaults to true, test override to false
	other := &Config{AddCaller: false}

	result := cfg.Merge(other)

	if result.AddCaller {
		t.Error("bool should override, expected AddCaller false")
	}
}

func TestConfigMerge_AllFields(t *testing.T) {
	cfg := DefaultConfig()
	other := &Config{
		Level:         "error",
		Format:        "json",
		Outputs:       []string{"file"},
		LogPath:       "/var/log",
		FileName:      "app.log",
		AddCaller:     false,
		AddStacktrace: true,
		Development:   true,
	}

	result := cfg.Merge(other)

	if result.Level != "error" {
		t.Errorf("expected Level 'error', got '%s'", result.Level)
	}
	if result.Format != "json" {
		t.Errorf("expected Format 'json', got '%s'", result.Format)
	}
	if len(result.Outputs) != 1 || result.Outputs[0] != "file" {
		t.Errorf("expected Outputs ['file'], got %v", result.Outputs)
	}
	if result.LogPath != "/var/log" {
		t.Errorf("expected LogPath '/var/log', got '%s'", result.LogPath)
	}
	if result.FileName != "app.log" {
		t.Errorf("expected FileName 'app.log', got '%s'", result.FileName)
	}
	if result.AddCaller {
		t.Error("expected AddCaller false")
	}
	if !result.AddStacktrace {
		t.Error("expected AddStacktrace true")
	}
	if !result.Development {
		t.Error("expected Development true")
	}
}
