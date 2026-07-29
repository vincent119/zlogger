package zlogger

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
)

func TestValidateLogLeaf(t *testing.T) {
	tests := []struct {
		name       string
		leaf       string
		allowEmpty bool
		wantErr    bool
	}{
		{name: "一般檔名", leaf: "app.log"},
		{name: "Unicode 檔名", leaf: "服務日誌.log"},
		{name: "空字串允許", leaf: "", allowEmpty: true},
		{name: "空字串拒絕", leaf: "", wantErr: true},
		{name: "上層 traversal", leaf: "../outside.log", wantErr: true},
		{name: "Unix 絕對路徑", leaf: "/tmp/outside.log", wantErr: true},
		{name: "正斜線", leaf: "sub/app.log", wantErr: true},
		{name: "反斜線", leaf: `sub\app.log`, wantErr: true},
		{name: "目前目錄", leaf: ".", wantErr: true},
		{name: "上層目錄", leaf: "..", wantErr: true},
		{name: "NUL", leaf: "app\x00.log", wantErr: true},
		{name: "Windows drive", leaf: `C:app.log`, wantErr: true},
		{name: "Windows 絕對路徑", leaf: `C:\logs\app.log`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLogLeaf(tt.leaf, tt.allowEmpty)
			if tt.wantErr {
				if !errors.Is(err, ErrUnsafeLogPath) {
					t.Fatalf("錯誤 = %v，預期 ErrUnsafeLogPath", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("安全 leaf %q 不應失敗：%v", tt.leaf, err)
			}
		})
	}
}

func TestSecureLogPath(t *testing.T) {
	base := filepath.Join(t.TempDir(), "logs")
	target, err := secureLogPath(base, "app.log")
	if err != nil {
		t.Fatalf("組合安全路徑失敗：%v", err)
	}
	if target != filepath.Join(base, "app.log") {
		t.Fatalf("target = %q，預期 %q", target, filepath.Join(base, "app.log"))
	}

	if _, err := secureLogPath(base, "../outside.log"); !errors.Is(err, ErrUnsafeLogPath) {
		t.Fatalf("錯誤 = %v，預期 ErrUnsafeLogPath", err)
	}
}

func TestFileOutputsStayWithinBaseDirectory(t *testing.T) {
	t.Run("一般輸出", func(t *testing.T) {
		parent := t.TempDir()
		base := filepath.Join(parent, "logs")
		cfg := &Config{
			Level:    "info",
			Format:   "json",
			Outputs:  []string{"file"},
			LogPath:  base,
			FileName: "app.log",
		}

		instance, err := New(cfg)
		if err != nil {
			t.Fatalf("建立一般 file output 失敗：%v", err)
		}
		instance.Logger().Info("安全路徑測試")
		if err := instance.Sync(); err != nil {
			t.Fatalf("同步一般 file output 失敗：%v", err)
		}
		if err := instance.Close(); err != nil {
			t.Fatalf("關閉一般 file output 失敗：%v", err)
		}

		if _, err := os.Stat(filepath.Join(base, "app.log")); err != nil {
			t.Fatalf("預期檔案不存在：%v", err)
		}
		if _, err := os.Stat(filepath.Join(parent, "app.log")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("base 外不應建立檔案，Stat 錯誤 = %v", err)
		}
	})

	t.Run("分級輸出", func(t *testing.T) {
		parent := t.TempDir()
		base := filepath.Join(parent, "logs")
		output, err := NewSplitOutput(base, "app")
		if err != nil {
			t.Fatalf("建立 SplitOutput 失敗：%v", err)
		}
		if _, err := output.Write(zapcore.InfoLevel, []byte("安全路徑測試\n")); err != nil {
			t.Fatalf("寫入 SplitOutput 失敗：%v", err)
		}
		if err := output.Close(); err != nil {
			t.Fatalf("關閉 SplitOutput 失敗：%v", err)
		}

		date := time.Now().Format("2006-01-02")
		if _, err := os.Stat(filepath.Join(base, "app-info-"+date+".log")); err != nil {
			t.Fatalf("預期 info 檔不存在：%v", err)
		}
		if _, err := os.Stat(filepath.Join(parent, "app-info-"+date+".log")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("base 外不應建立分級檔案，Stat 錯誤 = %v", err)
		}
	})
}

func TestFileOutputsRejectExistingSymlink(t *testing.T) {
	t.Run("一般輸出", func(t *testing.T) {
		parent := t.TempDir()
		base := filepath.Join(parent, "logs")
		if err := os.Mkdir(base, 0o700); err != nil {
			t.Fatalf("建立 base 失敗：%v", err)
		}
		outside := filepath.Join(parent, "outside.log")
		const original = "外部原始內容\n"
		if err := os.WriteFile(outside, []byte(original), 0o600); err != nil {
			t.Fatalf("建立外部檔案失敗：%v", err)
		}
		link := filepath.Join(base, "app.log")
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("平台無法建立 symlink：%v", err)
		}

		cfg := &Config{
			Level:    "info",
			Format:   "json",
			Outputs:  []string{"file"},
			LogPath:  base,
			FileName: "app.log",
		}
		instance, err := New(cfg)
		if err == nil {
			_ = instance.Close()
			t.Fatal("既有 symlink 目標應被拒絕")
		}
		if !errors.Is(err, ErrUnsafeLogPath) {
			t.Fatalf("錯誤 = %v，預期 ErrUnsafeLogPath", err)
		}
		assertFileContent(t, outside, original)
	})

	t.Run("分級輸出", func(t *testing.T) {
		parent := t.TempDir()
		base := filepath.Join(parent, "logs")
		if err := os.Mkdir(base, 0o700); err != nil {
			t.Fatalf("建立 base 失敗：%v", err)
		}
		outside := filepath.Join(parent, "outside.log")
		const original = "外部原始內容\n"
		if err := os.WriteFile(outside, []byte(original), 0o600); err != nil {
			t.Fatalf("建立外部檔案失敗：%v", err)
		}
		date := time.Now().Format("2006-01-02")
		link := filepath.Join(base, "app-warn-"+date+".log")
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("平台無法建立 symlink：%v", err)
		}

		output, err := NewSplitOutput(base, "app")
		if err == nil {
			_, _ = output.Write(zapcore.WarnLevel, []byte("不應寫入外部\n"))
			_ = output.Close()
			t.Fatal("既有 symlink 目標應被拒絕")
		}
		if !errors.Is(err, ErrUnsafeLogPath) {
			t.Fatalf("錯誤 = %v，預期 ErrUnsafeLogPath", err)
		}
		assertFileContent(t, outside, original)
	})
}

func TestFileOutputsUsePrivatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不提供相同 POSIX mode 語意")
	}

	t.Run("一般輸出", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "logs")
		cfg := &Config{
			Level:    "info",
			Format:   "json",
			Outputs:  []string{"file"},
			LogPath:  base,
			FileName: "app.log",
		}
		instance, err := New(cfg)
		if err != nil {
			t.Fatalf("建立一般 file output 失敗：%v", err)
		}
		if err := instance.Close(); err != nil {
			t.Fatalf("關閉一般 file output 失敗：%v", err)
		}
		assertPrivateMode(t, base)
		assertPrivateMode(t, filepath.Join(base, "app.log"))
	})

	t.Run("分級輸出", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "logs")
		output, err := NewSplitOutput(base, "app")
		if err != nil {
			t.Fatalf("建立 SplitOutput 失敗：%v", err)
		}
		if err := output.Close(); err != nil {
			t.Fatalf("關閉 SplitOutput 失敗：%v", err)
		}
		assertPrivateMode(t, base)
		date := time.Now().Format("2006-01-02")
		for _, level := range []string{"info", "warn", "error"} {
			assertPrivateMode(t, filepath.Join(base, "app-"+level+"-"+date+".log"))
		}
	})
}

func TestFileOutputPreservesExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不提供相同 POSIX mode 語意")
	}

	base := t.TempDir()
	path := filepath.Join(base, "app.log")
	if err := os.WriteFile(path, []byte("既有內容\n"), 0o640); err != nil {
		t.Fatalf("建立既有檔案失敗：%v", err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatalf("設定既有 mode 失敗：%v", err)
	}

	cfg := &Config{
		Level:    "info",
		Format:   "json",
		Outputs:  []string{"file"},
		LogPath:  base,
		FileName: "app.log",
	}
	instance, err := New(cfg)
	if err != nil {
		t.Fatalf("開啟既有檔案失敗：%v", err)
	}
	instance.Logger().Info("追加內容")
	if err := instance.Close(); err != nil {
		t.Fatalf("關閉一般 file output 失敗：%v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("讀取既有檔案資訊失敗：%v", err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("既有 mode = %04o，預期 0640", info.Mode().Perm())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("讀取既有檔案失敗：%v", err)
	}
	if !strings.Contains(string(content), "追加內容") {
		t.Fatalf("既有檔案未追加內容：%s", content)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("讀取檔案 %q 失敗：%v", path, err)
	}
	if string(content) != want {
		t.Fatalf("檔案 %q 內容 = %q，預期 %q", path, content, want)
	}
}

func assertPrivateMode(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("讀取 mode %q 失敗：%v", path, err)
	}
	if got := info.Mode().Perm() & 0o077; got != 0 {
		t.Fatalf("%q 的 group／other mode bits = %04o，預期為 0", path, got)
	}
}
