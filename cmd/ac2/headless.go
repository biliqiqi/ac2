package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/biliqiqi/ac2/internal/logger"
	"github.com/biliqiqi/ac2/internal/pool"
	"github.com/biliqiqi/ac2/internal/webterm"
)

// runHeadless runs ac2 without a local TUI and waits for shutdown signals.
func runHeadless(agentPool *pool.AgentPool, mainAgent *pool.AgentInstance, webServer *webterm.Server, pidPath string) error {
	if err := writePIDFile(pidPath); err != nil {
		return err
	}
	defer removePIDFile(pidPath)

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(sigCh)

	agentExit := make(chan error, 1)
	if mainAgent != nil {
		go func() {
			err := <-mainAgent.ExitCh
			agentExit <- err
		}()
	}

	select {
	case sig := <-sigCh:
		logger.Printf("NoTUI: received signal %v, shutting down", sig)
	case err := <-agentExit:
		logger.Printf("NoTUI: agent exited (%v), shutting down", err)
	}

	shutdownDone := make(chan struct{})
	go func() {
		fmt.Println("Shutting down...")
		if webServer != nil {
			_ = webServer.Stop()
		}
		if agentPool != nil {
			_ = agentPool.Shutdown()
		}
		fmt.Println("Goodbye!")
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		return nil
	case sig := <-sigCh:
		logger.Printf("NoTUI: received signal %v during shutdown, forcing exit", sig)
		os.Exit(1)
	case <-time.After(10 * time.Second):
		logger.Printf("NoTUI: shutdown timeout, forcing exit")
		os.Exit(1)
	}
	return nil
}
