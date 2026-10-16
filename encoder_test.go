package zlogger

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewNoEscapeJSONEncoderPreservesHTMLCharacters(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		escapedHex string
	}{
		{name: "less_than", value: "<", escapedHex: `\u003c`},
		{name: "greater_than", value: ">", escapedHex: `\u003e`},
		{name: "ampersand", value: "&", escapedHex: `\u0026`},
		{name: "combined", value: "<script>&value</script>", escapedHex: `\u003c`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := encodeTestEntry(
				t,
				NewNoEscapeJSONEncoder(testEncoderConfig()),
				zapcore.Entry{Message: tt.value},
				zap.String("field", tt.value),
			)

			if !strings.Contains(output, tt.value) {
				t.Errorf("output did not preserve HTML characters: %q", output)
			}
			if strings.Contains(strings.ToLower(output), tt.escapedHex) {
				t.Errorf("output must not contain HTML Unicode escape %q: %q", tt.escapedHex, output)
			}

			decoded := decodeTestJSON(t, output)
			if decoded["msg"] != tt.value || decoded["field"] != tt.value {
				t.Errorf("decoded JSON value: got %#v, want %q", decoded, tt.value)
			}
		})
	}
}

func TestNewNoEscapeJSONEncoderMatchesZap(t *testing.T) {
	entry := zapcore.Entry{Message: "quote \" backslash \\ newline\n"}
	field := zap.String("field", "<value> & quote \" backslash \\")

	got := decodeTestJSON(t, encodeTestEntry(
		t,
		NewNoEscapeJSONEncoder(testEncoderConfig()),
		entry,
		field,
	))
	want := decodeTestJSON(t, encodeTestEntry(
		t,
		zapcore.NewJSONEncoder(testEncoderConfig()),
		entry,
		field,
	))

	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrapper and zap encoder differ: got %#v, want %#v", got, want)
	}
}

func TestDisableHTMLEscapingReturnsOriginalLogger(t *testing.T) {
	logger := zap.NewNop()

	result := DisableHTMLEscaping(logger)
	if result != logger {
		t.Fatal("DisableHTMLEscaping must return the logger unchanged")
	}
}

func TestDisableHTMLEscapingNilLogger(t *testing.T) {
	if result := DisableHTMLEscaping(nil); result != nil {
		t.Fatalf("nil logger must return nil, got %p", result)
	}
}

func testEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		MessageKey: "msg",
		LineEnding: "\n",
	}
}

func encodeTestEntry(
	t *testing.T,
	encoder zapcore.Encoder,
	entry zapcore.Entry,
	fields ...zapcore.Field,
) string {
	t.Helper()

	buf, err := encoder.EncodeEntry(entry, fields)
	if err != nil {
		t.Fatalf("encode entry: %v", err)
	}
	defer buf.Free()

	return buf.String()
}

func decodeTestJSON(t *testing.T, output string) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("invalid JSON output: %v; content: %q", err, output)
	}
	return decoded
}
