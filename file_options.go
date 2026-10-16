package zlogger

import (
	"errors"
	"fmt"
	"os"
)

// ErrInvalidFilePermission indicates unsafe file output creation permissions.
var ErrInvalidFilePermission = errors.New("invalid log file permissions")

// FileOutputOption configures file creation permissions or split-only output observation.
//
// Options can only be constructed with this package's WithDirPerm, WithFilePerm, or WithOutputObserver.
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
	observer *FailureObserver
}

type outputObserverOption struct {
	observer *FailureObserver
}

// WithOutputObserver registers split output failure observation. Nil, zero-value, and duplicate observers are rejected.
// NewWithOptions and ConfigureWithOptions reject this option with ErrInvalidConfig.
func WithOutputObserver(observer *FailureObserver) FileOutputOption {
	return outputObserverOption{observer: observer}
}

func (o outputObserverOption) applyFileOutput(settings *fileOutputSettings) error {
	if o.observer == nil || o.observer.events == nil || settings.observer != nil {
		return fmt.Errorf("%w: observer is uninitialized or already registered", ErrInvalidConfig)
	}
	settings.observer = o.observer
	return nil
}

// WithDirPerm sets permission bits for new log directories.
//
// Mode must include owner rwx and exclude other-write and non-permission bits.
// The process umask can further restrict permissions. Existing directories are not changed.
func WithDirPerm(perm os.FileMode) FileOutputOption {
	return fileOutputOption{kind: fileOutputOptionDirPerm, perm: perm}
}

// WithFilePerm sets permission bits for new log files.
//
// Mode must include owner rw and exclude other-write and non-permission bits.
// The process umask can further restrict permissions. Existing files are not changed.
func WithFilePerm(perm os.FileMode) FileOutputOption {
	return fileOutputOption{kind: fileOutputOptionFilePerm, perm: perm}
}

func (o fileOutputOption) applyFileOutput(settings *fileOutputSettings) error {
	switch o.kind {
	case fileOutputOptionDirPerm:
		if err := validateFileOutputPermission("directory", o.perm, 0o700); err != nil {
			return err
		}
		settings.dirPerm = o.perm
	case fileOutputOptionFilePerm:
		if err := validateFileOutputPermission("file", o.perm, 0o600); err != nil {
			return err
		}
		settings.filePerm = o.perm
	default:
		return fmt.Errorf("%w: unknown option type %d", ErrInvalidFilePermission, o.kind)
	}
	return nil
}

func resolveFileOutputOptions(opts ...FileOutputOption) (fileOutputSettings, error) {
	settings := fileOutputSettings{
		dirPerm:  defaultLogDirMode,
		filePerm: defaultLogFileMode,
	}
	if err := validateFileOutputPermission("directory", settings.dirPerm, 0o700); err != nil {
		return fileOutputSettings{}, err
	}
	if err := validateFileOutputPermission("file", settings.filePerm, 0o600); err != nil {
		return fileOutputSettings{}, err
	}

	for index, opt := range opts {
		if opt == nil {
			return fileOutputSettings{}, fmt.Errorf(
				"%w: option %d must not be nil",
				ErrInvalidFilePermission,
				index+1,
			)
		}
		if err := opt.applyFileOutput(&settings); err != nil {
			return fileOutputSettings{}, fmt.Errorf("apply option %d: %w", index+1, err)
		}
	}

	return settings, nil
}

func validateFileOutputPermission(label string, perm, requiredOwner os.FileMode) error {
	if perm.Perm() != perm {
		return fmt.Errorf(
			"%w: %s mode %04o contains non-permission bits",
			ErrInvalidFilePermission,
			label,
			perm,
		)
	}
	if perm&requiredOwner != requiredOwner {
		return fmt.Errorf(
			"%w: %s mode %04o lacks required owner permissions %04o",
			ErrInvalidFilePermission,
			label,
			perm,
			requiredOwner,
		)
	}
	if perm&0o002 != 0 {
		return fmt.Errorf(
			"%w: %s mode %04o must not allow other-write",
			ErrInvalidFilePermission,
			label,
			perm,
		)
	}
	return nil
}
