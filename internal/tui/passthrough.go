//go:build unix

package tui

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/biliqiqi/ac2/internal/logger"
	"github.com/biliqiqi/ac2/internal/pool"
	ptyproxy "github.com/biliqiqi/ac2/internal/pty"
	"github.com/biliqiqi/ac2/internal/webterm"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

const (
	keyCtrlBackslash = 0x1c // Ctrl+\
	keyCtrlQ         = 0x11 // Ctrl+Q
)

type Passthrough struct {
	agentPool    *pool.AgentPool
	currentAgent *pool.AgentInstance
	mainAgent    *pool.AgentInstance

	mcpSocketPath string
	webServer     WebTerminalServer

	oldState *term.State
	mu       sync.Mutex

	quit     chan struct{}
	stopOnce sync.Once

	exitWatchStop chan struct{}

	inputPaused bool
	switching   bool
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
		quit:          make(chan struct{}),
	}
}

func (p *Passthrough) Run() error {
	if p.mainAgent == nil {
		return fmt.Errorf("no main agent provided")
	}

	// Print hint banner
	p.printBanner()

	// Set terminal to raw mode
	var err error
	p.oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw mode: %w", err)
	}

	// Attach current agent output to stdout for interactive mode
	p.currentAgent.SetOutputSink(os.Stdout)

	p.startExitWatcher(p.currentAgent)

	// Handle window resize
	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	go p.handleResize(sigwinch)

	// Handle interrupt signals (Ctrl+C, SIGTERM)
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	go func() {
		logger.Printf("Passthrough.Run: signal handler goroutine started")
		sig := <-sigint
		logger.Printf("Passthrough.Run: received signal: %v, calling stop()", sig)
		p.stop()
		logger.Printf("Passthrough.Run: signal handler calling stop() completed")
	}()

	// Start stdin loop
	go p.readLoop()

	// Wait for exit
	logger.Printf("Passthrough.Run: waiting for quit signal...")
	<-p.quit
	logger.Printf("Passthrough.Run: quit signal received, starting shutdown sequence")

	// Detach current agent output to prevent mixing with shutdown messages
	logger.Printf("Passthrough.Run: detaching current agent output")
	p.mu.Lock()
	if p.currentAgent != nil {
		logger.Printf("Passthrough.Run: current agent is %s, detaching output sink", p.currentAgent.ID)
		p.currentAgent.SetOutputSink(nil)
	} else {
		logger.Printf("Passthrough.Run: no current agent to detach")
	}
	p.mu.Unlock()

	// Give a moment for any pending output to flush
	logger.Printf("Passthrough.Run: waiting 50ms for output to flush")
	time.Sleep(50 * time.Millisecond)

	// Restore terminal before showing exit message
	logger.Printf("Passthrough.Run: restoring terminal state")
	p.restoreTerminal()
	logger.Printf("Passthrough.Run: terminal state restored")

	// Show visible shutdown message
	fmt.Println("\nShutting down...")

	// Shutdown web server
	if p.webServer != nil {
		logger.Printf("Passthrough.Run: stopping web terminal server...")
		err := p.webServer.Stop()
		if err != nil {
			logger.Printf("Passthrough.Run: web server stop error: %v", err)
		} else {
			logger.Printf("Passthrough.Run: web server stopped successfully")
		}
	} else {
		logger.Printf("Passthrough.Run: no web server to stop")
	}

	// Shutdown all agents with visible progress
	if p.agentPool != nil {
		logger.Printf("Passthrough.Run: shutting down agent pool...")
		_ = p.agentPool.Shutdown()
		logger.Printf("Passthrough.Run: agent pool shutdown complete")
	} else {
		logger.Printf("Passthrough.Run: no agent pool to shutdown")
	}

	fmt.Println("Goodbye!")
	logger.Printf("Passthrough.Run: shutdown sequence complete, returning nil")
	return nil
}

func (p *Passthrough) printBanner() {
	// Gray colored hint, will scroll away as agent outputs
	fmt.Printf("\033[90m[ac2] Ctrl+\\ control mode │ Ctrl+Q quit │ Current: %s\033[0m\n",
		p.currentAgent.Name)
}

