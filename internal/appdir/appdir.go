package appdir

import (
	"os"
	"path/filepath"
)

// Base returns the directory containing the running executable.
// Falls back to the current working directory.
func Base() string {
	exe, err := os.Executable()
	if err != nil {
		wd, _ := os.Getwd()
		return wd
	}
	dir, err := filepath.Abs(filepath.Dir(exe))
	if err != nil {
		wd, _ := os.Getwd()
		return wd
	}
	return dir
}

// Join returns path under the executable directory.
func Join(elem ...string) string {
	return filepath.Join(append([]string{Base()}, elem...)...)
}

// ConfigPath is the default gateway.yaml beside the executable.
func ConfigPath() string {
	return Join("gateway.yaml")
}

// LogPath is the default log file beside the executable.
func LogPath() string {
	return Join("gateway.log")
}
