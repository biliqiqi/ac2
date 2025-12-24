package prompts

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/biliqiqi/ac2/internal/detector"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterBuiltin registers slash-command prompts for the MCP server.
func RegisterBuiltin(server *sdkmcp.Server, agents []detector.AgentInfo) {
	registerAgentPrompts(server, agents)
}

// CountBuiltin returns the number of prompts registered by RegisterBuiltin.
func CountBuiltin(agents []detector.AgentInfo) int {
	return len(agents)
}

func registerAgentPrompts(server *sdkmcp.Server, agents []detector.AgentInfo) {
	for _, agent := range agents {
		agentName := string(agent.Type)
		server.AddPrompt(&sdkmcp.Prompt{
			Name:        fmt.Sprintf("ask-%s", agentName),
			Title:       fmt.Sprintf("ask-%s", agentName),
			Description: fmt.Sprintf("Directly ask %s a question or give it a task.", agent.Name),
			Arguments: []*sdkmcp.PromptArgument{
				{Name: "message", Description: "Message to send to the agent.", Required: true},
				{Name: "timeout", Description: "Optional timeout in seconds."},
			},
		}, toolPromptHandler(fmt.Sprintf("ask-%s", agentName), func(args map[string]string) (map[string]any, error) {
			payload := map[string]any{
				"message": args["message"],
			}
			if timeout, ok := parseOptionalInt(args["timeout"]); ok {
				payload["timeout"] = timeout
			}
			return payload, nil
		}))
	}
}

func toolPromptHandler(
	toolName string,
	buildArgs func(map[string]string) (map[string]any, error),
) sdkmcp.PromptHandler {
	return func(_ context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		args, err := buildArgs(req.Params.Arguments)
		if err != nil {
			return nil, err
		}

		payload, err := json.MarshalIndent(args, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to encode prompt arguments: %w", err)
		}

		text := fmt.Sprintf("Use the MCP tool `%s` with these arguments:\n```json\n%s\n```", toolName, payload)
		return &sdkmcp.GetPromptResult{
			Description: fmt.Sprintf("Slash command for %s", toolName),
			Messages: []*sdkmcp.PromptMessage{
				{
					Role:    "user",
					Content: &sdkmcp.TextContent{Text: text},
				},
			},
		}, nil
	}
}

func parseOptionalInt(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}