func (p *Passthrough) readLoop() {
	buf := make([]byte, 4096)
	pollFds := []unix.PollFd{{
		Fd:     int32(os.Stdin.Fd()),
		Events: unix.POLLIN,
	}}
	for {
		select {
		case <-p.quit:
			return
		default:
		}

		p.mu.Lock()
		paused := p.inputPaused
		p.mu.Unlock()
		if paused {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		nready, err := unix.Poll(pollFds, 100)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			return
		}
		if nready == 0 {
			continue
		}

		n, err := os.Stdin.Read(buf)
		if err != nil {
			return
		}

		// Process input buffer
		start := 0
		for i := 0; i < n; i++ {
			switch buf[i] {
			case keyCtrlBackslash:
				// Flush accumulated bytes to CURRENT agent
				if i > start {
					p.mu.Lock()
					if p.currentAgent != nil && p.currentAgent.Proxy != nil {
						_, _ = p.currentAgent.Proxy.Write(buf[start:i])
					}
					p.mu.Unlock()
				}

				// Handle control mode
				p.enterControlMode()

				// After return, we skip the control key itself
				start = i + 1

			case keyCtrlQ:
				logger.Printf("readLoop: Ctrl+Q detected, showing quit confirmation")
				if p.confirmQuit() {
					logger.Printf("readLoop: stop() completed, returning from readLoop")
					return
				}
			}
		}

		// Flush remaining bytes to CURRENT (potentially new) agent
		if start < n {
			p.mu.Lock()
			if p.currentAgent != nil && p.currentAgent.Proxy != nil {
				_, _ = p.currentAgent.Proxy.Write(buf[start:n])
			}
			p.mu.Unlock()
		}
	}
}

func (p *Passthrough) handleResize(sigwinch chan os.Signal) {
	for {
		select {
		case <-p.quit:
			return
		case <-sigwinch:
			p.mu.Lock()
			if p.currentAgent != nil && p.currentAgent.Proxy != nil {
				cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
				if err == nil {
					_ = p.currentAgent.Proxy.Resize(uint16(rows), uint16(cols))
				}
			}
			p.mu.Unlock()
		}
	}
}

func (p *Passthrough) enterControlMode() {
	p.mu.Lock()
	p.inputPaused = true
	p.mu.Unlock()

	// Restore terminal for TUI
	if p.oldState != nil {
		_ = term.Restore(int(os.Stdin.Fd()), p.oldState)
	}

	// Show control menu
	ctrl := NewControlMode(p.agentPool, p.currentAgent, p)
	action := ctrl.Run()

	// Handle action
	if p.handleControlAction(action) {
		return
	}

	// Return to raw mode
	var err error
	p.oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		p.stop()
	}
	p.mu.Lock()
	p.inputPaused = false
	p.mu.Unlock()
}

func (p *Passthrough) confirmQuit() bool {
	p.mu.Lock()
	p.inputPaused = true
	p.mu.Unlock()

	// Restore terminal for TUI
	if p.oldState != nil {
		_ = term.Restore(int(os.Stdin.Fd()), p.oldState)
	}

	// Show quit confirmation
	ctrl := NewControlMode(p.agentPool, p.currentAgent, p)
	action := ctrl.RunExitConfirm()

	if p.handleControlAction(action) {
		return true
	}

	// Return to raw mode
	var err error
	p.oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		p.stop()
		return true
	}
	p.mu.Lock()
	p.inputPaused = false
	p.mu.Unlock()
	return false
}

