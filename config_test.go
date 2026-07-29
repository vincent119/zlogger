package zlogger

import (
	"errors"
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

func TestConfigPatchResolvePreservesDefaults(t *testing.T) {
	level := "DEBUG"
	patch := &ConfigPatch{Level: &level}

	cfg, err := patch.Resolve()
	if err != nil {
		t.Fatalf("解析部分設定失敗：%v", err)
	}

	if cfg.Level != "debug" {
		t.Errorf("Level = %q，預期為 debug", cfg.Level)
	}
	if !cfg.AddCaller {
		t.Error("未提供 AddCaller 時應保留預設 true")
	}
	if !cfg.ColorEnabled {
		t.Error("未提供 ColorEnabled 時應保留預設 true")
	}
}

func TestConfigPatchResolveExplicitFalse(t *testing.T) {
	disabled := false
	patch := &ConfigPatch{
		AddCaller:    &disabled,
		ColorEnabled: &disabled,
	}

	cfg, err := patch.Resolve()
	if err != nil {
		t.Fatalf("解析部分設定失敗：%v", err)
	}

	if cfg.AddCaller {
		t.Error("明確提供 AddCaller=false 時不應保留預設值")
	}
	if cfg.ColorEnabled {
		t.Error("明確提供 ColorEnabled=false 時不應保留預設值")
	}
}

func TestConfigPatchResolveCopiesOutputs(t *testing.T) {
	outputs := []string{"CONSOLE", "FILE"}
	patch := &ConfigPatch{Outputs: &outputs}

	cfg, err := patch.Resolve()
	if err != nil {
		t.Fatalf("解析部分設定失敗：%v", err)
	}

	outputs[0] = "changed-source"
	if cfg.Outputs[0] != "console" {
		t.Fatalf("結果與來源共享 slice：%v", cfg.Outputs)
	}

	cfg.Outputs[1] = "changed-result"
	if (*patch.Outputs)[1] != "FILE" {
		t.Fatalf("來源與結果共享 slice：%v", *patch.Outputs)
	}
}

func TestConfigPatchResolveUsesDefaultLogPathWhenOmitted(t *testing.T) {
	outputs := []string{"file"}
	patch := &ConfigPatch{Outputs: &outputs}

	cfg, err := patch.Resolve()
	if err != nil {
		t.Fatalf("解析部分設定失敗：%v", err)
	}
	if cfg.LogPath != "./logs" {
		t.Errorf("LogPath = %q，預期為 ./logs", cfg.LogPath)
	}
}

func TestConfigValidateRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "未知 Level",
			cfg:  &Config{Level: "trace", Format: "console", Outputs: []string{"console"}, LogPath: "./logs"},
		},
		{
			name: "未知 Format",
			cfg:  &Config{Level: "info", Format: "xml", Outputs: []string{"console"}, LogPath: "./logs"},
		},
		{
			name: "空 Outputs",
			cfg:  &Config{Level: "info", Format: "console", Outputs: []string{}, LogPath: "./logs"},
		},
		{
			name: "未知 Output",
			cfg:  &Config{Level: "info", Format: "console", Outputs: []string{"stderr"}, LogPath: "./logs"},
		},
		{
			name: "重複 Output",
			cfg:  &Config{Level: "info", Format: "console", Outputs: []string{"console", "console"}, LogPath: "./logs"},
		},
		{
			name: "file 的 LogPath 空白",
			cfg:  &Config{Level: "info", Format: "console", Outputs: []string{"file"}, LogPath: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("錯誤 = %v，預期可由 errors.Is 判斷為 ErrInvalidConfig", err)
			}
		})
	}
}

func TestConfigPatchResolveRejectsExplicitEmptyLogPath(t *testing.T) {
	outputs := []string{"file"}
	logPath := ""
	patch := &ConfigPatch{Outputs: &outputs, LogPath: &logPath}

	_, err := patch.Resolve()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("錯誤 = %v，預期可由 errors.Is 判斷為 ErrInvalidConfig", err)
	}
}

func TestConfigValidateRejectsUnsafeFileName(t *testing.T) {
	unsafeNames := []string{
		"../outside.log",
		"/tmp/outside.log",
		"sub/app.log",
		`sub\app.log`,
		".",
		"..",
		"app\x00.log",
		`C:app.log`,
		`C:\logs\app.log`,
	}

	for _, name := range unsafeNames {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{
				Level:    "info",
				Format:   "json",
				Outputs:  []string{"file"},
				LogPath:  t.TempDir(),
				FileName: name,
			}
			err := cfg.Validate()
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("錯誤 = %v，預期 ErrInvalidConfig", err)
			}
			if !errors.Is(err, ErrUnsafeLogPath) {
				t.Fatalf("錯誤 = %v，預期 ErrUnsafeLogPath", err)
			}
		})
	}

	consoleOnly := &Config{
		Level:    "info",
		Format:   "json",
		Outputs:  []string{"console"},
		LogPath:  "./logs",
		FileName: "../unused.log",
	}
	if err := consoleOnly.Validate(); err != nil {
		t.Fatalf("console-only Config 不應驗證未使用的 FileName：%v", err)
	}
}
