package log

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// InitOptions configures logging sinks.
type InitOptions struct {
	Level    string
	FilePath string
	MaxBytes int64
	Console  bool
}

// Init configures global logging (stdout + optional file).
func Init(levelName, filePath string) error {
	return InitWithOptions(InitOptions{
		Level:    levelName,
		FilePath: filePath,
		MaxBytes: defaultMaxLogBytes,
		Console:  true,
	})
}

// InitWithOptions configures logging with rotation and optional console output.
func InitWithOptions(opts InitOptions) error {
	mu.Lock()
	defer mu.Unlock()

	level = parseLevel(opts.Level)
	writers := make([]io.Writer, 0, 2)

	if opts.Console {
		writers = append(writers, os.Stdout)
	}

	if opts.FilePath != "" {
		dir := filepath.Dir(opts.FilePath)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
		writers = append(writers, newRotateFile(opts.FilePath, opts.MaxBytes))
	}

	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	logger = log.New(io.MultiWriter(writers...), "", log.LstdFlags)
	return nil
}
