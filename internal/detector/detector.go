package detector

import (
	"context"
	"os/exec"
	"sync"
	"time"
)

type AgentType string

const (
	AgentClaude AgentType = "claude"
	AgentCodex  AgentType = "codex"
	AgentGemini AgentType = "gemini"
)

type AgentInfo struct {
	Type    AgentType
	Name    string
	Command string
	Version string
	Found   bool
}

var knownAgents = []AgentInfo{
	{Type: AgentClaude, Name: "Claude Code", Command: "claude"},
	{Type: AgentCodex, Name: "Codex", Command: "codex"},
	{Type: AgentGemini, Name: "Gemini CLI", Command: "gemini"},
}

type Detector struct {
	agents []AgentInfo
}

func New() *Detector {
	// Create a copy of knownAgents
	agents := make([]AgentInfo, len(knownAgents))
	copy(agents, knownAgents)

	return &Detector{
		agents: agents,
	}
}

func (d *Detector) Scan() []AgentInfo {
	var wg sync.WaitGroup
	results := make([]AgentInfo, len(knownAgents))

	for i, agent := range knownAgents {
		wg.Add(1)
		go func(idx int, a AgentInfo) {
			defer wg.Done()
			results[idx] = d.checkAgent(a)
		}(i, agent)
	}

	wg.Wait()
	d.agents = results
	return results
}

func (d *Detector) checkAgent(agent AgentInfo) AgentInfo {
	result := agent
	path, err := exec.LookPath(agent.Command)
	if err != nil {
		result.Found = false
		return result
	}

	result.Found = true
	result.Version = d.getVersion(agent.Command, path)
	return result
}

func (d *Detector) getVersion(command, path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--version")
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return "unknown (timeout)"
	}
	if err != nil {
		return "unknown"
	}
	version := string(out)
	if len(version) > 50 {
		version = version[:50] + "..."
	}
	return version
}

func (d *Detector) GetAvailable() []AgentInfo {
	var available []AgentInfo
	for _, agent := range d.agents {
		if agent.Found {
			available = append(available, agent)
		}
	}
	return available
}

func (d *Detector) GetAll() []AgentInfo {
	return d.agents
}
