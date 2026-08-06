package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/BCPathway/devex-cli/internal/env"
	"golang.org/x/term"
)

// ExitCodeCI is the exit code used when a prompt is required but we are in CI mode.
const ExitCodeCI = 2

// Prompt displays a labelled input prompt with an optional default value.
func Prompt(r *bufio.Reader, label, defaultVal string) string {
	if env.IsCI() {
		if defaultVal != "" {
			return defaultVal
		}
		fmt.Fprintf(os.Stderr, "Error: interactive prompt %q requires input but running in CI mode.\n", label)
		os.Exit(ExitCodeCI)
	}

	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("  %s: ", label)
	}

	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

// ConfirmOverwrite asks the user whether to overwrite an existing file.
func ConfirmOverwrite(path string) (bool, error) {
	if env.IsCI() {
		fmt.Fprintf(os.Stderr, "Error: overwrite confirmation required for %s but running in CI mode. Use --force or --yes.\n", path)
		os.Exit(ExitCodeCI)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s already exists. Overwrite? [y/N]: ", path)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes", nil
}

// PromptConfirmation asks for a simple yes/no confirmation.
func PromptConfirmation(promptText string) (bool, error) {
	if env.IsCI() {
		fmt.Fprintf(os.Stderr, "Error: confirmation required %q but running in CI mode. Use --yes.\n", promptText)
		os.Exit(ExitCodeCI)
	}

	fmt.Print(promptText)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes", nil
}

// PromptConflictAction asks for an action when files conflict.
func PromptConflictAction(path string, canMerge bool) (string, error) {
	if env.IsCI() {
		fmt.Fprintf(os.Stderr, "Error: conflict action required for %s but running in CI mode. Use --force.\n", path)
		os.Exit(ExitCodeCI)
	}

	reader := bufio.NewReader(os.Stdin)
	if canMerge {
		fmt.Printf("%s already exists and has locked splits. [M]erge (preserve locked) / [o]verwrite / [c]ancel [M/o/c]: ", path)
	} else {
		fmt.Printf("%s already exists. [O]verwrite / [c]ancel [O/c]: ", path)
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(strings.ToLower(line))

	if canMerge {
		switch line {
		case "", "m", "merge":
			return "merge", nil
		case "o", "overwrite":
			return "overwrite", nil
		default:
			return "cancel", nil
		}
	} else {
		switch line {
		case "", "o", "overwrite", "y", "yes":
			return "overwrite", nil
		default:
			return "cancel", nil
		}
	}
}

// PromptPassword securely prompts for a password or key.
func PromptPassword(promptText string) (string, error) {
	if env.IsCI() {
		fmt.Fprintf(os.Stderr, "Error: interactive password/key input required but running in CI mode. Use flags (e.g. --key) or env vars.\n")
		os.Exit(ExitCodeCI)
	}

	fmt.Print(promptText)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", err
		}
		return string(bytePassword), nil
	}

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// StartSpinner starts a loading spinner, or falls back to plain log if CI mode is true.
func StartSpinner(msg string) {
	if env.IsCI() {
		fmt.Println(msg + "...")
	} else {
		// Mock spinner fallback for future implementations
		fmt.Println(msg + "...")
	}
}

// StopSpinner stops a spinner, replacing it with a DONE message if in CI mode.
func StopSpinner(msg string) {
	if env.IsCI() {
		fmt.Println("[DONE] " + msg)
	}
}
