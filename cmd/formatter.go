package cmd

import (
	"encoding/json"
	"fmt"
	"io"
)

// OutputFormatter defines a standard interface for commands to render output.
type OutputFormatter interface {
	Format(w io.Writer, data any) error
}

// JSONFormatter handles JSON output formatting.
type JSONFormatter struct{}

func (f *JSONFormatter) Format(w io.Writer, data any) error {
	return f.encode(w, data)
}

func (f *JSONFormatter) encode(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}

// TerminalFormatter uses a provided callback for legacy terminal rendering.
// It allows commands to retain their existing textFormatter functions.
type TerminalFormatter struct {
	textFormatter func()
}

func (f *TerminalFormatter) Format(w io.Writer, data any) error {
	if f.textFormatter != nil {
		f.textFormatter()
	}
	return nil
}
