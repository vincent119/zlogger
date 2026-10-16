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
		{name: "regular filename", leaf: "app.log"},
		{name: "Unicode filename", leaf: "服務日誌.log"},
		{name: "empty name allowed", leaf: "", allowEmpty: true},
		{name: "empty name rejected", leaf: "", wantErr: true},
		{name: "parent traversal", leaf: "../outside.log", wantErr: true},
		{name: "Unix absolute path", leaf: "/tmp/outside.log", wantErr: true},
		{name: "forward slash", leaf: "sub/app.log", wantErr: true},
		{name: "backslash", leaf: `sub\app.log`, wantErr: true},
		{name: "current directory", leaf: ".", wantErr: true},
		{name: "parent directory", leaf: "..", wantErr: true},
		{name: "NUL", leaf: "app\x00.log", wantErr: true},
		{name: "Windows drive", leaf: `C:app.log`, wantErr: true},
		{name: "Windows absolute path", leaf: `C:\logs\app.log`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLogLeaf(tt.leaf, tt.allowEmpty)
			if tt.wantErr {
				if !errors.Is(err, ErrUnsafeLogPath) {
					t.Fatalf("error = %v, want ErrUnsafeLogPath", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("safe leaf %q must not fail: %v", tt.leaf, err)
			}
		})
	}
}

func TestFileOutputsStayWithinBaseDirectory(t *testing.T) {
	t.Run("regular output", func(t *testing.T) {
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
			t.Fatalf("create regular file output: %v", err)
		}
		instance.Logger().Info("safe path test")
		if err := instance.Sync(); err != nil {
			t.Fatalf("sync regular file output: %v", err)
		}
		if err := instance.Close(); err != nil {
			t.Fatalf("close regular file output: %v", err)
		}

		if _, err := os.Stat(filepath.Join(base, "app.log")); err != nil {
			t.Fatalf("expected file does not exist: %v", err)
		}
		if _, err := os.Stat(filepath.Join(parent, "app.log")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("must not create files outside base, Stat error = %v", err)
		}
	})

	t.Run("split output", func(t *testing.T) {
		parent := t.TempDir()
		base := filepath.Join(parent, "logs")
		output, err := NewSplitOutput(base, "app")
		if err != nil {
			t.Fatalf("create SplitOutput: %v", err)
		}
		if _, err := output.Write(zapcore.InfoLevel, []byte("safe path test\n")); err != nil {
			t.Fatalf("write SplitOutput: %v", err)
		}
		if err := output.Close(); err != nil {
			t.Fatalf("close SplitOutput: %v", err)
		}

		date := time.Now().Format("2006-01-02")
		if _, err := os.Stat(filepath.Join(base, "app-info-"+date+".log")); err != nil {
			t.Fatalf("expected info file does not exist: %v", err)
		}
		if _, err := os.Stat(filepath.Join(parent, "app-info-"+date+".log")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("must not create split files outside base, Stat error = %v", err)
		}
	})
}

func TestRootedFileOutputRejectsExistingSymlink(t *testing.T) {
	parent := t.TempDir()
	base := filepath.Join(parent, "logs")
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatalf("create base directory: %v", err)
	}
	outside := filepath.Join(parent, "outside.log")
	const original = "original external content\n"
	if err := os.WriteFile(outside, []byte(original), 0o600); err != nil {
		t.Fatalf("create external file: %v", err)
	}
	link := filepath.Join(base, "app.log")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("platform cannot create symlink: %v", err)
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
		t.Fatal("existing symlink target must be rejected")
	}
	if !errors.Is(err, ErrUnsafeLogPath) {
		t.Fatalf("error = %v, want ErrUnsafeLogPath", err)
	}
	assertFileContent(t, outside, original)
}

