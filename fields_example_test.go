package zlogger_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/vincent119/zlogger"
)

// Run examples through the public file-output API without importing zap.
func printFieldExample(fields ...zlogger.Field) (result error) {
	directory, err := os.MkdirTemp("", "zlogger-fields-example-")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, os.RemoveAll(directory)) }()
	cfg := zlogger.DefaultConfig()
	cfg.Format, cfg.Outputs = "json", []string{"file"}
	cfg.LogPath, cfg.FileName, cfg.AddCaller = directory, "example.log", false
	instance, err := zlogger.New(cfg)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, instance.Close()) }()
	instance.Logger().Info("example", fields...)
	if err := instance.Sync(); err != nil {
		return err
	}
	if err := instance.Close(); err != nil {
		return err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, root.Close()) }()
	data, err := root.ReadFile("example.log")
	if err != nil {
		return err
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	var entry map[string]json.RawMessage
	if err := json.Unmarshal(lines[len(lines)-1], &entry); err != nil {
		return err
	}
	delete(entry, "ts")
	delete(entry, "level")
	delete(entry, "msg")
	encoded, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(encoded))
	return err
}

func ExampleObject() {
	if err := printFieldExample(zlogger.Object("profile", helperProfile{"reader"})); err != nil {
		fmt.Println(err)
	}
	// Output: {"profile":{"name":"reader","codes":[2,1]}}
}

func ExampleArray() {
	if err := printFieldExample(zlogger.Array("codes", helperCodes{2, 1})); err != nil {
		fmt.Println(err)
	}
	// Output: {"codes":[2,1]}
}

func ExampleInline() {
	if err := printFieldExample(zlogger.Inline(helperProfile{"reader"})); err != nil {
		fmt.Println(err)
	}
	// Output: {"codes":[2,1],"name":"reader"}
}

func ExampleNamespace() {
	if err := printFieldExample(zlogger.Namespace("request"), zlogger.String("id", "r1"), zlogger.Int("attempt", 2)); err != nil {
		fmt.Println(err)
	}
	// Output: {"request":{"id":"r1","attempt":2}}
}

func ExampleInts() {
	if err := printFieldExample(zlogger.Ints("attempts", nil), zlogger.Ints("numbers", []int{2, 1})); err != nil {
		fmt.Println(err)
	}
	// Output: {"attempts":[],"numbers":[2,1]}
}
