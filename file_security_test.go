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

func TestRootedFileOutputRejectsExistingSymlink(t *testing.T) {
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
}

func TestRootedSplitOutputRejectsExistingSymlink(t *testing.T) {
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
	//nolint:gosec // 測試刻意建立 0640 檔案，驗證 logger 不會改寫既有權限。
	if err := os.WriteFile(path, []byte("既有內容\n"), 0o640); err != nil {
		t.Fatalf("建立既有檔案失敗：%v", err)
	}
	//nolint:gosec // 測試刻意設定 0640，驗證既有 group-read 權限保持不變。
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
	//nolint:gosec // path 由 t.TempDir 與固定安全 leaf 組成。
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("讀取既有檔案失敗：%v", err)
	}
	if !strings.Contains(string(content), "追加內容") {
		t.Fatalf("既有檔案未追加內容：%s", content)
	}
}

func TestRootedFileOpenContainsConcurrentReplacement(t *testing.T) {
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

	const leaf = "app.log"
	target := filepath.Join(base, leaf)
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatalf("建立初始 leaf 失敗：%v", err)
	}

	probe := filepath.Join(base, "symlink-probe")
	if err := os.Symlink(outside, probe); err != nil {
		t.Skipf("平台無法建立 symlink：%v", err)
	}
	if err := os.Remove(probe); err != nil {
		t.Fatalf("移除 symlink probe 失敗：%v", err)
	}

	replaced := false
	var tracked *trackingLogRoot
	openRoot := func(name string) (rootedDirectory, error) {
		root, err := os.OpenRoot(name)
		if err != nil {
			return nil, err
		}
		tracked = &trackingLogRoot{root: root}
		tracked.beforeOpen = func(name string) error {
			if err := os.Remove(filepath.Join(base, name)); err != nil {
				return err
			}
			if err := os.Symlink(outside, filepath.Join(base, name)); err != nil {
				return err
			}
			replaced = true
			return nil
		}
		return tracked, nil
	}

	files, err := openRootedLogFilesWith(openRoot, base, leaf)
	closeTestFiles(t, files)
	if err == nil {
		t.Fatal("並行替換成 root 外 symlink 時應拒絕開檔")
	}
	if !replaced {
		t.Fatal("測試未在檢查後執行 leaf 替換")
	}
	if tracked == nil || tracked.closeCalls != 1 {
		t.Fatalf("root close 次數 = %d，預期 1", rootCloseCalls(tracked))
	}
	assertFileContent(t, outside, original)
}

func TestOpenRootedLogFilesUsesSingleRoot(t *testing.T) {
	base := t.TempDir()
	openCalls := 0
	var tracked *trackingLogRoot
	openRoot := func(name string) (rootedDirectory, error) {
		openCalls++
		root, err := os.OpenRoot(name)
		if err != nil {
			return nil, err
		}
		tracked = &trackingLogRoot{root: root}
		return tracked, nil
	}

	files, err := openRootedLogFilesWith(
		openRoot,
		base,
		"app-info.log",
		"app-warn.log",
		"app-error.log",
	)
	if err != nil {
		t.Fatalf("批次開啟 rooted files 失敗：%v", err)
	}
	t.Cleanup(func() {
		closeTestFiles(t, files)
	})
	if openCalls != 1 {
		t.Fatalf("OpenRoot 次數 = %d，預期 1", openCalls)
	}
	if tracked.closeCalls != 1 {
		t.Fatalf("root close 次數 = %d，預期 1", tracked.closeCalls)
	}
	if len(files) != 3 {
		t.Fatalf("files 數量 = %d，預期 3", len(files))
	}
	for index, file := range files {
		if _, err := file.Write([]byte("批次寫入\n")); err != nil {
			t.Fatalf("寫入第 %d 個 rooted file 失敗：%v", index, err)
		}
	}
}

