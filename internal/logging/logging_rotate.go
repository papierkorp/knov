// Package logging provides a size-based rotating file writer.
package logging

import (
	"fmt"
	"os"
	"sync"
)

type rotatingWriter struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	maxBytes int64
	maxFiles int
	size     int64
}

func newRotatingWriter(path string, maxMB, maxFiles int) (*rotatingWriter, error) {
	rw := &rotatingWriter{
		path:     path,
		maxBytes: int64(maxMB) * 1024 * 1024,
		maxFiles: maxFiles,
	}
	if err := rw.open(); err != nil {
		return nil, err
	}
	return rw, nil
}

func (rw *rotatingWriter) open() error {
	f, err := os.OpenFile(rw.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	rw.file = f
	rw.size = info.Size()
	return nil
}

func (rw *rotatingWriter) Write(p []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.size+int64(len(p)) > rw.maxBytes {
		rw.rotate()
	}

	n, err := rw.file.Write(p)
	rw.size += int64(n)
	return n, err
}

func (rw *rotatingWriter) rotate() {
	rw.file.Close()

	// shift: app.log -> app.log.1 -> app.log.2 -> ... -> oldest deleted
	for i := rw.maxFiles - 1; i >= 1; i-- {
		older := fmt.Sprintf("%s.%d", rw.path, i)
		newer := fmt.Sprintf("%s.%d", rw.path, i-1)
		if i == 1 {
			newer = rw.path
		}
		os.Rename(newer, older)
	}

	if err := rw.open(); err != nil {
		// nothing useful we can do here without risking infinite recursion
		_ = err
	}
}
