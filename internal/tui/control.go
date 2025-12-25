package tui

import (
	"fmt"

	"github.com/biliqiqi/ac2/internal/pool"
	"github.com/biliqiqi/ac2/internal/webterm"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ActionType int

const (
	ActionNone ActionType = iota
	ActionResume
	ActionQuit
	ActionSwitch
)

type Action struct {
	Type            ActionType
	AgentID         string
	TargetAgentType string
}

type ControlMode struct {
	agentPool    *pool.AgentPool
	currentAgent *pool.AgentInstance
	passthrough  *Passthrough

	app         *tview.Application
	action      Action
	menuCapture func(event *tcell.EventKey) *tcell.EventKey
	clientIDs   []string
	clientInfo  map[string]webterm.ClientInfo
}

func NewControlMode(agentPool *pool.AgentPool, currentAgent *pool.AgentInstance, passthrough *Passthrough) *ControlMode {
	return &ControlMode{
		agentPool:    agentPool,
		currentAgent: currentAgent,
		passthrough:  passthrough,
		action:       Action{Type: ActionNone},
	}
}

func (c *ControlMode) Run() Action {
	c.app = tview.NewApplication()

	root := c.buildUI()

	c.app.SetRoot(root, true)
	c.app.EnableMouse(false)

	if err := c.app.Run(); err != nil {
		return Action{Type: ActionResume}
	}

	return c.action
}

func (c *ControlMode) RunExitConfirm() Action {
	c.app = tview.NewApplication()

	root := c.buildUI()
	c.app.SetRoot(root, true)
	c.app.EnableMouse(false)
	c.showExitConfirm(true)

	if err := c.app.Run(); err != nil {
		return Action{Type: ActionResume}
	}

	return c.action
}

func (c *ControlMode) buildUI() tview.Primitive {
	statusView := tview.NewTextView()
	statusView.SetDynamicColors(true)
	statusView.SetBorder(true)
	statusView.SetTitle(" Status ")
	statusView.SetText(c.buildStatusText())

	clientsList := c.buildClientsList()

	canResume := c.currentAgent != nil && c.currentAgent.Status == pool.StatusRunning
	menuBar := tview.NewTextView()
	menuBar.SetDynamicColors(true)
	menuBar.SetTextAlign(tview.AlignCenter)
	menuBar.SetBorder(true)
	menuBar.SetTitle(" Menu ")

	resumeLabel := "[gray]r Resume[-]"
	if canResume {
		resumeLabel = "[white]r Resume[-]"
	}
	menuBar.SetText(fmt.Sprintf("%s   [white]s Switch Agent[-]   [white]f Refresh[-]   [white]d Disconnect Client[-]   [white]h Help[-]   [white]q Quit[-]", resumeLabel))

	// Layout
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(statusView, 3, 0, false).
		AddItem(clientsList, 0, 1, true).
		AddItem(menuBar, 3, 0, false)

	// Global key handler
	c.menuCapture = func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'r', 'R':
				if canResume {
					c.action = Action{Type: ActionResume}
					c.app.Stop()
				}
				return nil
			case 's', 'S':
				c.showSwitchAgentMenu()
				return nil
			case 'd', 'D':
				c.disconnectSelectedClient()
				return nil
			case 'f', 'F':
				c.refreshUI()
				return nil
			case 'h', 'H':
				c.showHelp()
				return nil
			case 'q', 'Q':
				c.handleQuit()
				return nil
			}
		}
		switch event.Key() {
		case tcell.KeyEscape:
			if canResume {
				c.action = Action{Type: ActionResume}
				c.app.Stop()
			}
			return nil
		}
		return event
	}
	c.app.SetInputCapture(c.menuCapture)
	c.app.SetFocus(clientsList)

	return flex
}

