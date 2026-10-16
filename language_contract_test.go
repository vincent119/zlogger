package zlogger

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func containsHan(text string) bool {
	return strings.ContainsFunc(text, func(r rune) bool { return unicode.Is(unicode.Han, r) })
}

// Unicode exceptions are scoped to the files that exercise them.
// Ordinary mock errors and log messages are not Unicode fixtures.
var unicodeTextFixtures = map[string]map[string]string{
	"benchmark_test.go": {
		"示範用值": "preserve benchmark input bytes",
		"效能測試": "preserve benchmark input bytes",
		"日誌訊息": "preserve benchmark input bytes",
	},
	"file_security_test.go": {
		"服務日誌.log": "exercise a Unicode filename",
	},
	"redaction_encoder_test.go": {
		"公開": "exercise Unicode scalar and byte encoding",
	},
	"redaction_test.go": {
		"公開":               "verify unmasked Unicode values",
		"測試秘密":             "verify Unicode secret fields never leak",
		"\"other\":\"公開\"": "assert the unmasked Unicode field",
		"結構測試秘密":           "fuzz nested Unicode secret redaction",
	},
	"language_contract_test.go": {
		"服務日誌.log": "preserve Unicode path in an error chain",
		"服務啟動 <>&": "preserve Unicode and HTML-like message bytes",
		"使用者資料":    "preserve Unicode field bytes",
	},
}

func TestEnglishCodeTextContract(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if containsHan(comment.Text) {
					t.Errorf("%s: comments must be in English", fset.Position(comment.Pos()))
				}
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			// The registry declares the reviewed values, not runtime display text.
			if declaration, ok := node.(*ast.ValueSpec); ok && name == "language_contract_test.go" && len(declaration.Names) == 1 && declaration.Names[0].Name == "unicodeTextFixtures" {
				return false
			}
			if call, ok := node.(*ast.CallExpr); ok && len(call.Args) > 0 {
				if method, ok := call.Fun.(*ast.SelectorExpr); ok {
					if receiver, ok := method.X.(*ast.Ident); ok && receiver.Name == "errors" && method.Sel.Name == "New" {
						if literal, ok := call.Args[0].(*ast.BasicLit); ok && literal.Kind == token.STRING {
							value, err := strconv.Unquote(literal.Value)
							if err != nil {
								t.Fatal(err)
							}
							if containsHan(value) {
								t.Errorf("%s: errors.New text must be in English, including mock errors", fset.Position(literal.Pos()))
							}
						}
					}
					if receiver, ok := method.X.(*ast.Ident); ok && (receiver.Name == "t" || receiver.Name == "b") {
						switch method.Sel.Name {
						case "Fatal", "Fatalf", "Error", "Errorf", "Log", "Logf", "Skip", "Skipf", "Run":
							if literal, ok := call.Args[0].(*ast.BasicLit); ok && literal.Kind == token.STRING {
								value, err := strconv.Unquote(literal.Value)
								if err != nil {
									t.Fatal(err)
								}
								if containsHan(value) {
									t.Errorf("%s: fixed test diagnostics must be in English", fset.Position(literal.Pos()))
								}
							}
						}
					}
				}
			}
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			if containsHan(value) {
				if !strings.HasSuffix(name, "_test.go") || unicodeTextFixtures[name][value] == "" {
					t.Errorf("%s: unclassified non-English text: %q", fset.Position(literal.Pos()), value)
				}
			}
			return true
		})
	}
}

