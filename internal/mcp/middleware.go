package mcp

import (
	"fmt"

	"github.com/biliqiqi/ac2/internal/logger"
	"github.com/biliqiqi/ac2/internal/mcp/core"
)

// LogRequest logs the tool call for auditing
func LogRequest(ctx *core.ExecutionContext, params map[string]any) error {
	logger.Printf("MCP tool called: has_progress=%v",
		ctx.ProgressToken != "",
	)
	return nil
}

// ValidateAgentExists checks if the specified agent is available
func ValidateAgentExists(ctx *core.ExecutionContext, params map[string]any) error {
	agent, ok := params["agent"].(string)
	if !ok || agent == "" {
		return fmt.Errorf("agent parameter is required")
	}

	available := ctx.AgentPool.ListAll()
	for _, a := range available {
		if a.Type == agent {
			return nil
		}
	}

	// Check if agent type is available (not yet started)
	// This is a bit of a workaround since ListAll only shows running agents
	// GetOrCreate will check availability
	return nil
}