func (c *ControlMode) buildStatusText() string {
	name := "Unknown"
	if c.currentAgent != nil {
		if c.currentAgent.Name != "" {
			name = c.currentAgent.Name
		} else if c.currentAgent.Type != "" {
			name = c.currentAgent.Type
		} else if c.currentAgent.ID != "" {
			name = c.currentAgent.ID
		}
	}
	return fmt.Sprintf(" Current Agent: [white::b]%s[-]\n", name)
}

func (c *ControlMode) buildClientsList() *tview.List {
	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(" Web Clients ")
	list.ShowSecondaryText(true)
	list.SetBackgroundColor(tcell.ColorDefault)
	list.SetBorderColor(tcell.ColorDefault)
	list.SetTitleColor(tcell.ColorDefault)

	clients := c.getWebClients()
	c.clientIDs = c.clientIDs[:0]
	c.clientInfo = make(map[string]webterm.ClientInfo)

	if len(clients) == 0 {
		list.AddItem("No web clients connected", "", 0, nil)
		list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape {
				c.restoreMenuCapture()
				c.app.SetRoot(c.buildUI(), true)
				return nil
			}
			return event
		})
		return list
	}

	for _, client := range clients {
		clientID := client.ID
		label := client.Addr
		if label == "" {
			label = "Unknown"
		}
		secondary := client.UserAgent
		if secondary == "" {
			secondary = "Unknown User Agent"
		}
		c.clientIDs = append(c.clientIDs, clientID)
		c.clientInfo[clientID] = client
		list.AddItem(label, secondary, 0, func() {
			c.showDisconnectConfirm(clientID)
		})
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			if c.currentAgent != nil && c.currentAgent.Status == pool.StatusRunning {
				c.action = Action{Type: ActionResume}
				c.app.Stop()
				return nil
			}
			c.restoreMenuCapture()
			c.app.SetRoot(c.buildUI(), true)
			return nil
		}
		return event
	})

	return list
}

func (c *ControlMode) showSwitchAgentMenu() {
	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(" Switch Agent ")
	list.ShowSecondaryText(false)

	available := c.agentPool.GetAvailableAgents()
	for _, agent := range available {
		agentType := string(agent.Type)
		name := agent.Name
		list.AddItem(name, "", 0, func() {
			c.action = Action{
				Type:            ActionSwitch,
				TargetAgentType: agentType,
			}
			c.app.Stop()
		})
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			c.restoreMenuCapture()
			c.app.SetRoot(c.buildUI(), true)
			return nil
		}
		return event
	})

	c.suspendMenuCapture()
	c.app.SetRoot(list, true)
}

func (c *ControlMode) disconnectSelectedClient() {
	if len(c.clientIDs) == 0 {
		return
	}
	list := c.app.GetFocus()
	if list == nil {
		return
	}
	clientList, ok := list.(*tview.List)
	if !ok {
		return
	}
	index := clientList.GetCurrentItem()
	if index < 0 || index >= len(c.clientIDs) {
		return
	}
	clientID := c.clientIDs[index]
	if clientID == "" {
		return
	}
	c.showDisconnectConfirm(clientID)
}

func (c *ControlMode) refreshUI() {
	c.restoreMenuCapture()
	c.app.SetRoot(c.buildUI(), true)
}

func (c *ControlMode) showDisconnectConfirm(clientID string) {
	info, ok := c.clientInfo[clientID]
	if !ok {
		return
	}
	confirm := tview.NewModal()
	back := c.buildUI()
	c.styleModal(confirm, back)
	confirm.SetText(fmt.Sprintf("Disconnect web client?\n%s\n%s", info.Addr, info.UserAgent))
	confirm.AddButtons([]string{"Cancel", "Disconnect"})
	confirm.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		switch buttonIndex {
		case 0:
			c.restoreMenuCapture()
			c.app.SetRoot(c.buildUI(), true)
		case 1:
			if err := c.disconnectClient(clientID); err != nil {
				c.showError(fmt.Sprintf("Disconnect failed: %v", err))
				return
			}
			c.restoreMenuCapture()
			c.app.SetRoot(c.buildUI(), true)
		}
	})
	c.app.SetRoot(confirm, true)
}

