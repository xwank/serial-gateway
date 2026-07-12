package log

import (
	"os"
	"sync"
)

const defaultMaxLogBytes = 1 << 20 // 1 MiB

// rotateFile keeps log size under maxBytes by truncating to a backup file.
type rotateFile struct {
	path     string
	maxBytes int64
	mu       sync.Mutex
}

func newRotateFile(path string, maxBytes int64) *rotateFile {
	if maxBytes <= 0 {
		maxBytes = defaultMaxLogBytes
	}
	return &rotateFile{path: path, maxBytes: maxBytes}
}

func (r *rotateFile) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.rotateIfNeeded(int64(len(p))); err != nil {
		return 0, err
	}

	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	return f.Write(p)
}

func (r *rotateFile) rotateIfNeeded(incoming int64) error {
	fi, err := os.Stat(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if fi.Size()+incoming <= r.maxBytes {
		return nil
	}

	backup := r.path + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(r.path, backup); err != nil {
		// fallback: truncate
		return os.WriteFile(r.path, nil, 0o644)
	}
	return nil
}
