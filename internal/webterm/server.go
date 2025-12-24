package webterm

import (
	"crypto/subtle"
	"fmt"
	"html"
	"html/template"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	ptyproxy "github.com/biliqiqi/ac2/internal/pty"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	port         int
	authUser     string
	authPass     string
	agentName    string
	agentMu      sync.RWMutex
	proxy        *ptyproxy.Proxy
	handlerID    string
	clients      map[string]*Client
	clientsMu    sync.RWMutex
	httpServer   *http.Server
	activeSource string    // "web" or "local" or ""
	activeTime   time.Time // last input time
	activeMu     sync.RWMutex
}

type ClientInfo struct {
	ID        string
	Addr      string
	UserAgent string
}

const disconnectCloseCode = 4001

func NewServer(port int, authUser, authPass, agentName string) *Server {
	return &Server{
		port:      port,
		authUser:  authUser,
		authPass:  authPass,
		agentName: agentName,
		clients:   make(map[string]*Client),
		handlerID: fmt.Sprintf("webterm-%d", time.Now().UnixNano()),
	}
}

func (s *Server) Start(proxy *ptyproxy.Proxy) error {
	s.proxy = proxy

	// Keep local terminal output active
	// Both local and web terminals will show output
	proxy.AddOutputHandler(s.handlerID, s.broadcastOutput)

	// Start activity timeout checker
	go s.checkActivityTimeout()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/static/xterm.css", s.handleXtermCSS)
	mux.HandleFunc("/static/xterm.js", s.handleXtermJS)
	mux.HandleFunc("/static/addon-fit.js", s.handleAddonFitJS)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.withAuth(mux),
	}

	return s.httpServer.ListenAndServe()
}

func (s *Server) broadcastOutput(data []byte) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for _, client := range s.clients {
		client.Send(data)
	}
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.authUser == "" && s.authPass == "" {
			next.ServeHTTP(w, r)
			return
		}

		user, pass, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="ac2 Web Terminal"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(s.authUser)) == 1
		passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(s.authPass)) == 1

		if !userMatch || !passMatch {
			w.Header().Set("WWW-Authenticate", `Basic realm="ac2 Web Terminal"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(renderIndexHTML(s.agentName)))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleXtermCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	_, _ = w.Write(xtermCSS)
}

func (s *Server) handleXtermJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	_, _ = w.Write(xtermJS)
}

func (s *Server) handleAddonFitJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	_, _ = w.Write(addonFitJS)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())
	client := NewClient(clientID, conn, s, clientAddr(r.RemoteAddr), r.UserAgent())

	s.clientsMu.Lock()
	s.clients[clientID] = client
	s.clientsMu.Unlock()

	client.SendAgent(s.getAgentName())
}

func (s *Server) removeClient(id string) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	delete(s.clients, id)
}

func (s *Server) setActiveSource(source string) {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	s.activeSource = source
	s.activeTime = time.Now()
}

func (s *Server) IsWebActive() bool {
	s.activeMu.RLock()
	defer s.activeMu.RUnlock()
	return s.activeSource == "web"
}

func (s *Server) SetActiveSource(source string) {
	s.setActiveSource(source)
}

func (s *Server) SetProxy(proxy *ptyproxy.Proxy) {
	if s.proxy != nil {
		s.proxy.RemoveOutputHandler(s.handlerID)
	}

	s.proxy = proxy
	if s.proxy != nil {
		s.proxy.AddOutputHandler(s.handlerID, s.broadcastOutput)
	}
}

func (s *Server) SetAgentName(name string) {
	s.agentMu.Lock()
	s.agentName = name
	s.agentMu.Unlock()
	s.broadcastAgentName(name)
}

func (s *Server) checkActivityTimeout() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.activeMu.Lock()
		if s.activeSource != "" && time.Since(s.activeTime) > 2*time.Second {
			// Reset to no active source after 2 seconds of inactivity
			s.activeSource = ""
		}
		s.activeMu.Unlock()
	}
}

func (s *Server) Stop() error {
	if s.proxy != nil {
		s.proxy.RemoveOutputHandler(s.handlerID)
	}

	s.clientsMu.Lock()
	for _, client := range s.clients {
		client.Close()
	}
	s.clientsMu.Unlock()

	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

func (s *Server) ListClients() []ClientInfo {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	list := make([]ClientInfo, 0, len(s.clients))
	for _, client := range s.clients {
		list = append(list, client.Info())
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Addr == list[j].Addr {
			return list[i].UserAgent < list[j].UserAgent
		}
		return list[i].Addr < list[j].Addr
	})
	return list
}

func (s *Server) DisconnectClient(id string) error {
	s.clientsMu.RLock()
	client := s.clients[id]
	s.clientsMu.RUnlock()
	if client == nil {
		return fmt.Errorf("client not found")
	}
	client.SendDisconnect("Disconnected by server")
	client.CloseWithReason(disconnectCloseCode, "Disconnected by server")
	return nil
}

func (s *Server) getAgentName() string {
	s.agentMu.RLock()
	defer s.agentMu.RUnlock()
	return s.agentName
}

func (s *Server) broadcastAgentName(name string) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for _, client := range s.clients {
		client.SendAgent(name)
	}
}

func (s *Server) BroadcastReset() {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for _, client := range s.clients {
		client.SendReset()
	}
}

func renderIndexHTML(agentName string) string {
	escapedHTML := html.EscapeString(agentName)
	escapedJS := template.JSEscapeString(agentName)
	page := strings.ReplaceAll(indexHTMLTemplate, "{{AGENT_NAME}}", escapedHTML)
	return strings.ReplaceAll(page, "{{AGENT_NAME_JS}}", escapedJS)
}

func clientAddr(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err == nil && host != "" {
		return host
	}
	return remote
}