func (c *ControlMode) disconnectClient(clientID string) error {
	if c.passthrough == nil || c.passthrough.webServer == nil {
		return fmt.Errorf("web terminal server not available")
	}
	return c.passthrough.webServer.DisconnectClient(clientID)
}

func (c *ControlMode) getWebClients() []webterm.ClientInfo {
	if c.passthrough == nil || c.passthrough.webServer == nil {
		return nil
	}
	return c.passthrough.webServer.ListClients()
}

func (c *ControlMode) showError(message string) {
	modal := tview.NewModal()
	back := c.buildUI()
	c.styleModal(modal, back)
	modal.SetText(fmt.Sprintf("[red]Error:[white]\n%s", message))
	modal.AddButtons([]string{"OK"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		c.restoreMenuCapture()
		c.app.SetRoot(c.buildUI(), true)
	})
	c.app.SetRoot(modal, true)
}

func (c *ControlMode) showExitConfirm(resumeOnCancel bool) {
	modal := tview.NewModal()
	back := c.buildUI()
	c.styleModal(modal, back)
	modal.SetText("Quit will terminate all agents.\n\nContinue?")
	modal.AddButtons([]string{"Cancel", "Quit"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		switch buttonIndex {
		case 0: // Cancel
			if resumeOnCancel {
				c.restoreMenuCapture()
				c.action = Action{Type: ActionResume}
				c.app.Stop()
				return
			}
			c.restoreMenuCapture()
			c.app.SetRoot(c.buildUI(), true)
		case 1: // Quit
			c.restoreMenuCapture()
			c.action = Action{Type: ActionQuit}
			c.app.Stop()
		}
	})

	if resumeOnCancel {
		modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape {
				c.restoreMenuCapture()
				c.action = Action{Type: ActionResume}
				c.app.Stop()
				return nil
			}
			return event
		})
	}

	c.app.SetRoot(modal, true)
}

func (c *ControlMode) handleQuit() {
	agents := c.agentPool.ListAll()
	running := 0
	for _, agent := range agents {
		if agent.Status == pool.StatusRunning {
			running++
		}
	}

	if running == 0 {
		c.action = Action{Type: ActionQuit}
		c.app.Stop()
		return
	}

	c.showExitConfirm(false)
}

func (c *ControlMode) showHelp() {
	help := "" +
		"Resume: back to current agent\n" +
		"Switch Agent: switch current agent to another\n" +
		"Web Clients: select and press Enter to disconnect\n" +
		"Disconnect Client: press d to disconnect selected client\n" +
		"Refresh: reload web client list\n" +
		"Help: show this help menu\n" +
		"Quit: exit ac2\n\n" +
		"Shortcuts: r/s/f/d/h/q, Esc: back (when resume is available)"

	modal := tview.NewModal()
	back := c.buildUI()
	c.styleModal(modal, back)
	modal.SetText(help)
	modal.AddButtons([]string{"Back"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		c.restoreMenuCapture()
		c.app.SetRoot(c.buildUI(), true)
	})

	c.app.SetRoot(modal, true)
}

func (c *ControlMode) styleModal(modal *tview.Modal, back tview.Primitive) {
	modal.SetBackgroundColor(tcell.ColorBlack)
	modal.SetBorderColor(tcell.ColorWhite)
	modal.SetTitleColor(tcell.ColorWhite)
	modal.SetTextColor(tcell.ColorWhite)
	modal.SetButtonBackgroundColor(tcell.ColorBlack)
	modal.SetButtonTextColor(tcell.ColorWhite)
	c.suspendMenuCapture()
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			c.restoreMenuCapture()
			c.app.SetRoot(back, true)
			return nil
		}
		return event
	})
}

func (c *ControlMode) suspendMenuCapture() {
	c.app.SetInputCapture(nil)
}

func (c *ControlMode) restoreMenuCapture() {
	if c.menuCapture != nil {
		c.app.SetInputCapture(c.menuCapture)
	}
}