func TestRootedSplitOutputRejectsExistingSymlink(t *testing.T) {
	parent := t.TempDir()
	base := filepath.Join(parent, "logs")
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatalf("create base directory: %v", err)
	}
	outside := filepath.Join(parent, "outside.log")
	const original = "original external content\n"
	if err := os.WriteFile(outside, []byte(original), 0o600); err != nil {
		t.Fatalf("create external file: %v", err)
	}
	date := time.Now().Format("2006-01-02")
	link := filepath.Join(base, "app-warn-"+date+".log")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("platform cannot create symlink: %v", err)
	}

	output, err := NewSplitOutput(base, "app")
	if err == nil {
		_, _ = output.Write(zapcore.WarnLevel, []byte("must not write outside\n"))
		_ = output.Close()
		t.Fatal("existing symlink target must be rejected")
	}
	if !errors.Is(err, ErrUnsafeLogPath) {
		t.Fatalf("error = %v, want ErrUnsafeLogPath", err)
	}
	assertFileContent(t, outside, original)
}

func TestFileOutputsUsePrivatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not provide equivalent POSIX mode semantics")
	}

	t.Run("regular output", func(t *testing.T) {
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
			t.Fatalf("create regular file output: %v", err)
		}
		if err := instance.Close(); err != nil {
			t.Fatalf("close regular file output: %v", err)
		}
		assertPrivateMode(t, base)
		assertPrivateMode(t, filepath.Join(base, "app.log"))
	})

	t.Run("split output", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "logs")
		output, err := NewSplitOutput(base, "app")
		if err != nil {
			t.Fatalf("create SplitOutput: %v", err)
		}
		if err := output.Close(); err != nil {
			t.Fatalf("close SplitOutput: %v", err)
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
		t.Skip("Windows does not provide equivalent POSIX mode semantics")
	}

	base := t.TempDir()
	path := filepath.Join(base, "app.log")
	//nolint:gosec // Deliberately create a 0640 file to verify that existing permissions remain unchanged.
	if err := os.WriteFile(path, []byte("existing content\n"), 0o640); err != nil {
		t.Fatalf("create existing file: %v", err)
	}
	//nolint:gosec // Deliberately set 0640 to verify that existing group-read permissions remain unchanged.
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatalf("set existing mode: %v", err)
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
		t.Fatalf("open existing file: %v", err)
	}
	instance.Logger().Info("appended content")
	if err := instance.Close(); err != nil {
		t.Fatalf("close regular file output: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat existing file: %v", err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("existing mode = %04o, want 0640", info.Mode().Perm())
	}
	//nolint:gosec // The path combines t.TempDir with a fixed safe leaf name.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read existing file: %v", err)
	}
	if !strings.Contains(string(content), "appended content") {
		t.Fatalf("content was not appended to existing file: %s", content)
	}
}

func TestFileOutputOptionsPreserveExistingPermissions(t *testing.T) {
	base := filepath.Join(t.TempDir(), "logs")
	if err := os.Mkdir(base, 0o750); err != nil {
		t.Fatalf("create existing directory: %v", err)
	}
	path := filepath.Join(base, "app.log")
	const original = "existing content\n"
	//nolint:gosec // Deliberately create a 0640 file to verify that options preserve existing permissions.
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatalf("create existing file: %v", err)
	}
	beforeDir := pathPermission(t, base)
	beforeFile := pathPermission(t, path)

	instance, err := NewWithOptions(
		fileOutputTestConfig(base, "app.log"),
		WithDirPerm(0o700),
		WithFilePerm(0o600),
	)
	if err != nil {
		t.Fatalf("open existing file output: %v", err)
	}
	if err := instance.Close(); err != nil {
		t.Fatalf("close existing file output: %v", err)
	}

	if got := pathPermission(t, base); got != beforeDir {
		t.Fatalf("existing directory mode = %04o, originally %04o", got, beforeDir)
	}
	if got := pathPermission(t, path); got != beforeFile {
		t.Fatalf("existing file mode = %04o, originally %04o", got, beforeFile)
	}
	//nolint:gosec // The path combines t.TempDir with a fixed safe leaf name.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read existing log file: %v", err)
	}
	if !strings.HasPrefix(string(content), original) {
		t.Fatalf("existing content was overwritten: %q", content)
	}
}

