package main

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// getStopCmd returns the stop subcommand.
func getStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop a headless ac2 instance",
		RunE:  runStop,
	}
}

func runStop(cmd *cobra.Command, args []string) error {
	pid, err := readPIDFile(pidFile)
	if err != nil {
		return err
	}

	if !isProcessRunning(pid) {
		removePIDFile(pidFile)
		fmt.Println("No running ac2 process found.")
		return nil
	}

	if err := signalProcess(pid, syscall.SIGTERM); err != nil {
		return err
	}
	if err := waitForExit(pid, 5*time.Second); err != nil {
		_ = signalProcess(pid, syscall.SIGKILL)
		if err := waitForExit(pid, 2*time.Second); err != nil {
			return err
		}
	}

	removePIDFile(pidFile)
	fmt.Println("Stopped.")
	return nil
}

// signalProcess sends a signal to the target pid.
func signalProcess(pid int, sig syscall.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(sig)
}

// waitForExit waits until the process disappears or times out.
func waitForExit(pid int, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()

	for {
		if !isProcessRunning(pid) {
			return nil
		}
		select {
		case <-deadline.C:
			return fmt.Errorf("timeout waiting for process %d to exit", pid)
		case <-ticker.C:
		}
	}
}
