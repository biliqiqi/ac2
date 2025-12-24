package pool

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/biliqiqi/ac2/internal/detector"
	"github.com/biliqiqi/ac2/internal/logger"
	ptyproxy "github.com/biliqiqi/ac2/internal/pty"
	"github.com/creack/pty"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
	StatusError   Status = "error"
)

type AgentInstance struct {
	ID   string
	Type string
	Name string

	Proxy  *ptyproxy.Proxy
	Status Status

	OutputBuffer *bytes.Buffer
	OutputMu     sync.Mutex
	OutputSink   io.Writer
	ExitCh       chan error
	outputFilter *ansiFilter
	hintSent     bool
	hintMu       sync.Mutex
}

type AgentPool struct {
	agents    map[string]*AgentInstance
	mu        sync.RWMutex
	available map[string]*detector.AgentInfo
	counter   map[string]int
	mcpAddr   string
}

type AgentInfo struct {
	ID     string
	Type   string
	Status Status
}

type AgentOption func(*agentOptions)

type agentOptions struct {
	outputSink io.Writer
	quiet      bool
}

func WithOutputSink(sink io.Writer) AgentOption {
	return func(opts *agentOptions) {
		opts.outputSink = sink
	}
}

func WithQuiet(quiet bool) AgentOption {
	return func(opts *agentOptions) {
		opts.quiet = quiet
	}
}

func NewAgentPool(available []detector.AgentInfo, mcpAddr string) *AgentPool {
	availableMap := make(map[string]*detector.AgentInfo)
	for i := range available {
		agent := &available[i]
		availableMap[string(agent.Type)] = agent
	}

	return &AgentPool{
		agents:    make(map[string]*AgentInstance),
		available: availableMap,
		counter:   make(map[string]int),
		mcpAddr:   mcpAddr,
	}
}

func (p *AgentPool) setupClaudeMCP(quiet bool) error {
	// Use official `claude mcp add` command
	// Reference: https://github.com/anthropics/claude-code

	if !quiet {
		logger.Printf("Configuring Claude Code MCP via CLI...")
	}

	// Remove existing ac2 MCP server if present
	cmd := exec.Command("claude", "mcp", "remove", "ac2")
	_ = cmd.Run() // Ignore error if not exists

	// Add ac2 MCP server with HTTP transport
	cmd = exec.Command("claude", "mcp", "add",
		"--transport", "http",
		"ac2",
		p.mcpAddr+"/mcp",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add Claude MCP: %w\nOutput: %s", err, output)
	}

	if !quiet {
		logger.Printf("Claude MCP configured: %s/mcp", p.mcpAddr)
	}
	return nil
}

func (p *AgentPool) setupGeminiMCP(quiet bool) error {
	// TODO: Research official Gemini CLI MCP configuration
	// For now, use environment variable fallback
	if !quiet {
		logger.Printf("Gemini MCP: No official CLI command found, using env vars")
	}
	return nil
}

func (p *AgentPool) setupCodexMCP(quiet bool) error {
	// TODO: Research official Codex CLI MCP configuration
	// For now, use environment variable fallback
	if !quiet {
		logger.Printf("Codex MCP: No official CLI command found, using env vars")
	}
	return nil
}

func (p *AgentPool) buildMCPEnv(agentType string, quiet bool) []string {
	env := []string{}

	switch agentType {
	case "claude":
		// Claude Code is configured via `claude mcp add` command
		// No environment variables needed
		if !quiet {
			logger.Printf("Claude MCP configured via CLI command")
		}

	case "gemini":
		// Gemini CLI MCP configuration via environment variables
		env = append(env, fmt.Sprintf("MCP_SERVER_URL=%s/mcp", p.mcpAddr))
		if !quiet {
			logger.Printf("Gemini env: MCP_SERVER_URL=%s/mcp", p.mcpAddr)
		}

	case "codex":
		// Codex CLI MCP configuration via environment variables
		env = append(env, fmt.Sprintf("MCP_SERVER_URL=%s/mcp", p.mcpAddr))
		if !quiet {
			logger.Printf("Codex env: MCP_SERVER_URL=%s/mcp", p.mcpAddr)
		}

	default:
		// Generic MCP configuration
		env = append(env, fmt.Sprintf("MCP_SERVER_URL=%s/mcp", p.mcpAddr))
		if !quiet {
			logger.Printf("%s env: MCP_SERVER_URL=%s/mcp", agentType, p.mcpAddr)
		}
	}

	return env
}

