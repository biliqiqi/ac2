package pty

import (
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/biliqiqi/ac2/internal/logger"
	"github.com/creack/pty"
)

type Status int

const (
	StatusStopped Status = iota
	StatusStarting
	StatusRunning
	StatusError
)

func (s Status) String() string {
	switch s {
	case StatusStopped:
		return "stopped"
	case StatusStarting:
		return "starting"
	case StatusRunning:
		return "running"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

type OutputHandler struct {
	id      string
	handler func([]byte)
}

type Proxy struct {
	command string
	args    []string
	env     []string
	cmd     *exec.Cmd
	ptmx    *os.File
	status  Status
	mu      sync.RWMutex

	onOutput func([]byte)
	onExit   func(error)

	handlers   map[string]*OutputHandler
	handlersMu sync.RWMutex
}

func NewProxy(command string, args ...string) *Proxy {
	return &Proxy{
		command: command,
		args:    args,
		env:     []string{},
		status:  StatusStopped,
	}
}

func (p *Proxy) SetEnv(env []string) {
	p.env = env
}

func (p *Proxy) SetOutputHandler(handler func([]byte)) {
	p.onOutput = handler
}

func (p *Proxy) SetExitHandler(handler func(error)) {
	p.onExit = handler
}

func (p *Proxy) AddOutputHandler(id string, handler func([]byte)) {
	p.handlersMu.Lock()
	defer p.handlersMu.Unlock()

	if p.handlers == nil {
		p.handlers = make(map[string]*OutputHandler)
	}
	p.handlers[id] = &OutputHandler{
		id:      id,
		handler: handler,
	}
}

func (p *Proxy) RemoveOutputHandler(id string) {
	p.handlersMu.Lock()
	defer p.handlersMu.Unlock()
	delete(p.handlers, id)
}

func (p *Proxy) Start(size *pty.Winsize) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.status = StatusStarting
	p.cmd = exec.Command(p.command, p.args...)
	p.cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	p.cmd.Env = append(p.cmd.Env, p.env...)

	var err error
	if size != nil {
		p.ptmx, err = pty.StartWithSize(p.cmd, size)
	} else {
		p.ptmx, err = pty.Start(p.cmd)
	}

	if err != nil {
		p.status = StatusError
		return err
	}

	p.status = StatusRunning

	go p.readLoop()
	go p.waitLoop()

	return nil
}

func (p *Proxy) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := p.ptmx.Read(buf)
		if err != nil {
			if err != io.EOF {
				p.setStatus(StatusError)
			}
			return
		}
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])

			// Call legacy handler for backward compatibility
			if p.onOutput != nil {
				p.onOutput(data)
			}

			// Broadcast to all registered handlers
			p.handlersMu.RLock()
			for _, h := range p.handlers {
				dataCopy := make([]byte, len(data))
				copy(dataCopy, data)
				go h.handler(dataCopy)
			}
			p.handlersMu.RUnlock()
		}
	}
}

func (p *Proxy) waitLoop() {
	err := p.cmd.Wait()
	p.mu.Lock()
	p.status = StatusStopped
	p.mu.Unlock()

	if p.onExit != nil {
		p.onExit(err)
	}
}

func (p *Proxy) Write(data []byte) (int, error) {
	if p.ptmx == nil {
		return 0, io.ErrClosedPipe
	}
	return p.ptmx.Write(data)
}

func (p *Proxy) Read(data []byte) (int, error) {
	if p.ptmx == nil {
		return 0, io.ErrClosedPipe
	}
	return p.ptmx.Read(data)
}

func (p *Proxy) Resize(rows, cols uint16) error {
	if p.ptmx == nil {
		return nil
	}
	return pty.Setsize(p.ptmx, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

func (p *Proxy) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd != nil && p.cmd.Process != nil {
		logger.Printf("Proxy.Stop: attempting graceful shutdown with SIGTERM for PID %d", p.cmd.Process.Pid)

		// Try graceful shutdown with SIGTERM first
		if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			logger.Printf("Proxy.Stop: SIGTERM failed: %v, will use SIGKILL", err)
			_ = p.cmd.Process.Kill()
		} else {
			// Wait up to 1 second for graceful exit
			done := make(chan error, 1)
			go func() {
				_, err := p.cmd.Process.Wait()
				done <- err
			}()

			select {
			case <-done:
				logger.Printf("Proxy.Stop: process exited gracefully")
			case <-time.After(3 * time.Second):
				logger.Printf("Proxy.Stop: graceful shutdown timeout, forcing with SIGKILL")
				_ = p.cmd.Process.Kill()
			}
		}
	}

	if p.ptmx != nil {
		_ = p.ptmx.Close()
	}
	p.status = StatusStopped
	return nil
}

func (p *Proxy) Status() Status {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

func (p *Proxy) setStatus(s Status) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = s
}

func (p *Proxy) Fd() uintptr {
	if p.ptmx == nil {
		return 0
	}
	return p.ptmx.Fd()
}

func (p *Proxy) Pid() int {
	if p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}
