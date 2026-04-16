//go:build windows

package aigo

import "os"

// fileLockShared is a no-op on Windows (mutex still provides in-process safety).
func fileLockShared(_ string) (*os.File, error) { return nil, nil }

// fileLockExclusive is a no-op on Windows (mutex still provides in-process safety).
func fileLockExclusive(_ string) (*os.File, error) { return nil, nil }

// fileUnlock is a no-op on Windows.
func fileUnlock(_ *os.File) {}
