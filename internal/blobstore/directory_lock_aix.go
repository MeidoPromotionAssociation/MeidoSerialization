//go:build aix

package blobstore

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

// AIX exposes fcntl record locks rather than flock through x/sys. Record
// locks are process-scoped, so keep a small in-process guard as well: closing
// a failed second acquisition must not release the first acquisition's lock.
var aixDirectoryLocks struct {
	sync.Mutex
	paths map[string]struct{}
}

type directoryLock struct {
	file *os.File
	path string
}

func acquireDirectoryLock(path string) (*directoryLock, error) {
	aixDirectoryLocks.Lock()
	if aixDirectoryLocks.paths == nil {
		aixDirectoryLocks.paths = make(map[string]struct{})
	}
	if _, exists := aixDirectoryLocks.paths[path]; exists {
		aixDirectoryLocks.Unlock()
		return nil, fmt.Errorf("directory is already used by another blob store")
	}
	aixDirectoryLocks.paths[path] = struct{}{}
	aixDirectoryLocks.Unlock()

	release := func() {
		aixDirectoryLocks.Lock()
		delete(aixDirectoryLocks.paths, path)
		aixDirectoryLocks.Unlock()
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		release()
		return nil, err
	}
	flock := unix.Flock_t{Type: int16(unix.F_WRLCK), Whence: 0, Start: 0, Len: 1}
	if err := unix.FcntlFlock(file.Fd(), unix.F_SETLK, &flock); err != nil {
		_ = file.Close()
		release()
		return nil, fmt.Errorf("directory is already used by another blob store: %w", err)
	}
	return &directoryLock{file: file, path: path}, nil
}

func (l *directoryLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	flock := unix.Flock_t{Type: int16(unix.F_UNLCK), Whence: 0, Start: 0, Len: 1}
	unlockErr := unix.FcntlFlock(l.file.Fd(), unix.F_SETLK, &flock)
	closeErr := l.file.Close()
	aixDirectoryLocks.Lock()
	delete(aixDirectoryLocks.paths, l.path)
	aixDirectoryLocks.Unlock()
	return errors.Join(unlockErr, closeErr)
}
