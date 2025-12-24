package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/biliqiqi/ac2/internal/mcp/core"
	"github.com/biliqiqi/ac2/internal/pool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolRegistry manages tool registration and lifecycle
type ToolRegistry struct {
	server    *sdkmcp.Server
	tools     map[string]*core.RegisteredTool
	agentPool *pool.AgentPool
	mu        sync.RWMutex
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(server *sdkmcp.Server, agentPool *pool.AgentPool) *ToolRegistry {
	return &ToolRegistry{
		server:    server,
		tools:     make(map[string]*core.RegisteredTool),
		agentPool: agentPool,
	}
}

// RegisterTool registers a typed tool with automatic schema inference
func RegisterTool[In, Out any](
	r *ToolRegistry,
	tool *core.UnifiedTool[In, Out],
) error {
	// Wrap our handler to inject ExecutionContext
	wrappedHandler := sdkmcp.ToolHandlerFor[In, Out](func(
		ctx context.Context,
		req *sdkmcp.CallToolRequest,
		input In,
	) (*sdkmcp.CallToolResult, Out, error) {
		// Extract progress token if available
		var progressToken string
		if req.Params.Meta != nil {
			if token, ok := req.Params.Meta["progressToken"].(string); ok {
				progressToken = token
			}
		}

		// Create execution context
		execCtx := &core.ExecutionContext{
			Context:       ctx,
			AgentPool:     r.agentPool,
			Progress:      core.NewProgressReporter(progressToken),
			ProgressToken: progressToken,
		}

		// Apply middleware
		if len(tool.Middleware) > 0 {
			// Convert input to map for middleware
			inputBytes, err := json.Marshal(input)
			if err != nil {
				var zero Out
				return nil, zero, fmt.Errorf("failed to marshal input for middleware: %w", err)
			}
			var inputMap map[string]any
			if err := json.Unmarshal(inputBytes, &inputMap); err != nil {
				var zero Out
				return nil, zero, fmt.Errorf("failed to unmarshal input for middleware: %w", err)
			}

			for _, mw := range tool.Middleware {
				if err := mw(execCtx, inputMap); err != nil {
					var zero Out
					return nil, zero, err
				}
			}
		}

		// Call the actual handler
		output, err := tool.Handler(execCtx, input)
		if err != nil {
			// Return error result
			return &sdkmcp.CallToolResult{
				IsError: true,
				Content: []sdkmcp.Content{
					&sdkmcp.TextContent{
						Text: fmt.Sprintf("Error: %v", err),
					},
				},
			}, output, nil
		}

		// Convert output to string for TextContent
		// This ensures Claude Code displays it as text output
		str := fmt.Sprintf("%v", output)

		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{
				&sdkmcp.TextContent{
					Text: str,
				},
			},
		}, output, nil
	})

	// Register with SDK
	sdkmcp.AddTool(r.server, &sdkmcp.Tool{
		Name:        tool.Name,
		Description: tool.Description,
	}, wrappedHandler)

	// Track registration
	r.mu.Lock()
	r.tools[tool.Name] = &core.RegisteredTool{
		Name:        tool.Name,
		Description: tool.Description,
		Category:    tool.Category,
	}
	r.mu.Unlock()

	return nil
}

// List returns all registered tools
func (r *ToolRegistry) List() []core.RegisteredTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]core.RegisteredTool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, *tool)
	}
	return result
}

// Get returns a specific tool by name
func (r *ToolRegistry) Get(name string) (*core.RegisteredTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}
