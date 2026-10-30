package zlogger_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vincent119/zlogger"
)

func ExampleNew_redaction() {
	directory, err := os.MkdirTemp("", "zlogger-redaction-")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(directory) }()
	cfg := zlogger.DefaultConfig()
	cfg.Format = "json"
	cfg.Outputs, cfg.LogPath, cfg.FileName = []string{"file"}, directory, "app.log"
	cfg.Redaction = zlogger.RedactionConfig{Enabled: true, Keys: []string{"token"}}
	instance, err := zlogger.New(cfg)
	if err != nil {
		panic(err)
	}
	instance.Logger().Info("authentication request", zlogger.String("token", "example value"))
	if err := errors.Join(instance.Sync(), instance.Close()); err != nil {
		panic(err)
	}
	// The path is confined to the temporary example directory.
	data, err := os.ReadFile(filepath.Join(directory, "app.log")) //nolint:gosec
	if err != nil {
		panic(err)
	}
	fmt.Println(strings.Contains(string(data), `"token":"[REDACTED]"`))
	// Output: true
}

func ExampleConfigure_redaction() {
	directory, err := os.MkdirTemp("", "zlogger-redaction-")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(directory) }()
	format, fileName, outputs := "json", "app.log", []string{"file"}
	cleanup, err := zlogger.Configure(&zlogger.ConfigPatch{
		Format: &format, Outputs: &outputs, LogPath: &directory, FileName: &fileName,
		Redaction: &zlogger.RedactionConfig{Enabled: true, Keys: []string{"token"}},
	})
	if err != nil {
		panic(err)
	}
	defer func() { _ = cleanup() }()
	zlogger.Info("authentication request", zlogger.String("token", "example value"))
	if err := errors.Join(zlogger.Sync(), cleanup()); err != nil {
		panic(err)
	}
	// The path is confined to the temporary example directory.
	data, err := os.ReadFile(filepath.Join(directory, "app.log")) //nolint:gosec
	if err != nil {
		panic(err)
	}
	fmt.Println(strings.Contains(string(data), `"token":"[REDACTED]"`))
	// Output: true
}

func ExampleNewSplit_redaction() {
	directory, err := os.MkdirTemp("", "zlogger-redaction-")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(directory) }()
	cfg := zlogger.DefaultConfig()
	cfg.Format, cfg.LogPath, cfg.FileName = "json", directory, "app"
	cfg.Redaction = zlogger.RedactionConfig{Enabled: true, Keys: []string{"token"}}
	instance, err := zlogger.NewSplit(cfg)
	if err != nil {
		panic(err)
	}
	defer func() { _ = instance.Close() }()
	instance.Logger().Warn("authentication request", zlogger.String("token", "example value"))
	if err := instance.Close(); err != nil {
		panic(err)
	}
	files, err := filepath.Glob(filepath.Join(directory, "app-warn-*.log"))
	if err != nil || len(files) != 1 {
		panic("expected one warn file")
	}
	// Glob selects a fixed log pattern inside the temporary example directory.
	data, err := os.ReadFile(files[0]) //nolint:gosec
	if err != nil {
		panic(err)
	}
	fmt.Println(strings.Contains(string(data), `"token":"[REDACTED]"`))
	// Output: true
}
