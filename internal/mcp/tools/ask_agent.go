package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/biliqiqi/ac2/internal/mcp/core"
)

// AskAgentInput defines the input for asking a specific agent
// It's similar to CallAgentInput but without the 'agent' field since that's implied by the tool name
type AskAgentInput struct {
	Message string `json:"message"`
	Timeout int    `json:"timeout,omitempty"`
}

// CallAgentOutput contains the agent's response
type CallAgentOutput struct {
	Response string  `json:"response"`
	Duration float64 `json:"duration"`
}

// String formats the output for display
func (o CallAgentOutput) String() string {
	return o.Response
}

// NewAskAgentTool creates a UnifiedTool for a specific agent
func NewAskAgentTool(agentName string, description string) *core.UnifiedTool[AskAgentInput, CallAgentOutput] {
	return &core.UnifiedTool[AskAgentInput, CallAgentOutput]{
		Name:        fmt.Sprintf("ask-%s", agentName),
		Description: description,
		Category:    core.CategoryAgent,
		Handler: func(ctx *core.ExecutionContext, input AskAgentInput) (CallAgentOutput, error) {
			// Reuse the handleCallAgent logic by constructing a CallAgentInput
			// This avoids code duplication
			return handleAskAgent(ctx, input, agentName)
		},
	}
}

func handleAskAgent(
	ctx *core.ExecutionContext,
	input AskAgentInput,
	agentName string,
) (CallAgentOutput, error) {
	start := time.Now()

	// Report progress: starting
	if ctx.ProgressToken != "" {
		_ = ctx.Progress.Report(0.1, fmt.Sprintf("Preparing %s in non-interactive mode...", agentName))
	}

	// Report progress: ready
	if ctx.ProgressToken != "" {
		_ = ctx.Progress.Report(0.3, fmt.Sprintf("Running %s command...", agentName))
	}

	// Setup timeout
	timeout := 30 * time.Second
	if input.Timeout > 0 {
		timeout = time.Duration(input.Timeout) * time.Second
	}

	callCtx, cancel := context.WithTimeout(ctx.Context, timeout)
	defer cancel()

	// Report progress: waiting for response
	if ctx.ProgressToken != "" {
		_ = ctx.Progress.Report(0.5, fmt.Sprintf("Waiting for response from %s...", agentName))
	}

	// Call agent in non-interactive mode
	response, err := ctx.AgentPool.CallNonInteractive(callCtx, agentName, input.Message)
	if err != nil {
		return CallAgentOutput{}, fmt.Errorf("agent call failed: %w", err)
	}

	// Report progress: complete
	if ctx.ProgressToken != "" {
		_ = ctx.Progress.Report(1.0, "Complete")
	}

	return CallAgentOutput{
		Response: response,
		Duration: time.Since(start).Seconds(),
	}, nil
}
