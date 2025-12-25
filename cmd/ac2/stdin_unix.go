//go:build unix

package main

import (
	"os"
	"syscall"
)

func flushStdin() {
	// Set non-blocking
	_ = syscall.SetNonblock(int(os.Stdin.Fd()), true)
	defer func() {
		// Restore blocking mode
		_ = syscall.SetNonblock(int(os.Stdin.Fd()), false)
	}()

	buf := make([]byte, 1024)
	for {
		// Read until error (EAGAIN/EWOULDBLOCK) or EOF
		n, err := os.Stdin.Read(buf)
		if n <= 0 || err != nil {
			break
		}
	}
}
