//go:build !windows

package aigo

import (
	"os"
	"syscall"
)

// fileLockShared acquires a shared (read) file lock for cross-process synchronization.
// Multiple readers can hold a shared lock concurrently.
func fileLockShared(path string) (*os.File, error) {
	return flockWith(path, syscall.LOCK_SH)
}

// fileLockExclusive acquires an exclusive (write) file lock for cross-process synchronization.
// Only one writer can hold an exclusive lock; blocks all other readers and writers.
func fileLockExclusive(path string) (*os.File, error) {
	return flockWith(path, syscall.LOCK_EX)
}

func flockWith(path string, how int) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// fileUnlock releases the file lock and closes the file.
func fileUnlock(f *os.File) {
	if f != nil {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}
}