func (p *Passthrough) handleControlAction(action Action) bool {
	switch action.Type {
	case ActionQuit:
		p.stop()
		return true
	case ActionSwitch:
		p.mu.Lock()
		p.switching = true
		logger.Printf("ActionSwitch: acquired lock, switching=true")
		p.mu.Unlock()

		fmt.Println("\n\033[36mSwitching agents... please wait...\033[0m")

		p.stopExitWatcher()

		// Stop current agent
		if p.currentAgent != nil && p.currentAgent.Proxy != nil {
			logger.Printf("ActionSwitch: stopping current agent %s", p.currentAgent.ID)

			// Mark as stopped immediately to prevent reuse if switching to same type
			p.currentAgent.Status = pool.StatusStopped
			_ = p.currentAgent.Proxy.Stop()

			// Wait for agent process to truly exit
			startWait := time.Now()
			for {
				if p.currentAgent.Proxy.Status() == ptyproxy.StatusStopped {
					logger.Printf("ActionSwitch: agent %s stopped gracefully", p.currentAgent.ID)
					break
				}
				if time.Since(startWait) > 3*time.Second {
					logger.Printf("ActionSwitch: timed out waiting for agent %s to stop", p.currentAgent.ID)
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
		}

		// Start new agent
		logger.Printf("ActionSwitch: starting new agent %s", action.TargetAgentType)
		agent, err := p.agentPool.GetOrCreate(action.TargetAgentType)
		if err != nil {
			logger.Printf("Failed to switch agent: %v", err)
			p.mu.Lock()
			p.switching = false
			p.mu.Unlock()
			p.stop()
			return true
		}

		// Update main agent reference if we are replacing the main entry point
		if p.mainAgent != nil && p.currentAgent != nil && p.mainAgent.ID == p.currentAgent.ID {
			logger.Printf("ActionSwitch: updating main agent from %s to %s", p.mainAgent.ID, agent.ID)
			p.mainAgent = agent
		}

		p.currentAgent = agent
		p.currentAgent.SetOutputSink(os.Stdout)
		p.startExitWatcher(p.currentAgent)

		logger.Printf("Switched to agent: %s", agent.Name)
		p.printBanner()

		// Trigger resize to ensure correct size
		cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
		if err == nil {
			_ = p.currentAgent.Proxy.Resize(uint16(rows), uint16(cols))
		}

		if p.webServer != nil {
			p.webServer.SetProxy(agent.Proxy)
			p.webServer.SetAgentName(agent.Name)
			p.webServer.BroadcastReset()
		}

		p.mu.Lock()
		p.switching = false
		logger.Printf("ActionSwitch: switch complete, switching=false")
		p.mu.Unlock()
	}

	return false
}

func (p *Passthrough) restoreTerminal() {
	if p.oldState != nil {
		_ = term.Restore(int(os.Stdin.Fd()), p.oldState)
	}
}

func (p *Passthrough) stop() {
	p.stopOnce.Do(func() {
		logger.Printf("Passthrough.stop: called, stopping exit watcher and closing quit channel")
		p.stopExitWatcher()
		logger.Printf("Passthrough.stop: exit watcher stopped")
		close(p.quit)
		logger.Printf("Passthrough.stop: quit channel closed")
	})
}

func (p *Passthrough) startExitWatcher(agent *pool.AgentInstance) {
	p.stopExitWatcher()

	if agent == nil || agent.ExitCh == nil {
		return
	}

	stopCh := make(chan struct{})
	p.exitWatchStop = stopCh

	go func() {
		select {
		case err := <-agent.ExitCh:
			p.handleAgentExit(agent, err)
		case <-stopCh:
			return
		}
	}()
}

func (p *Passthrough) stopExitWatcher() {
	if p.exitWatchStop != nil {
		close(p.exitWatchStop)
		p.exitWatchStop = nil
	}
}

func (p *Passthrough) handleAgentExit(agent *pool.AgentInstance, err error) {
	logger.Printf("HandleAgentExit: agent=%s error=%v", agent.ID, err)
	p.mu.Lock()
	if p.switching {
		logger.Printf("HandleAgentExit: ignoring exit due to switching flag")
		p.mu.Unlock()
		return
	}
	isCurrent := p.currentAgent != nil && p.currentAgent.ID == agent.ID
	p.mu.Unlock()

	if err != nil {
		logger.Printf("Agent exited with error: %s (%v)", agent.ID, err)
	} else {
		logger.Printf("Agent exited: %s", agent.ID)
	}

	if !isCurrent {
		logger.Printf("HandleAgentExit: agent %s is not current, ignoring", agent.ID)
		return
	}

	if p.mainAgent != nil && agent.ID == p.mainAgent.ID {
		logger.Printf("HandleAgentExit: agent %s is main agent, but not stopping app; entering control mode", agent.ID)
		// p.stop()
		// return
	}

	p.enterControlMode()
}

func (p *Passthrough) SwitchToAgent(agentID string) error {
	agent, err := p.agentPool.Get(agentID)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Detach current agent output from stdout
	if p.currentAgent != nil {
		p.currentAgent.SetOutputSink(nil)
	}

	p.currentAgent = agent

	// Attach new agent output to stdout
	agent.SetOutputSink(os.Stdout)
	p.startExitWatcher(agent)

	// Update banner
	logger.Printf("Switched to: %s", agent.ID)
	if p.webServer != nil {
		name := agent.Name
		if name == "" {
			name = agent.Type
		}
		p.webServer.SetAgentName(name)
	}

	return nil
}

func (p *Passthrough) SwitchToMain() error {
	return p.SwitchToAgent(p.mainAgent.ID)
}