func TestEnglishErrorMessages(t *testing.T) {
	for _, test := range []struct {
		err  error
		want string
	}{
		{ErrInvalidConfig, "invalid logger configuration"},
		{ErrAlreadyConfigured, "global logger is already configured"},
		{ErrUnsafeLogPath, "unsafe log file path"},
		{ErrInvalidFilePermission, "invalid log file permissions"},
		{ErrInvalidSplitCore, "invalid split core configuration"},
		{ErrRedactionFailed, "log redaction failed"},
	} {
		if got := test.err.Error(); got != test.want {
			t.Errorf("error = %q, want %q", got, test.want)
		}
	}
	_, err := (*ConfigPatch)(nil).Resolve()
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.Level = "invalid"
	if err := cfg.Validate(); !errors.Is(err, ErrInvalidConfig) || containsHan(err.Error()) {
		t.Fatalf("validation must preserve the sentinel with English context: %v", err)
	}
	source := &os.PathError{Op: "write", Path: "服務日誌.log", Err: os.ErrPermission}
	joined := closeOwnedResources([]io.Closer{languageErrorCloser{source}}, "close logger resources")
	var pathError *os.PathError
	if !errors.Is(joined, os.ErrPermission) || !errors.As(joined, &pathError) || pathError != source {
		t.Fatalf("cleanup lost the original error chain: %v", joined)
	}
}

type languageErrorCloser struct{ err error }

func (c languageErrorCloser) Close() error { return c.err }

func TestEnglishRuntimeDiagnostics(t *testing.T) {
	t.Run("initialization", func(t *testing.T) {
		cfg := fileOutputTestConfig(t.TempDir(), "app.log")
		cfg.Format = "json"
		instance, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := instance.Close(); err != nil {
				t.Error(err)
			}
		})
		if err := instance.Sync(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(cfg.LogPath, cfg.FileName))
		if err != nil {
			t.Fatal(err)
		}
		entry := decodeTestJSON(t, string(data))
		if entry["msg"] != "logger initialized" {
			t.Fatalf("initialization diagnostic = %v", entry)
		}
	})
	t.Run("rotation", testEnglishRotationDiagnostic)
	output, err := NewSplitOutput(t.TempDir(), "app")
	if err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = output.Write(zapcore.InfoLevel, nil)
	if !errors.Is(err, os.ErrClosed) || !strings.HasPrefix(err.Error(), "split output is closed: ") {
		t.Fatalf("closed output diagnostic = %v", err)
	}
}

func testEnglishRotationDiagnostic(t *testing.T) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stderr
	os.Stderr = writer
	t.Cleanup(func() {
		os.Stderr = previous
		if err := writer.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Error(err)
		}
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	})
	now := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)
	clock := newManualRotationClock(now)
	source := errors.New("source error")
	calls := 0
	opener := func(directory, prefix, date string, _ os.FileMode) (splitFileSet, error) {
		calls++
		if calls > 1 {
			return splitFileSet{}, source
		}
		return openSplitFiles(directory, prefix, date)
	}
	observer := newTestObserver(t, 1)
	settings, err := resolveFileOutputOptions(WithOutputObserver(observer))
	if err != nil {
		t.Fatal(err)
	}
	output, err := newSplitOutputWithSettings(t.TempDir(), "app", clock, opener, settings)
	if err != nil {
		t.Fatal(err)
	}
	// Stop the worker before restoring stderr, including when an assertion fails.
	t.Cleanup(func() {
		if err := output.Close(); err != nil {
			t.Error(err)
		}
	})
	timer := clock.nextTimer(t)
	clock.setNow(now.AddDate(0, 0, 1))
	timer.fire(clock.Now())
	select {
	case event := <-observer.Events():
		if event.Operation != "rotate" || !errors.Is(event.Err, source) {
			t.Fatalf("rotation event lost its source error: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for rotation failure")
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stderr = previous
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "daily log rotation failed: "+source.Error()+"\n"; got != want {
		t.Fatalf("rotation stderr = %q, want %q", got, want)
	}
}

func TestUnicodePayloadPreserved(t *testing.T) {
	var buffer bytes.Buffer
	logger := zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(testEncoderConfig()), zapcore.AddSync(&buffer), zapcore.DebugLevel))
	message := "服務啟動 <>&"
	value := "使用者資料"
	source := errors.New("source error")
	logger.Info(message, String("value", value), Err(source))
	entry := decodeTestJSON(t, buffer.String())
	if entry["msg"] != message || entry["value"] != value || entry["error"] != source.Error() {
		t.Fatalf("caller-provided Unicode data changed: %v", entry)
	}
}
