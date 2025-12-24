package core

import "github.com/biliqiqi/ac2/internal/logger"

// progressReporter implements ProgressReporter interface
type progressReporter struct {
	token string
}

// NewProgressReporter creates a new progress reporter
func NewProgressReporter(token string) ProgressReporter {
	return &progressReporter{
		token: token,
	}
}

// Report sends a progress notification
func (p *progressReporter) Report(progress float64, message string) error {
	if p.token == "" {
		// No progress token, skip reporting
		return nil
	}

	// For now, just log the progress
	// TODO: Send actual MCP progress notification when integrated with SDK
	logger.Printf("Progress [token=%s]: %.1f%% - %s", p.token, progress*100, message)

	return nil
}
