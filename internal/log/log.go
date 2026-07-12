package log

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

var (
	mu     sync.Mutex
	level  = LevelInfo
	logger *log.Logger
)

// Level defines log severity.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func parseLevel(name string) Level {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func logf(lv Level, tag string, format string, args ...interface{}) {
	if lv < level {
		return
	}
	mu.Lock()
	l := logger
	mu.Unlock()
	if l == nil {
		return
	}

	lvName := "INFO"
	switch lv {
	case LevelDebug:
		lvName = "DEBUG"
	case LevelWarn:
		lvName = "WARN"
	case LevelError:
		lvName = "ERROR"
	}

	msg := fmt.Sprintf(format, args...)
	if tag != "" {
		_ = l.Output(3, fmt.Sprintf("[%s] [%s] %s", lvName, tag, msg))
	} else {
		_ = l.Output(3, fmt.Sprintf("[%s] %s", lvName, msg))
	}
}

// Debug logs debug message.
func Debug(tag, format string, args ...interface{}) { logf(LevelDebug, tag, format, args...) }

// Info logs info message.
func Info(tag, format string, args ...interface{}) { logf(LevelInfo, tag, format, args...) }

// Warn logs warning message.
func Warn(tag, format string, args ...interface{}) { logf(LevelWarn, tag, format, args...) }

// Error logs error message.
func Error(tag, format string, args ...interface{}) { logf(LevelError, tag, format, args...) }

// SlotTag returns a standard slot log tag.
func SlotTag(slotID int) string {
	return fmt.Sprintf("slot=%d", slotID)
}
