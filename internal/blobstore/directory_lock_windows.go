//go:build windows

package blobstore

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

type directoryLock struct {
	file       *os.File
	overlapped windows.Overlapped
}

func acquireDirectoryLock(path string) (*directoryLock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	lock := &directoryLock{file: file}
	err = windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, &lock.overlapped,
	)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("directory is already used by another blob store: %w", err)
	}
	return lock, nil
}

func (l *directoryLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := windows.UnlockFileEx(windows.Handle(l.file.Fd()), 0, 1, 0, &l.overlapped)
	return errors.Join(err, l.file.Close())
}
