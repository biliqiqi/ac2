//go:build windows

package main

import (
	"os"
	"time"
)

func flushStdin() {
	// Windows version: use a simple time-based approach
	// Since we can't easily set non-blocking mode on Windows without CGO,
	// we use a goroutine with timeout to drain stdin

	done := make(chan struct{})
	buf := make([]byte, 1024)

	go func() {
		defer close(done)
		for {
			// Try to read, will block if no data
			n, err := os.Stdin.Read(buf)
			if n <= 0 || err != nil {
				return
			}
		}
	}()

	// Wait a short time for any pending input
	select {
	case <-done:
		// Stdin was closed or errored
	case <-time.After(50 * time.Millisecond):
		// Timeout - assume stdin is empty or blocking
	}
}
