// Package logger provides a minimal structured logger with debug/verbose
// support. It is intentionally simple — swap in zerolog or slog when the
// project graduates beyond scaffolding.
package logger

import (
	"fmt"
	"os"
)

var debugEnabled bool

// Init configures the logger. Call once during CLI bootstrap.
func Init(verbose bool) {
	debugEnabled = verbose
}

// Debug prints a message only when verbose mode is enabled.
func Debug(format string, args ...any) {
	if !debugEnabled {
		return
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
}

// Info prints an informational message to stderr (keeps stdout clean for
// structured output).
func Info(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Warn prints a warning message to stderr.
func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[WARN]  "+format+"\n", args...)
}

// Error prints an error message to stderr.
func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[ERROR] "+format+"\n", args...)
}
