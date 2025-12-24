package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/biliqiqi/ac2/internal/detector"
	"github.com/biliqiqi/ac2/internal/logger"
	"github.com/biliqiqi/ac2/internal/mcp/prompts"
	"github.com/biliqiqi/ac2/internal/mcp/tools"
	"github.com/biliqiqi/ac2/internal/pool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the official SDK server with ac2-specific features
type Server struct {
	sdk       *sdkmcp.Server
	registry  *ToolRegistry
	agentPool *pool.AgentPool
	config    *ServerConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Name        string
	Version     string
	LogRequests bool
}

// NewServer creates a new MCP server with the given agent pool
func NewServer(agentPool *pool.AgentPool) *Server {
	config := &ServerConfig{
		Name:    "ac2",
		Version: "0.1.0",
	}

	// Create SDK server
	sdkServer := sdkmcp.NewServer(
		&sdkmcp.Implementation{
			Name:    config.Name,
			Version: config.Version,
		},
		nil,
	)

	// Create registry
	registry := NewToolRegistry(sdkServer, agentPool)

	server := &Server{
		sdk:       sdkServer,
		registry:  registry,
		agentPool: agentPool,
		config:    config,
	}

	// Register built-in tools
	server.registerBuiltinTools()
	server.registerBuiltinPrompts()

	return server
}

// EnableRequestLogs enables request logging
func (s *Server) EnableRequestLogs(enabled bool) {
	s.config.LogRequests = enabled
}

// GetSDKServer returns the underlying SDK server for stdio mode
func (s *Server) GetSDKServer() *sdkmcp.Server {
	return s.sdk
}

// registerBuiltinTools registers all built-in MCP tools
func (s *Server) registerBuiltinTools() {
	logger.Println("Registering built-in MCP tools...")

	// Dynamically register ask-{agent} tools for all known agents
	logger.Println("Registering dynamic agent tools...")
	det := detector.New()
	allAgents := det.GetAll()

	for _, agent := range allAgents {
		toolName := fmt.Sprintf("ask-%s", agent.Type)
		description := fmt.Sprintf("Directly ask %s (%s) a question or give a task. "+
			"The agent will be started automatically if not running. "+
			"Use this for quick, one-off interactions with %s.",
			agent.Name, agent.Command, agent.Name)

		tool := tools.NewAskAgentTool(string(agent.Type), description)
		if err := RegisterTool(s.registry, tool); err != nil {
			logger.Printf("Warning: Failed to register %s: %v", toolName, err)
		} else {
			logger.Printf("Registered: %s", toolName)
		}
	}

	logger.Printf("Successfully registered %d MCP tools", len(s.registry.tools))
}

// registerBuiltinPrompts registers MCP prompts for custom slash commands
func (s *Server) registerBuiltinPrompts() {
	logger.Println("Registering built-in MCP prompts...")

	det := detector.New()
	allAgents := det.GetAll()

	prompts.RegisterBuiltin(s.sdk, allAgents)

	logger.Printf("Registered %d MCP prompts", prompts.CountBuiltin(allAgents))
}

// ListenHTTP starts HTTP/SSE transports
func (s *Server) ListenHTTP(addr string) error {
	logger.Println("Setting up MCP HTTP handlers...")

	mux := http.NewServeMux()

	// SSE endpoint
	logger.Println("Creating SSE handler...")
	sseHandler := sdkmcp.NewSSEHandler(func(r *http.Request) *sdkmcp.Server {
		return s.sdk
	}, nil)
	mux.Handle("/mcp/sse", sseHandler)
	logger.Println("SSE handler registered")

	// Streamable HTTP endpoint
	logger.Println("Creating streamable HTTP handler...")
	streamHandler := sdkmcp.NewStreamableHTTPHandler(func(r *http.Request) *sdkmcp.Server {
		return s.sdk
	}, nil)
	mux.Handle("/mcp", streamHandler)
	logger.Println("Streamable HTTP handler registered")

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"tools":  len(s.registry.tools),
		})
	})

	logger.Printf("MCP Server listening on %s", addr)
	logger.Printf("MCP endpoints:")
	logger.Printf("  - POST %s/mcp", addr)
	logger.Printf("  - GET  %s/mcp/sse (SSE)", addr)

	return http.ListenAndServe(addr, mux)
}

// ListenUnix starts Unix socket transport
func (s *Server) ListenUnix(socketPath string) error {
	// Remove existing socket file
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket: %w", err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}()

	logger.Printf("MCP Server listening on %s", socketPath)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if s.config.LogRequests {
				logger.Printf("MCP: Accept error: %v", err)
			}
			continue
		}

		if s.config.LogRequests {
			logger.Println("MCP: New connection accepted")
		}
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single connection
func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()

	transport := &sdkmcp.IOTransport{Reader: conn, Writer: conn}
	if err := s.sdk.Run(context.Background(), transport); err != nil {
		if s.config.LogRequests {
			logger.Printf("MCP: Connection error: %v", err)
		}
	}
}
