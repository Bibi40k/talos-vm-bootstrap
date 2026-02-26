//go:build windows
// +build windows

package cli

// drainStdin is a no-op on Windows.
func drainStdin() {}

// restoreTTYOnExit is a no-op on Windows.
func restoreTTYOnExit() {}
