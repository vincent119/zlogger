package zlogger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrUnsafeLogPath 表示日誌 leaf name 可能逸出基準目錄或指向 symlink。
var ErrUnsafeLogPath = errors.New("日誌檔案路徑不安全")

const (
	defaultLogDirMode  os.FileMode = 0o700
	defaultLogFileMode os.FileMode = 0o600
)

func validateLogLeaf(name string, allowEmpty bool) error {
	if name == "" {
		if allowEmpty {
			return nil
		}
		return fmt.Errorf("%w: leaf name 不可為空", ErrUnsafeLogPath)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("%w: leaf name %q 不可表示目錄", ErrUnsafeLogPath, name)
	}
	if strings.IndexByte(name, 0) >= 0 {
		return fmt.Errorf("%w: leaf name 不可包含 NUL", ErrUnsafeLogPath)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%w: leaf name %q 不可包含路徑分隔符", ErrUnsafeLogPath, name)
	}
	if filepath.IsAbs(name) || hasWindowsDrivePrefix(name) {
		return fmt.Errorf("%w: leaf name %q 不可為絕對或磁碟路徑", ErrUnsafeLogPath, name)
	}
	if filepath.Base(name) != name {
		return fmt.Errorf("%w: leaf name %q 不可包含路徑語意", ErrUnsafeLogPath, name)
	}
	return nil
}

func hasWindowsDrivePrefix(name string) bool {
	if len(name) < 2 || name[1] != ':' {
		return false
	}
	first := name[0]
	return first >= 'a' && first <= 'z' || first >= 'A' && first <= 'Z'
}

type rootedDirectory interface {
	Lstat(name string) (os.FileInfo, error)
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	Close() error
}

type rootDirectoryOpener func(string) (rootedDirectory, error)

func openRootedLogFilesWithPermissions(
	baseDir string,
	filePerm os.FileMode,
	leaves ...string,
) ([]*os.File, error) {
	return openRootedLogFilesWithPermissionsAndOpener(
		openLogRoot,
		baseDir,
		filePerm,
		leaves...,
	)
}

func openLogRoot(name string) (rootedDirectory, error) {
	return os.OpenRoot(name)
}

func openRootedLogFilesWith(
	openRoot rootDirectoryOpener,
	baseDir string,
	leaves ...string,
) ([]*os.File, error) {
	return openRootedLogFilesWithPermissionsAndOpener(
		openRoot,
		baseDir,
		defaultLogFileMode,
		leaves...,
	)
}

func openRootedLogFilesWithPermissionsAndOpener(
	openRoot rootDirectoryOpener,
	baseDir string,
	filePerm os.FileMode,
	leaves ...string,
) ([]*os.File, error) {
	for _, leaf := range leaves {
		if err := validateLogLeaf(leaf, false); err != nil {
			return nil, err
		}
	}
	if len(leaves) == 0 {
		return []*os.File{}, nil
	}
	if openRoot == nil {
		return nil, fmt.Errorf("開啟日誌 root %q: %w", baseDir, os.ErrInvalid)
	}

	root, err := openRoot(baseDir)
	if err != nil {
		return nil, fmt.Errorf("開啟日誌 root %q: %w", baseDir, err)
	}

	files := make([]*os.File, 0, len(leaves))
	for _, leaf := range leaves {
		info, lstatErr := root.Lstat(leaf)
		switch {
		case lstatErr == nil && info.Mode()&os.ModeSymlink != 0:
			err = fmt.Errorf("%w: 日誌 root %q 的 leaf %q 是 symlink", ErrUnsafeLogPath, baseDir, leaf)
		case lstatErr != nil && !errors.Is(lstatErr, os.ErrNotExist):
			err = fmt.Errorf("檢查日誌 root %q 的 leaf %q: %w", baseDir, leaf, lstatErr)
		}
		if err != nil {
			return nil, cleanupRootedLogFiles(root, baseDir, files, err)
		}

		file, openErr := root.OpenFile(
			leaf,
			os.O_CREATE|os.O_APPEND|os.O_WRONLY,
			filePerm,
		)
		if openErr != nil {
			err = fmt.Errorf("開啟日誌 root %q 的 leaf %q: %w", baseDir, leaf, openErr)
			return nil, cleanupRootedLogFiles(root, baseDir, files, err)
		}
		files = append(files, file)
	}

	if err := root.Close(); err != nil {
		return nil, errors.Join(
			fmt.Errorf("關閉日誌 root %q: %w", baseDir, err),
			closeRootedLogFiles(files),
		)
	}
	return files, nil
}

func cleanupRootedLogFiles(
	root rootedDirectory,
	baseDir string,
	files []*os.File,
	cause error,
) error {
	rootCloseErr := root.Close()
	if rootCloseErr != nil {
		rootCloseErr = fmt.Errorf("關閉日誌 root %q: %w", baseDir, rootCloseErr)
	}
	return errors.Join(cause, closeRootedLogFiles(files), rootCloseErr)
}

func closeRootedLogFiles(files []*os.File) error {
	closeErrs := make([]error, 0, len(files))
	for index := len(files) - 1; index >= 0; index-- {
		if err := files[index].Close(); err != nil {
			closeErrs = append(closeErrs, fmt.Errorf("關閉 partial 日誌檔案: %w", err))
		}
	}
	return errors.Join(closeErrs...)
}
