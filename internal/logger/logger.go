package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	defaultLogger *log.Logger
	mu            sync.Mutex
	logFile       *os.File
)

func init() {
	// Default to discarding logs until Init is called
	defaultLogger = log.New(io.Discard, "", log.LstdFlags)
}

// Init initializes the logger to write to a file.
// If filename is empty, it uses "ac2.log" in the current directory.
func Init(filename string) error {
	mu.Lock()
	defer mu.Unlock()

	if filename == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		filename = filepath.Join(home, ".ac2.log")
	}

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	if logFile != nil {
		_ = logFile.Close()
	}
	logFile = f
	defaultLogger = log.New(f, "[ac2] ", log.LstdFlags|log.Lshortfile)
	return nil
}

// Close closes the log file.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}

// Printf logs a formatted string.
func Printf(format string, v ...interface{}) {
	_ = defaultLogger.Output(2, fmt.Sprintf(format, v...))
}

// Println logs a line.
func Println(v ...interface{}) {
	_ = defaultLogger.Output(2, fmt.Sprintln(v...))
}

// Errorf logs a formatted error string.
func Errorf(format string, v ...interface{}) {
	_ = defaultLogger.Output(2, fmt.Sprintf("ERROR: "+format, v...))
}
