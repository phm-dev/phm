package pkg

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// DefaultLockDir is the default directory for the lock file
const DefaultLockDir = "/opt/php"

// AcquireLock creates an exclusive lock file to prevent concurrent PHM operations.
// lockDir specifies the directory for the lock file (typically the install prefix).
// Returns a release function that must be called (typically via defer) to remove the lock.
func AcquireLock(lockDir string) (func(), error) {
	if lockDir == "" {
		lockDir = DefaultLockDir
	}
	lockPath := filepath.Join(lockDir, ".phm.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("another PHM process is running, lock file: %s", lockPath)
	}
	release := func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
		os.Remove(lockPath)
	}
	return release, nil
}
