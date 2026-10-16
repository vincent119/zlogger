package zlogger_test

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vincent119/zlogger"
)

type helperProfile struct{ name string }

func (p helperProfile) MarshalLogObject(encoder zlogger.ObjectEncoder) error {
	encoder.AddString("name", p.name)
	return encoder.AddArray("codes", helperCodes{2, 1})
}

type helperCodes []int

func (codes helperCodes) MarshalLogArray(encoder zlogger.ArrayEncoder) error {
	for _, code := range codes {
		encoder.AppendInt(code)
	}
	return nil
}

var (
	_ zlogger.ObjectMarshaler = helperProfile{}
	_ zlogger.ArrayMarshaler  = helperCodes{}
)

func TestStructuredFieldHelpersExternalAPI(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "fields_helpers_external_test.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imported := range file.Imports {
		if strings.Contains(imported.Path.Value, "go.uber.org/zap") {
			t.Fatalf("external example must not import zap: %s", imported.Path.Value)
		}
	}
	cfg := zlogger.DefaultConfig()
	cfg.Format = "json"
	cfg.Outputs = []string{"file"}
	cfg.LogPath = t.TempDir()
	cfg.FileName = "fields.log"
	cfg.AddCaller = false
	instance, err := zlogger.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := instance.Close(); err != nil {
			t.Error(err)
		}
	})
	instance.Logger().Info("external fields",
		zlogger.Object("profile", helperProfile{"reader"}),
		zlogger.Array("items", helperCodes{3, 1}),
		zlogger.Ints("attempts", []int{1, 2}),
		zlogger.Inline(helperProfile{"inline"}),
		zlogger.Namespace("request"),
		zlogger.String("id", "r1"),
	)
	if err := instance.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := instance.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(cfg.LogPath, cfg.FileName))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected initialization and application entries, got %d", len(lines))
	}
	var entry struct {
		Message string `json:"msg"`
		Profile struct {
			Name  string `json:"name"`
			Codes []int  `json:"codes"`
		} `json:"profile"`
		Codes   []int  `json:"items"`
		Name    string `json:"name"`
		Request struct {
			ID string `json:"id"`
		} `json:"request"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Message != "external fields" || entry.Profile.Name != "reader" || entry.Name != "inline" || entry.Request.ID != "r1" {
		t.Fatalf("unexpected structured fields: %+v", entry)
	}
	var slices struct {
		Attempts []int `json:"attempts"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &slices); err != nil {
		t.Fatal(err)
	}
	if len(slices.Attempts) != 2 || slices.Attempts[0] != 1 || slices.Attempts[1] != 2 {
		t.Fatalf("unexpected typed slice: %v", slices.Attempts)
	}
	if len(entry.Profile.Codes) != 2 || entry.Profile.Codes[0] != 2 || entry.Profile.Codes[1] != 1 || len(entry.Codes) != 2 || entry.Codes[0] != 3 || entry.Codes[1] != 1 {
		t.Fatalf("array order changed: %+v", entry)
	}
}
