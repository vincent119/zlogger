package zlogger

import (
	"errors"
	"fmt"
	"os"
)

// ErrInvalidFilePermission 表示檔案輸出建立權限不符合安全契約。
var ErrInvalidFilePermission = errors.New("檔案輸出權限無效")

// FileOutputOption 設定新建日誌目錄與檔案的權限。
//
// Option 只能由本 package 提供的 WithDirPerm 與 WithFilePerm 建立。
type FileOutputOption interface {
	applyFileOutput(*fileOutputSettings) error
}

type fileOutputOptionKind uint8

const (
	fileOutputOptionDirPerm fileOutputOptionKind = iota
	fileOutputOptionFilePerm
)

type fileOutputOption struct {
	kind fileOutputOptionKind
	perm os.FileMode
}

type fileOutputSettings struct {
	dirPerm  os.FileMode
	filePerm os.FileMode
}

// WithDirPerm 設定新建日誌目錄的 permission bits。
//
// Mode 必須包含 owner rwx、不得包含 other-write 或非 permission bits。
// 實際權限仍可能被 process umask 限縮，且不會改寫既有目錄。
func WithDirPerm(perm os.FileMode) FileOutputOption {
	return fileOutputOption{kind: fileOutputOptionDirPerm, perm: perm}
}

// WithFilePerm 設定新建日誌檔案的 permission bits。
//
// Mode 必須包含 owner rw、不得包含 other-write 或非 permission bits。
// 實際權限仍可能被 process umask 限縮，且不會改寫既有檔案。
func WithFilePerm(perm os.FileMode) FileOutputOption {
	return fileOutputOption{kind: fileOutputOptionFilePerm, perm: perm}
}

func (o fileOutputOption) applyFileOutput(settings *fileOutputSettings) error {
	switch o.kind {
	case fileOutputOptionDirPerm:
		if err := validateFileOutputPermission("目錄", o.perm, 0o700); err != nil {
			return err
		}
		settings.dirPerm = o.perm
	case fileOutputOptionFilePerm:
		if err := validateFileOutputPermission("檔案", o.perm, 0o600); err != nil {
			return err
		}
		settings.filePerm = o.perm
	default:
		return fmt.Errorf("%w: 未知 option 類型 %d", ErrInvalidFilePermission, o.kind)
	}
	return nil
}

func resolveFileOutputOptions(opts ...FileOutputOption) (fileOutputSettings, error) {
	settings := fileOutputSettings{
		dirPerm:  defaultLogDirMode,
		filePerm: defaultLogFileMode,
	}
	if err := validateFileOutputPermission("目錄", settings.dirPerm, 0o700); err != nil {
		return fileOutputSettings{}, err
	}
	if err := validateFileOutputPermission("檔案", settings.filePerm, 0o600); err != nil {
		return fileOutputSettings{}, err
	}

	for index, opt := range opts {
		if opt == nil {
			return fileOutputSettings{}, fmt.Errorf(
				"%w: 第 %d 個 option 不可為 nil",
				ErrInvalidFilePermission,
				index+1,
			)
		}
		if err := opt.applyFileOutput(&settings); err != nil {
			return fileOutputSettings{}, fmt.Errorf("套用第 %d 個 option: %w", index+1, err)
		}
	}

	return settings, nil
}

func validateFileOutputPermission(label string, perm, requiredOwner os.FileMode) error {
	if perm.Perm() != perm {
		return fmt.Errorf(
			"%w: %s mode %04o 含非 permission bits",
			ErrInvalidFilePermission,
			label,
			perm,
		)
	}
	if perm&requiredOwner != requiredOwner {
		return fmt.Errorf(
			"%w: %s mode %04o 缺少 owner 必要權限 %04o",
			ErrInvalidFilePermission,
			label,
			perm,
			requiredOwner,
		)
	}
	if perm&0o002 != 0 {
		return fmt.Errorf(
			"%w: %s mode %04o 不得允許 other-write",
			ErrInvalidFilePermission,
			label,
			perm,
		)
	}
	return nil
}
