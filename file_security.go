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

func secureLogPath(baseDir, leaf string) (string, error) {
	if err := validateLogLeaf(leaf, false); err != nil {
		return "", err
	}

	base := filepath.Clean(baseDir)
	target := filepath.Join(base, leaf)
	relative, err := filepath.Rel(base, target)
	if err != nil {
		return "", fmt.Errorf("計算日誌檔案相對路徑: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: leaf name %q 逸出基準目錄", ErrUnsafeLogPath, leaf)
	}

	return target, nil
}

func openSecureLogFile(baseDir, leaf string) (*os.File, error) {
	target, err := secureLogPath(baseDir, leaf)
	if err != nil {
		return nil, err
	}

	info, err := os.Lstat(target)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		return nil, fmt.Errorf("%w: 目標 %q 是 symlink", ErrUnsafeLogPath, target)
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return nil, fmt.Errorf("檢查日誌檔案 %q: %w", target, err)
	}

	//nolint:gosec // target 已通過 leaf containment 與既有 symlink 檢查。
	file, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, defaultLogFileMode)
	if err != nil {
		return nil, fmt.Errorf("開啟日誌檔案 %q: %w", target, err)
	}
	return file, nil
}
