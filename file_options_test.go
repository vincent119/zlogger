package zlogger

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap/zapcore"
)

var (
	_ func(*Config) (*Instance, error)                                          = New
	_ func(*ConfigPatch) (func() error, error)                                  = Configure
	_ func(string, string) (*SplitOutput, error)                                = NewSplitOutput
	_ func(string, string, zapcore.EncoderConfig) (zapcore.Core, func(), error) = GetSplitCore
)

func TestOutputObserverOptions(t *testing.T) {
	o := newTestObserver(t, 1)
	var zero FailureObserver
	for _, opts := range [][]FileOutputOption{
		{WithOutputObserver(nil)}, {WithOutputObserver(&zero)}, {WithOutputObserver(o), WithOutputObserver(o)},
	} {
		base := filepath.Join(t.TempDir(), "must-not-exist")
		if s, err := NewSplitOutputWithOptions(base, "app", opts...); s != nil || !errors.Is(err, ErrInvalidConfig) {
			t.Fatal(err)
		}
		assertPathDoesNotExist(t, base)
	}
	cfg := fileOutputTestConfig(filepath.Join(t.TempDir(), "must-not-exist"), "app.log")
	if i, err := NewWithOptions(cfg, WithOutputObserver(o)); i != nil || !errors.Is(err, ErrInvalidConfig) {
		t.Fatal(err)
	}
	assertPathDoesNotExist(t, cfg.LogPath)
	s, err := NewSplitOutputWithOptions(t.TempDir(), "app", WithOutputObserver(o))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-o.Events():
		t.Fatal(event)
	default:
	}
}

func TestFileOutputOptionsDefaultsAndLastWins(t *testing.T) {
	settings, err := resolveFileOutputOptions()
	if err != nil {
		t.Fatalf("resolve default file output options: %v", err)
	}
	if settings.dirPerm != defaultLogDirMode || settings.filePerm != defaultLogFileMode {
		t.Fatalf(
			"default permissions = %04o/%04o, want %04o/%04o",
			settings.dirPerm,
			settings.filePerm,
			defaultLogDirMode,
			defaultLogFileMode,
		)
	}

	settings, err = resolveFileOutputOptions(
		WithDirPerm(0o750),
		WithFilePerm(0o640),
		WithDirPerm(0o770),
		WithFilePerm(0o660),
	)
	if err != nil {
		t.Fatalf("resolve custom file output options: %v", err)
	}
	if settings.dirPerm != 0o770 || settings.filePerm != 0o660 {
		t.Fatalf("last option did not take effect, permissions = %04o/%04o", settings.dirPerm, settings.filePerm)
	}
}

func TestFileOutputOptionsRejectInvalidPermissions(t *testing.T) {
	tests := []struct {
		name   string
		option FileOutputOption
	}{
		{name: "nil option", option: nil},
		{name: "directory with type bits", option: WithDirPerm(os.ModeDir | 0o700)},
		{name: "directory missing owner execute", option: WithDirPerm(0o600)},
		{name: "directory allowing other write", option: WithDirPerm(0o702)},
		{name: "file with special bits", option: WithFilePerm(os.ModeSetuid | 0o600)},
		{name: "file missing owner write", option: WithFilePerm(0o400)},
		{name: "file allowing other write", option: WithFilePerm(0o602)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "must-not-exist")
			cfg := fileOutputTestConfig(base, "app.log")

			instance, err := NewWithOptions(cfg, tt.option)
			if !errors.Is(err, ErrInvalidFilePermission) {
				t.Fatalf("error = %v, want ErrInvalidFilePermission", err)
			}
			if instance != nil {
				t.Fatal("invalid permission option must not return a partial Instance")
			}
			assertPathDoesNotExist(t, base)
		})
	}
}

func TestFileOutputEntryPointsRejectInvalidPermissionBeforeIO(t *testing.T) {
	invalid := WithFilePerm(0o602)
	encoderConfig := zapcore.EncoderConfig{
		MessageKey: "msg",
		EncodeTime: zapcore.ISO8601TimeEncoder,
	}

	tests := []struct {
		name string
		run  func(*testing.T, string) error
	}{
		{
			name: "NewWithOptions",
			run: func(_ *testing.T, base string) error {
				instance, err := NewWithOptions(fileOutputTestConfig(base, "app.log"), invalid)
				if instance != nil {
					_ = instance.Close()
					return errors.New("must not return a partial Instance")
				}
				return err
			},
		},
		{
			name: "ConfigureWithOptions",
			run: func(t *testing.T, base string) error {
				resetGlobalState(t)
				t.Cleanup(func() { resetGlobalState(t) })
				outputs := []string{"file"}
				fileName := "app.log"
				cleanup, err := ConfigureWithOptions(&ConfigPatch{
					Outputs:  &outputs,
					LogPath:  &base,
					FileName: &fileName,
				}, invalid)
				if cleanup != nil {
					_ = cleanup()
					return errors.New("must not return cleanup")
				}
				if GetLogger() != nil {
					return errors.New("must not publish a global logger")
				}
				return err
			},
		},
		{
			name: "NewSplitOutputWithOptions",
			run: func(_ *testing.T, base string) error {
				output, err := NewSplitOutputWithOptions(base, "app", invalid)
				if output != nil {
					_ = output.Close()
					return errors.New("must not return a partial SplitOutput")
				}
				return err
			},
		},
		{
			name: "GetSplitCoreWithOptions",
			run: func(_ *testing.T, base string) error {
				core, cleanup, err := GetSplitCoreWithOptions(base, "app", encoderConfig, invalid)
				if cleanup != nil {
					cleanup()
					return errors.New("must not return cleanup")
				}
				if core != nil {
					return errors.New("must not return a partial core")
				}
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "must-not-exist")
			err := tt.run(t, base)
			if !errors.Is(err, ErrInvalidFilePermission) {
				t.Fatalf("error = %v, want ErrInvalidFilePermission", err)
			}
			assertPathDoesNotExist(t, base)
		})
	}
}

func fileOutputTestConfig(base, fileName string) *Config {
	return &Config{
		Level:    "info",
		Format:   "json",
		Outputs:  []string{"file"},
		LogPath:  base,
		FileName: fileName,
	}
}

func assertPathDoesNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("path %q must not exist, Stat error = %v", path, err)
	}
}

func pathPermission(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("read %q mode: %v", path, err)
	}
	return info.Mode().Perm()
}

func assertSamePermission(t *testing.T, gotPath, referencePath string) {
	t.Helper()
	got := pathPermission(t, gotPath)
	want := pathPermission(t, referencePath)
	if got != want {
		t.Fatalf("%q mode = %04o, reference %q mode = %04o", gotPath, got, referencePath, want)
	}
}

func createPermissionReference(t *testing.T, parent string) (string, string) {
	t.Helper()
	referenceDir := filepath.Join(parent, "reference")
	if err := os.Mkdir(referenceDir, 0o750); err != nil {
		t.Fatalf("create reference directory: %v", err)
	}
	referenceFile := filepath.Join(referenceDir, "reference.log")
	//nolint:gosec // Deliberately create a 0640 reference file to verify custom group-read permissions.
	if err := os.WriteFile(referenceFile, nil, 0o640); err != nil {
		t.Fatalf("create reference file: %v", err)
	}
	return referenceDir, referenceFile
}
