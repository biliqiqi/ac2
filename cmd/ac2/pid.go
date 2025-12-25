package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

const defaultPIDFile = "ac2.pid"

// readPIDFile reads a pid file and returns the parsed pid.
func readPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("pid file not found: %s", path)
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid pid in file %s", path)
	}
	return pid, nil
}

// writePIDFile writes the current process pid to the path.
func writePIDFile(path string) error {
	if pid, err := readPIDFile(path); err == nil {
		if isProcessRunning(pid) {
			return fmt.Errorf("pid file already exists and process %d is running", pid)
		}
	}

	pid := os.Getpid()
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0644)
}

// removePIDFile deletes the pid file if it exists.
func removePIDFile(path string) {
	_ = os.Remove(path)
}

// isProcessRunning checks if a process exists and is running.
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	return errors.Is(err, syscall.EPERM)
}
