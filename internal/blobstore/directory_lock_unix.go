//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

package blobstore

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type directoryLock struct {
	file *os.File
}

func acquireDirectoryLock(path string) (*directoryLock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("directory is already used by another blob store: %w", err)
	}
	return &directoryLock{file: file}, nil
}

func (l *directoryLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return errors.Join(unix.Flock(int(l.file.Fd()), unix.LOCK_UN), l.file.Close())
}
