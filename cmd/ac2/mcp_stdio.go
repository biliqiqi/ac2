package main

import (
	"context"
	"os"

	"github.com/biliqiqi/ac2/internal/detector"
	"github.com/biliqiqi/ac2/internal/logger"
	"github.com/biliqiqi/ac2/internal/mcp"
	"github.com/biliqiqi/ac2/internal/pool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

// getMCPStdioCmd returns the mcp-stdio subcommand
func getMCPStdioCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp-stdio",
		Short: "Run ac2 as an MCP server over stdio (for Claude Code integration)",
		Long: `Run ac2 as an MCP server using stdio transport.

This mode allows Claude Code to start ac2 on-demand when MCP tools are called.
Add to Claude Code with:

    claude mcp add --transport stdio ac2 "ac2 mcp-stdio"

Then you can use tools with slash commands like:
    /ac2:ask-claude
    /ac2:ask-gemini
    /ac2:ask-codex
`,
		RunE: runMCPStdio,
	}
}

func runMCPStdio(cmd *cobra.Command, args []string) error {
	// Initialize logger to file only (no stdout to avoid interfering with stdio)
	_ = logger.Init("") // Ignore error, continue anyway
	defer logger.Close()

	// Capture the original stdout for MCP communication
	mcpStdout := os.Stdout

	// Redirect global stdout to stderr to prevent libraries/functions
	// from corrupting the JSON-RPC stream on stdout.
	os.Stdout = os.Stderr

	logger.Println("Starting ac2 MCP server in stdio mode...")

	// Initialize with all known agents so tools work immediately
	// The pool will attempt to execute them by command name (e.g. "gemini")
	det := detector.New()
	available := det.GetAll()

	// Do lazy detection in background just for logging/status updates
	go func() {
		det.Scan()
		detected := det.GetAvailable()
		logger.Printf("Background detection found %d installed agents", len(detected))
	}()

	// Create Agent Pool with all known agents
	agentPool := pool.NewAgentPool(available, "")

	// Create MCP Server
	mcpServer := mcp.NewServer(agentPool)

	// Create stdio transport
	transport := &sdkmcp.IOTransport{
		Reader: os.Stdin,
		Writer: mcpStdout,
	}

	logger.Println("MCP server ready, starting stdio transport...")

	// Run the server (blocking until stdin closes)
	if err := mcpServer.GetSDKServer().Run(context.Background(), transport); err != nil {
		logger.Printf("MCP server error: %v", err)
		return err
	}

	logger.Println("MCP server stopped normally")
	return nil
}