func TestRootedFileOpenContainsConcurrentReplacement(t *testing.T) {
	parent := t.TempDir()
	base := filepath.Join(parent, "logs")
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatalf("create base directory: %v", err)
	}

	outside := filepath.Join(parent, "outside.log")
	const original = "original external content\n"
	if err := os.WriteFile(outside, []byte(original), 0o600); err != nil {
		t.Fatalf("create external file: %v", err)
	}

	const leaf = "app.log"
	target := filepath.Join(base, leaf)
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatalf("create initial leaf: %v", err)
	}

	probe := filepath.Join(base, "symlink-probe")
	if err := os.Symlink(outside, probe); err != nil {
		t.Skipf("platform cannot create symlink: %v", err)
	}
	if err := os.Remove(probe); err != nil {
		t.Fatalf("remove symlink probe: %v", err)
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
		t.Fatal("opening must reject concurrent replacement with a symlink outside root")
	}
	if !replaced {
		t.Fatal("test did not replace the leaf after inspection")
	}
	if tracked == nil || tracked.closeCalls != 1 {
		t.Fatalf("root close calls = %d, want 1", rootCloseCalls(tracked))
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
		t.Fatalf("open rooted files in batch: %v", err)
	}
	t.Cleanup(func() {
		closeTestFiles(t, files)
	})
	if openCalls != 1 {
		t.Fatalf("OpenRoot calls = %d, want 1", openCalls)
	}
	if tracked.closeCalls != 1 {
		t.Fatalf("root close calls = %d, want 1", tracked.closeCalls)
	}
	if len(files) != 3 {
		t.Fatalf("file count = %d, want 3", len(files))
	}
	for index, file := range files {
		if _, err := file.Write([]byte("batch write\n")); err != nil {
			t.Fatalf("write rooted file %d: %v", index, err)
		}
	}
}

func TestOpenRootedLogFilesClosesResourcesOnFailure(t *testing.T) {
	t.Run("partial file failure", func(t *testing.T) {
		parent := t.TempDir()
		base := filepath.Join(parent, "logs")
		if err := os.Mkdir(base, 0o700); err != nil {
			t.Fatalf("create base directory: %v", err)
		}
		outside := filepath.Join(parent, "outside.log")
		if err := os.WriteFile(outside, nil, 0o600); err != nil {
			t.Fatalf("create external file: %v", err)
		}
		if err := os.Symlink(outside, filepath.Join(base, "blocked.log")); err != nil {
			t.Skipf("platform cannot create symlink: %v", err)
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
			t.Fatalf("error = %v, want ErrUnsafeLogPath", err)
		}
		assertTrackedResourcesClosed(t, tracked, 1)
	})

	t.Run("root close failure", func(t *testing.T) {
		base := t.TempDir()
		closeErr := errors.New("test root close failed")
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
			t.Fatalf("error = %v, want preserved root close error", err)
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
		t.Fatal("expected a tracking root")
	}
	if root.closeCalls != 1 {
		t.Fatalf("root close calls = %d, want 1", root.closeCalls)
	}
	if len(root.opened) != wantFiles {
		t.Fatalf("opened file count = %d, want %d", len(root.opened), wantFiles)
	}
	for index, file := range root.opened {
		if _, err := file.Write([]byte("must not succeed")); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("partial file %d was not closed, Write error = %v", index, err)
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
			t.Errorf("close test file: %v", err)
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
	//nolint:gosec // The helper only accepts expected paths created under t.TempDir.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %q: %v", path, err)
	}
	if string(content) != want {
		t.Fatalf("file %q content = %q, want %q", path, content, want)
	}
}

func assertPrivateMode(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("read mode %q: %v", path, err)
	}
	if got := info.Mode().Perm() & 0o077; got != 0 {
		t.Fatalf("%q group/other mode bits = %04o, want 0", path, got)
	}
}
