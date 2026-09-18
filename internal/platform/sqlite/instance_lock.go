package sqlite

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type InstanceLock struct{ file *os.File }

func AcquireInstanceLock(directory string) (*InstanceLock, error) {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(directory, ".instance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("data directory is already locked: %w", err)
	}
	return &InstanceLock{file: file}, nil
}

func (l *InstanceLock) Close() error {
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}
