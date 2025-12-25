//go:build windows

package tui

import (
	"fmt"

	"github.com/biliqiqi/ac2/internal/pool"
	ptyproxy "github.com/biliqiqi/ac2/internal/pty"
	"github.com/biliqiqi/ac2/internal/webterm"
)

type Passthrough struct {
	agentPool     *pool.AgentPool
	currentAgent  *pool.AgentInstance
	mainAgent     *pool.AgentInstance
	mcpSocketPath string
	webServer     WebTerminalServer
}

type WebTerminalServer interface {
	IsWebActive() bool
	SetActiveSource(source string)
	SetAgentName(name string)
	SetProxy(proxy *ptyproxy.Proxy)
	BroadcastReset()
	ListClients() []webterm.ClientInfo
	DisconnectClient(id string) error
	Stop() error
}

func NewPassthrough(agentPool *pool.AgentPool, mainAgent *pool.AgentInstance, mcpSocketPath string, webServer WebTerminalServer) *Passthrough {
	return &Passthrough{
		agentPool:     agentPool,
		currentAgent:  mainAgent,
		mainAgent:     mainAgent,
		mcpSocketPath: mcpSocketPath,
		webServer:     webServer,
	}
}

func (p *Passthrough) Run() error {
	return fmt.Errorf("passthrough TUI mode is not supported on Windows, please use --no-tui flag for headless mode")
}

func (p *Passthrough) SwitchToAgent(agentID string) error {
	return fmt.Errorf("not supported on Windows")
}

func (p *Passthrough) SwitchToMain() error {
	return fmt.Errorf("not supported on Windows")
}
