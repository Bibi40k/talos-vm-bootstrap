//go:build !windows
// +build !windows

package cli

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

// drainStdin discards pending stdin bytes left by interactive TTY rendering
// (for example CPR responses), so they don't leak into the next prompt.
func drainStdin() {
	fd := int(os.Stdin.Fd())
	if err := syscall.SetNonblock(fd, true); err != nil {
		stdinReader.Reset(os.Stdin)
		return
	}
	defer func() {
		_ = syscall.SetNonblock(fd, false)
		stdinReader.Reset(os.Stdin)
	}()

	buf := make([]byte, 256)
	deadline := time.Now().Add(80 * time.Millisecond)
	for time.Now().Before(deadline) {
		n, err := syscall.Read(fd, buf)
		if n > 0 {
			deadline = time.Now().Add(80 * time.Millisecond)
			continue
		}
		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		break
	}
}

// restoreTTYOnExit best-effort restores terminal state before abrupt exit
// from signal handlers (which skip defers).
func restoreTTYOnExit() {
	fd := int(os.Stdin.Fd())
	_ = syscall.SetNonblock(fd, false)
	stdinReader.Reset(os.Stdin)

	cmd := exec.Command("stty", "sane")
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}
