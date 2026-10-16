package zlogger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrUnsafeLogPath indicates that a log leaf name may escape its base directory or reference a symlink.
var ErrUnsafeLogPath = errors.New("unsafe log file path")

const (
	defaultLogDirMode  os.FileMode = 0o700
	defaultLogFileMode os.FileMode = 0o600
)

func validateLogLeaf(name string, allowEmpty bool) error {
	if name == "" {
		if allowEmpty {
			return nil
		}
		return fmt.Errorf("%w: leaf name must not be empty", ErrUnsafeLogPath)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("%w: leaf name %q must not identify a directory", ErrUnsafeLogPath, name)
	}
	if strings.IndexByte(name, 0) >= 0 {
		return fmt.Errorf("%w: leaf name must not contain NUL", ErrUnsafeLogPath)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%w: leaf name %q must not contain path separators", ErrUnsafeLogPath, name)
	}
	if filepath.IsAbs(name) || hasWindowsDrivePrefix(name) {
		return fmt.Errorf("%w: leaf name %q must not be an absolute or volume path", ErrUnsafeLogPath, name)
	}
	if filepath.Base(name) != name {
		return fmt.Errorf("%w: leaf name %q must not contain path semantics", ErrUnsafeLogPath, name)
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
		return nil, fmt.Errorf("open log root %q: %w", baseDir, os.ErrInvalid)
	}

	root, err := openRoot(baseDir)
	if err != nil {
		return nil, fmt.Errorf("open log root %q: %w", baseDir, err)
	}

	files := make([]*os.File, 0, len(leaves))
	for _, leaf := range leaves {
		info, lstatErr := root.Lstat(leaf)
		switch {
		case lstatErr == nil && info.Mode()&os.ModeSymlink != 0:
			err = fmt.Errorf("%w: log root %q leaf %q is a symlink", ErrUnsafeLogPath, baseDir, leaf)
		case lstatErr != nil && !errors.Is(lstatErr, os.ErrNotExist):
			err = fmt.Errorf("inspect log root %q leaf %q: %w", baseDir, leaf, lstatErr)
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
			err = fmt.Errorf("open log root %q leaf %q: %w", baseDir, leaf, openErr)
			return nil, cleanupRootedLogFiles(root, baseDir, files, err)
		}
		files = append(files, file)
	}

	if err := root.Close(); err != nil {
		return nil, errors.Join(
			fmt.Errorf("close log root %q: %w", baseDir, err),
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
		rootCloseErr = fmt.Errorf("close log root %q: %w", baseDir, rootCloseErr)
	}
	return errors.Join(cause, closeRootedLogFiles(files), rootCloseErr)
}

func closeRootedLogFiles(files []*os.File) error {
	closeErrs := make([]error, 0, len(files))
	for index := len(files) - 1; index >= 0; index-- {
		if err := files[index].Close(); err != nil {
			closeErrs = append(closeErrs, fmt.Errorf("close partially opened log files: %w", err))
		}
	}
	return errors.Join(closeErrs...)
}
