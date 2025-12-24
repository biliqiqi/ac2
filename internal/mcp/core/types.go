package core

import (
	"context"

	"github.com/biliqiqi/ac2/internal/pool"
)

// ToolCategory for organizing tools
type ToolCategory string

const (
	CategoryAgent   ToolCategory = "agent"         // Agent management
	CategoryComm    ToolCategory = "communication" // Inter-agent communication
	CategoryContext ToolCategory = "context"       // Shared context
	CategorySystem  ToolCategory = "system"        // System utilities
)

// UnifiedTool represents a complete tool definition with typed handler
type UnifiedTool[In, Out any] struct {
	Name        string
	Description string
	Category    ToolCategory
	Handler     ToolHandlerFor[In, Out]
	Middleware  []MiddlewareFunc
}

// ToolHandlerFor is a typed handler with automatic marshaling
type ToolHandlerFor[In, Out any] func(
	ctx *ExecutionContext,
	input In,
) (Out, error)

// ExecutionContext provides rich context to tool handlers
type ExecutionContext struct {
	Context       context.Context
	AgentPool     *pool.AgentPool
	Progress      ProgressReporter
	ProgressToken string // Progress token from request
}

// ProgressReporter for long-running operations
type ProgressReporter interface {
	Report(progress float64, message string) error
}

// MiddlewareFunc for cross-cutting concerns
type MiddlewareFunc func(*ExecutionContext, map[string]any) error

// RegisteredTool contains metadata about a registered tool
type RegisteredTool struct {
	Name        string
	Description string
	Category    ToolCategory
}
