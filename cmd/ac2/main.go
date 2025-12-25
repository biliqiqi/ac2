package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/biliqiqi/ac2/internal/detector"
	"github.com/biliqiqi/ac2/internal/logger"
	"github.com/biliqiqi/ac2/internal/pool"
	"github.com/biliqiqi/ac2/internal/tui"
	"github.com/biliqiqi/ac2/internal/webterm"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	entryAgent string
	webPort    int
	webUser    string
	webPass    string
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			f, _ := os.OpenFile("panic.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			_, _ = fmt.Fprintf(f, "Panic: %v\n", r)
			_ = f.Close()
			os.Exit(1)
		}
	}()

	rootCmd := &cobra.Command{
		Use:   "ac2",
		Short: "Agents COOP - Multi-agent collaboration framework",
		RunE:  run,
	}

	rootCmd.Flags().StringVarP(&entryAgent, "entry", "e", "", "entry agent (claude, codex, gemini)")
	rootCmd.Flags().IntVar(&webPort, "web-port", 8080, "web terminal port")
	rootCmd.Flags().StringVar(&webUser, "web-user", "", "web terminal username for Basic Auth")
	rootCmd.Flags().StringVar(&webPass, "web-pass", "", "web terminal password for Basic Auth")

	// Add subcommands
	rootCmd.AddCommand(getMCPStdioCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Initialize logger
	debugEnv := strings.ToLower(os.Getenv("DEBUG"))
	if debugEnv == "true" || debugEnv == "1" {
		logFile := "ac2.log"
		if err := logger.Init(logFile); err != nil {
			fmt.Printf("Warning: failed to initialize logger: %v\n", err)
		} else {
			fmt.Printf("Logging debug info to %s\n", logFile)
		}
	}
	defer logger.Close()

	fmt.Println("ac2 - agentic cli toolkit")
	fmt.Println()

	det := detector.New()
	agents := det.Scan()

	available := det.GetAvailable()
	if len(available) == 0 {
		fmt.Println("No agents found. Please install one of:")
		fmt.Println("  - Claude Code: https://claude.ai/code")
		fmt.Println("  - Codex CLI")
		fmt.Println("  - Gemini CLI")
		return nil
	}

	// Check and find available port before displaying to user
	availablePort, err := findAvailablePort(webPort, 10)
	if err != nil {
		return err
	}
	if availablePort != webPort {
		fmt.Printf("\033[33mWarning: Port %d is already in use, will use port %d instead\033[0m\n\n", webPort, availablePort)
		webPort = availablePort
	}

	var entry *detector.AgentInfo

	if entryAgent != "" {
		for _, agent := range available {
			if string(agent.Type) == entryAgent || agent.Command == entryAgent {
				entry = &agent
				break
			}
		}
		if entry == nil {
			return fmt.Errorf("agent '%s' not found", entryAgent)
		}
	} else {
		fmt.Printf("Web terminal will listen at http://localhost:%d\n\n", webPort)
		fmt.Println("Select entry agent:")
		for _, a := range agents {
			if a.Found {
				fmt.Printf("  %s\n", a.Name)
			} else {
				fmt.Printf("  \033[90m%s (not found)\033[0m\n", a.Name)
			}
		}

		selector := tui.NewSelector(agents)
		selected, err := selector.Run()
		if err != nil {
			return err
		}
		if selected == nil {
			return nil
		}
		entry = selected
		// Give the terminal a moment to restore state before tview takes over
		time.Sleep(50 * time.Millisecond)
	}

	// Drain any residual input (like 'j' or 'Enter' from the selector)
	flushStdin()

	if webUser == "" && webPass == "" {
		user, pass, err := promptWebAuth()
		if err != nil {
			return err
		}
		webUser = user
		webPass = pass
	}

	// Create Agent Pool (no MCP HTTP server needed)
	agentPool := pool.NewAgentPool(available, "")

	// Create initial agent instance
	mainAgent, err := agentPool.GetOrCreate(string(entry.Type), pool.WithOutputSink(os.Stdout))
	if err != nil {
		return fmt.Errorf("failed to create entry agent: %w", err)
	}

	// Start Web Terminal Server (always enabled)
	webServer := webterm.NewServer(webPort, webUser, webPass, mainAgent.Name)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Printf("Web Terminal Server panic: %v", r)
			}
			logger.Printf("Web Terminal Server goroutine exiting")
		}()
		logger.Printf("Web Terminal Server goroutine started, listening on port %d...", webPort)
		if err := webServer.Start(mainAgent.Proxy); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Printf("Web Terminal Server error: %v", err)
		}
		logger.Printf("Web Terminal Server gracefully stopped")
	}()
	logger.Printf("Waiting 100ms for Web Terminal server to start...")
	time.Sleep(100 * time.Millisecond)
	logger.Printf("Web Terminal server should be running now")

	// Display Web Terminal info
	lines := []string{
		fmt.Sprintf("Web Terminal: http://localhost:%d", webPort),
		fmt.Sprintf("Entry Agent: %s", mainAgent.ID),
	}
	if webUser != "" && webPass != "" {
		lines = append(lines, fmt.Sprintf("Auth: %s / %s", webUser, "********"))
	} else {
		lines = append(lines, "Auth: None (use --web-user and --web-pass)")
	}
	printBox(lines)

	// Start Passthrough TUI
	logger.Printf("Main: starting Passthrough TUI")
	pt := tui.NewPassthrough(agentPool, mainAgent, "", webServer)
	err = pt.Run()
	logger.Printf("Main: Passthrough TUI returned with err=%v", err)
	logger.Printf("Main: run() function returning, all defers will execute")
	return err
}

func findAvailablePort(startPort int, maxRetries int) (int, error) {
	for i := 0; i < maxRetries; i++ {
		port := startPort + i
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			_ = listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found from %d to %d", startPort, startPort+maxRetries-1)
}

func flushStdin() {
	// Set non-blocking
	_ = syscall.SetNonblock(int(os.Stdin.Fd()), true)
	defer func() {
		// Restore blocking mode
		_ = syscall.SetNonblock(int(os.Stdin.Fd()), false)
	}()

	buf := make([]byte, 1024)
	for {
		// Read until error (EAGAIN/EWOULDBLOCK) or EOF
		n, err := os.Stdin.Read(buf)
		if n <= 0 || err != nil {
			break
		}
	}
}

func promptWebAuth() (string, string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Web terminal auth username(leave empty for no auth): ")
	user, err := reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	user = strings.TrimSpace(user)
	if user == "" {
		return "", "", nil
	}

	fmt.Print("password: ")
	passBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", "", err
	}
	pass := strings.TrimSpace(string(passBytes))
	if pass == "" {
		return "", "", nil
	}

	return user, pass, nil
}

func printBox(lines []string) {
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	border := strings.Repeat("═", maxLen+2)
	fmt.Printf("\n\033[36m╔%s╗\033[0m\n", border)
	for _, line := range lines {
		padding := strings.Repeat(" ", maxLen-len(line))
		fmt.Printf("\033[36m║\033[0m %s%s \033[36m║\033[0m\n", line, padding)
	}
	fmt.Printf("\033[36m╚%s╝\033[0m\n\n", border)
}
