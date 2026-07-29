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

func TestFileOutputOptionsDefaultsAndLastWins(t *testing.T) {
	settings, err := resolveFileOutputOptions()
	if err != nil {
		t.Fatalf("解析預設檔案輸出 options 失敗：%v", err)
	}
	if settings.dirPerm != defaultLogDirMode || settings.filePerm != defaultLogFileMode {
		t.Fatalf(
			"預設 permissions = %04o/%04o，預期 %04o/%04o",
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
		t.Fatalf("解析自訂檔案輸出 options 失敗：%v", err)
	}
	if settings.dirPerm != 0o770 || settings.filePerm != 0o660 {
		t.Fatalf("最後 option 未生效，permissions = %04o/%04o", settings.dirPerm, settings.filePerm)
	}
}

func TestFileOutputOptionsRejectInvalidPermissions(t *testing.T) {
	tests := []struct {
		name   string
		option FileOutputOption
	}{
		{name: "nil option", option: nil},
		{name: "目錄含型別位元", option: WithDirPerm(os.ModeDir | 0o700)},
		{name: "目錄缺少 owner execute", option: WithDirPerm(0o600)},
		{name: "目錄允許 other write", option: WithDirPerm(0o702)},
		{name: "檔案含特殊位元", option: WithFilePerm(os.ModeSetuid | 0o600)},
		{name: "檔案缺少 owner write", option: WithFilePerm(0o400)},
		{name: "檔案允許 other write", option: WithFilePerm(0o602)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "不應建立")
			cfg := fileOutputTestConfig(base, "app.log")

			instance, err := NewWithOptions(cfg, tt.option)
			if !errors.Is(err, ErrInvalidFilePermission) {
				t.Fatalf("錯誤 = %v，預期 ErrInvalidFilePermission", err)
			}
			if instance != nil {
				t.Fatal("permission option 無效時不應回傳部分 Instance")
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
					return errors.New("不應回傳部分 Instance")
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
					return errors.New("不應回傳 cleanup")
				}
				if GetLogger() != nil {
					return errors.New("不應發布全域 logger")
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
					return errors.New("不應回傳部分 SplitOutput")
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
					return errors.New("不應回傳 cleanup")
				}
				if core != nil {
					return errors.New("不應回傳部分 core")
				}
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "不應建立")
			err := tt.run(t, base)
			if !errors.Is(err, ErrInvalidFilePermission) {
				t.Fatalf("錯誤 = %v，預期 ErrInvalidFilePermission", err)
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
		t.Fatalf("路徑 %q 不應存在，Stat 錯誤 = %v", path, err)
	}
}

func pathPermission(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("讀取 %q mode 失敗：%v", path, err)
	}
	return info.Mode().Perm()
}

func assertSamePermission(t *testing.T, gotPath, referencePath string) {
	t.Helper()
	got := pathPermission(t, gotPath)
	want := pathPermission(t, referencePath)
	if got != want {
		t.Fatalf("%q mode = %04o，參考 %q mode = %04o", gotPath, got, referencePath, want)
	}
}

func createPermissionReference(t *testing.T, parent string) (string, string) {
	t.Helper()
	referenceDir := filepath.Join(parent, "reference")
	if err := os.Mkdir(referenceDir, 0o750); err != nil {
		t.Fatalf("建立參考目錄失敗：%v", err)
	}
	referenceFile := filepath.Join(referenceDir, "reference.log")
	//nolint:gosec // 測試刻意建立 0640 參考檔，驗證自訂 group-read 權限。
	if err := os.WriteFile(referenceFile, nil, 0o640); err != nil {
		t.Fatalf("建立參考檔案失敗：%v", err)
	}
	return referenceDir, referenceFile
}