func TestOpenRootedLogFilesClosesResourcesOnFailure(t *testing.T) {
	t.Run("partial file failure", func(t *testing.T) {
		parent := t.TempDir()
		base := filepath.Join(parent, "logs")
		if err := os.Mkdir(base, 0o700); err != nil {
			t.Fatalf("建立 base 失敗：%v", err)
		}
		outside := filepath.Join(parent, "outside.log")
		if err := os.WriteFile(outside, nil, 0o600); err != nil {
			t.Fatalf("建立外部檔案失敗：%v", err)
		}
		if err := os.Symlink(outside, filepath.Join(base, "blocked.log")); err != nil {
			t.Skipf("平台無法建立 symlink：%v", err)
		}

		var tracked *trackingLogRoot
		openRoot := func(name string) (rootedDirectory, error) {
			root, err := os.OpenRoot(name)
			if err != nil {
				return nil, err
			}
			tracked = &trackingLogRoot{root: root}
			return tracked, nil
		}

		files, err := openRootedLogFilesWith(openRoot, base, "first.log", "blocked.log")
		closeTestFiles(t, files)
		if !errors.Is(err, ErrUnsafeLogPath) {
			t.Fatalf("錯誤 = %v，預期 ErrUnsafeLogPath", err)
		}
		assertTrackedResourcesClosed(t, tracked, 1)
	})

	t.Run("root close failure", func(t *testing.T) {
		base := t.TempDir()
		closeErr := errors.New("測試 root close 失敗")
		var tracked *trackingLogRoot
		openRoot := func(name string) (rootedDirectory, error) {
			root, err := os.OpenRoot(name)
			if err != nil {
				return nil, err
			}
			tracked = &trackingLogRoot{root: root, closeErr: closeErr}
			return tracked, nil
		}

		files, err := openRootedLogFilesWith(openRoot, base, "first.log", "second.log")
		closeTestFiles(t, files)
		if !errors.Is(err, closeErr) {
			t.Fatalf("錯誤 = %v，預期保留 root close error", err)
		}
		assertTrackedResourcesClosed(t, tracked, 2)
	})
}

type trackingLogRoot struct {
	root       *os.Root
	beforeOpen func(string) error
	opened     []*os.File
	closeErr   error
	closeCalls int
}

func (r *trackingLogRoot) Lstat(name string) (os.FileInfo, error) {
	return r.root.Lstat(name)
}

func (r *trackingLogRoot) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	if r.beforeOpen != nil {
		beforeOpen := r.beforeOpen
		r.beforeOpen = nil
		if err := beforeOpen(name); err != nil {
			return nil, err
		}
	}
	file, err := r.root.OpenFile(name, flag, perm)
	if file != nil {
		r.opened = append(r.opened, file)
	}
	return file, err
}

func (r *trackingLogRoot) Close() error {
	r.closeCalls++
	return errors.Join(r.root.Close(), r.closeErr)
}

func assertTrackedResourcesClosed(t *testing.T, root *trackingLogRoot, wantFiles int) {
	t.Helper()
	if root == nil {
		t.Fatal("預期建立 tracking root")
	}
	if root.closeCalls != 1 {
		t.Fatalf("root close 次數 = %d，預期 1", root.closeCalls)
	}
	if len(root.opened) != wantFiles {
		t.Fatalf("已開啟檔案數 = %d，預期 %d", len(root.opened), wantFiles)
	}
	for index, file := range root.opened {
		if _, err := file.Write([]byte("不應成功")); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("第 %d 個 partial file 未關閉，Write 錯誤 = %v", index, err)
		}
	}
}

func closeTestFiles(t *testing.T, files []*os.File) {
	t.Helper()
	for _, file := range files {
		if file == nil {
			continue
		}
		if err := file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("關閉測試檔案失敗：%v", err)
		}
	}
}

func rootCloseCalls(root *trackingLogRoot) int {
	if root == nil {
		return 0
	}
	return root.closeCalls
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	//nolint:gosec // helper 只接收測試建立於 t.TempDir 的預期路徑。
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