func (p *AgentPool) GetOrCreate(agentType string, opts ...AgentOption) (*AgentInstance, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	options := &agentOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(options)
		}
	}

	agentInfo, ok := p.available[agentType]
	if !ok {
		return nil, fmt.Errorf("agent type '%s' not available", agentType)
	}

	for _, agent := range p.agents {
		if agent.Type == agentType && agent.Status == StatusRunning {
			if options.outputSink != nil {
				agent.SetOutputSink(options.outputSink)
			}
			return agent, nil
		}
	}

	p.counter[agentType]++
	id := fmt.Sprintf("%s-%d", agentType, p.counter[agentType])

	if !options.quiet {
		logger.Printf("Starting %s...", id)
	}

	// Setup MCP configuration via CLI (one-time setup for each agent type)
	if p.counter[agentType] == 1 {
		switch agentType {
		case "claude":
			if err := p.setupClaudeMCP(options.quiet); err != nil && !options.quiet {
				logger.Printf("Warning: Failed to setup Claude MCP: %v", err)
			}
		case "gemini":
			if err := p.setupGeminiMCP(options.quiet); err != nil && !options.quiet {
				logger.Printf("Warning: Failed to setup Gemini MCP: %v", err)
			}
		case "codex":
			if err := p.setupCodexMCP(options.quiet); err != nil && !options.quiet {
				logger.Printf("Warning: Failed to setup Codex MCP: %v", err)
			}
		}
	}

	instance := &AgentInstance{
		ID:           id,
		Type:         agentType,
		Name:         agentInfo.Name,
		Status:       StatusStopped,
		OutputBuffer: new(bytes.Buffer),
		OutputSink:   options.outputSink,
		ExitCh:       make(chan error, 1),
	}
	if agentType == "codex" {
		instance.outputFilter = &ansiFilter{}
	}

	proxy := ptyproxy.NewProxy(agentInfo.Command)

	// Inject MCP environment variables (if needed)
	mcpEnv := p.buildMCPEnv(agentType, options.quiet)
	proxy.SetEnv(mcpEnv)

	proxy.SetOutputHandler(func(data []byte) {
		if instance.outputFilter != nil {
			data = instance.outputFilter.Filter(data)
			if len(data) == 0 {
				return
			}
		}
		instance.OutputMu.Lock()
		instance.OutputBuffer.Write(data)
		sink := instance.OutputSink
		instance.OutputMu.Unlock()
		if sink != nil {
			_, _ = sink.Write(data)
		}
	})
	proxy.SetExitHandler(func(err error) {
		if err != nil {
			instance.Status = StatusError
		} else {
			instance.Status = StatusStopped
		}
		select {
		case instance.ExitCh <- err:
		default:
		}
	})

	if err := proxy.Start(&pty.Winsize{Rows: 24, Cols: 80}); err != nil {
		return nil, fmt.Errorf("failed to start agent: %w", err)
	}

	instance.Proxy = proxy
	instance.Status = StatusRunning

	p.agents[id] = instance

	return instance, nil
}

func (p *AgentPool) Get(id string) (*AgentInstance, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	instance, ok := p.agents[id]
	if !ok {
		return nil, fmt.Errorf("agent instance '%s' not found", id)
	}

	return instance, nil
}

func (p *AgentPool) ListAll() []AgentInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]AgentInfo, 0, len(p.agents))
	for _, agent := range p.agents {
		result = append(result, AgentInfo{
			ID:     agent.ID,
			Type:   agent.Type,
			Status: agent.Status,
		})
	}

	return result
}

func (p *AgentPool) GetAvailableAgents() []detector.AgentInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]detector.AgentInfo, 0, len(p.available))
	for _, agent := range p.available {
		result = append(result, *agent)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func (p *AgentPool) CallNonInteractive(ctx context.Context, agentType string, message string) (string, error) {
	p.mu.RLock()
	agentInfo, ok := p.available[agentType]
	p.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("agent type '%s' not available", agentType)
	}

	args := buildNonInteractiveArgs(agentType, message)
	cmd := exec.CommandContext(ctx, agentInfo.Command, args...)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("agent command failed: %w\nOutput: %s", err, output)
	}

	return strings.TrimSpace(string(output)), nil
}

func buildNonInteractiveArgs(agentType string, message string) []string {
	envKey := fmt.Sprintf("AC2_AGENT_ARGS_%s", strings.ToUpper(agentType))
	rawArgs := strings.TrimSpace(os.Getenv(envKey))
	if rawArgs == "" {
		// Default arguments for different agent types
		switch agentType {
		case "codex":
			// Codex uses 'exec' subcommand for non-interactive execution
			return []string{"exec", "--sandbox", "danger-full-access", message}
		default:
			// Claude and Gemini use -p flag
			return []string{"-p", message}
		}
	}

	args := strings.Fields(rawArgs)
	hasPlaceholder := false
	for i, arg := range args {
		if strings.Contains(arg, "{message}") {
			args[i] = strings.ReplaceAll(arg, "{message}", message)
			hasPlaceholder = true
		}
	}

	if !hasPlaceholder {
		args = append(args, message)
	}

	return args
}

func (ai *AgentInstance) SendAndWait(ctx context.Context, message string) (string, error) {
	ai.OutputMu.Lock()
	ai.OutputBuffer.Reset()
	ai.OutputMu.Unlock()

	_, err := ai.Proxy.Write([]byte(message + "\n"))
	if err != nil {
		return "", err
	}

	timeout := 3 * time.Second
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var lastSize int
	var silenceStart time.Time

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			ai.OutputMu.Lock()
			currentSize := ai.OutputBuffer.Len()
			ai.OutputMu.Unlock()

			if currentSize > lastSize {
				lastSize = currentSize
				silenceStart = time.Now()
				continue
			}

			if !silenceStart.IsZero() && time.Since(silenceStart) >= timeout {
				ai.OutputMu.Lock()
				result := ai.OutputBuffer.String()
				ai.OutputMu.Unlock()
				return result, nil
			}

			if silenceStart.IsZero() && currentSize > 0 {
				silenceStart = time.Now()
			}
		}
	}
}

func (ai *AgentInstance) SetOutputSink(sink io.Writer) {
	ai.OutputMu.Lock()
	ai.OutputSink = sink
	ai.OutputMu.Unlock()
}

func (ai *AgentInstance) SendHintOnce(message string) error {
	ai.hintMu.Lock()
	if ai.hintSent {
		ai.hintMu.Unlock()
		return nil
	}
	ai.hintSent = true
	ai.hintMu.Unlock()

	if ai.Proxy == nil {
		return fmt.Errorf("agent proxy not initialized")
	}
	_, err := ai.Proxy.Write([]byte(message + "\n"))
	return err
}
