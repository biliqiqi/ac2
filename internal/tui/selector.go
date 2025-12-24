package tui

import (
	"fmt"
	"os"

	"github.com/biliqiqi/ac2/internal/detector"
	"golang.org/x/term"
)

type Selector struct {
	agents   []detector.AgentInfo
	selected int
	oldState *term.State
}

func NewSelector(agents []detector.AgentInfo) *Selector {
	return &Selector{
		agents:   agents,
		selected: 0,
	}
}

func (s *Selector) Run() (*detector.AgentInfo, error) {
	available := s.getAvailable()
	if len(available) == 0 {
		return nil, fmt.Errorf("no agents available")
	}

	var err error
	s.oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), s.oldState) }()

	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h")

	s.render(available)

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return nil, err
		}

		if n == 1 {
			switch buf[0] {
			case 'j', 'J':
				s.selected = (s.selected + 1) % len(available)
			case 'k', 'K':
				s.selected = (s.selected - 1 + len(available)) % len(available)
			case '\r', '\n':
				s.clearMenu(len(available))
				return &available[s.selected], nil
			case 'q', 3: // q or Ctrl+C
				s.clearMenu(len(available))
				return nil, nil
			}
		} else if n == 3 && buf[0] == 27 && buf[1] == '[' {
			switch buf[2] {
			case 'A': // Up
				s.selected = (s.selected - 1 + len(available)) % len(available)
			case 'B': // Down
				s.selected = (s.selected + 1) % len(available)
			}
		}

		s.render(available)
	}
}

func (s *Selector) getAvailable() []detector.AgentInfo {
	var available []detector.AgentInfo
	for _, a := range s.agents {
		if a.Found {
			available = append(available, a)
		}
	}
	return available
}

func (s *Selector) render(available []detector.AgentInfo) {
	fmt.Printf("\033[%dA", len(available))

	for i, agent := range available {
		fmt.Print("\r\033[K")
		if i == s.selected {
			fmt.Printf("\033[36m❯ %s\033[0m\n", agent.Name)
		} else {
			fmt.Printf("  %s\n", agent.Name)
		}
	}
}

func (s *Selector) clearMenu(count int) {
	fmt.Printf("\033[%dA", count)
	for i := 0; i < count; i++ {
		fmt.Print("\r\033[K\n")
	}
	fmt.Printf("\033[%dA", count)
}
