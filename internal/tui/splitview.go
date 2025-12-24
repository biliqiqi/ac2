package tui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/biliqiqi/ac2/internal/detector"
	"github.com/biliqiqi/ac2/internal/pty"
	creackpty "github.com/creack/pty"
	"github.com/gdamore/tcell/v2"
	"github.com/hinshun/vt10x"
	"github.com/rivo/tview"
	"golang.org/x/term"
)

const (
	sideWidth   = 26
	defaultCols = 80
	defaultRows = 24
)

type SplitView struct {
	agents     []detector.AgentInfo
	entryAgent *detector.AgentInfo

	proxy *pty.Proxy

	app      *tview.Application
	sidebar  *tview.TextView
	termView *terminalView

	vt   vt10x.Terminal
	vtMu sync.Mutex

	quit     chan struct{}
	stopOnce sync.Once

	sizeMu   sync.Mutex
	lastRows uint16
	lastCols uint16
}

func NewSplitView(agents []detector.AgentInfo, entry *detector.AgentInfo) *SplitView {
	return &SplitView{
		agents:     agents,
		entryAgent: entry,
		quit:       make(chan struct{}),
	}
}

func (sv *SplitView) Run() error {
	if sv.entryAgent == nil {
		return fmt.Errorf("no entry agent selected")
	}

	sv.app = tview.NewApplication()

	// Initialize proxy first so we can use it as a writer for vt10x
	sv.proxy = pty.NewProxy(sv.entryAgent.Command)
	sv.proxy.SetOutputHandler(sv.handleOutput)
	sv.proxy.SetExitHandler(func(err error) {
		f, _ := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			_, _ = fmt.Fprintf(f, "[SplitView] Proxy exited with error: %v\n", err)
		} else {
			_, _ = fmt.Fprintln(f, "[SplitView] Proxy exited successfully")
		}
		_ = f.Close()
		sv.stop()
	})

	// Initialize VT with the proxy as the writer for response codes (e.g. cursor position)
	sv.vt = vt10x.New(
		vt10x.WithSize(defaultCols, defaultRows),
		vt10x.WithWriter(sv.proxy),
	)
	sv.lastCols = defaultCols
	sv.lastRows = defaultRows

	sv.setupViews()

	// Direct update for initialization
	sv.sidebar.SetText(sv.getSidebarText())

	// Calculate initial size based on physical terminal size
	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		cols = defaultCols
		rows = defaultRows
	}

	// Adjust for sidebar
	mainCols := cols - sideWidth - 2
	if mainCols < 10 {
		mainCols = 10
	}
	// Adjust for borders
	mainRows := rows - 2
	if mainRows < 5 {
		mainRows = 5
	}

	initialSize := &creackpty.Winsize{
		Rows: uint16(mainRows),
		Cols: uint16(mainCols),
		X:    0,
		Y:    0,
	}

	f, _ := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	_, _ = fmt.Fprintf(f, "[SplitView] Initial PTY size: %dx%d\n", mainCols, mainRows)
	_ = f.Close()

	f, _ = os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	_, _ = fmt.Fprintf(f, "[SplitView] Starting command: %s\n", sv.entryAgent.Command)
	_ = f.Close()

	if err := sv.proxy.Start(initialSize); err != nil {
		return fmt.Errorf("failed to start agent: %w", err)
	}

	go sv.statusWatcher()

	if err := sv.app.Run(); err != nil {
		sv.stop()
		return err
	}

	sv.stop()
	return nil
}

func (sv *SplitView) setupViews() {
	sv.sidebar = tview.NewTextView()
	sv.sidebar.SetDynamicColors(true)
	sv.sidebar.SetWrap(false)
	sv.sidebar.SetBorder(true)
	sv.sidebar.SetTitle(" ac2 ")

	sv.termView = newTerminalView(&sv.vtMu, sv.vt)
	sv.termView.SetBorder(true)
	sv.termView.SetTitle(" Agent ")

	root := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(sv.sidebar, sideWidth+2, 0, false).
		AddItem(sv.termView, 0, 1, true)

	sv.app.SetRoot(root, true)
	sv.app.SetFocus(sv.termView)
	sv.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		sv.resizePTY(screen)
		return false
	})
	sv.app.SetInputCapture(sv.handleInput)
}

func (sv *SplitView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyCtrlQ {
		sv.stop()
		return nil
	}

	if event.Key() == tcell.KeyCtrlC && (sv.proxy == nil || sv.proxy.Status() != pty.StatusRunning) {
		// Fallback exit if agent is gone; otherwise forward to agent.
		sv.stop()
		return nil
	}

	if data := encodeKey(event); len(data) > 0 && sv.proxy != nil {
		_, _ = sv.proxy.Write(data)
		return nil
	}

	return event
}

func encodeKey(event *tcell.EventKey) []byte {
	switch event.Key() {
	case tcell.KeyRune:
		return []byte(string(event.Rune()))
	case tcell.KeyEnter:
		return []byte("\r")
	case tcell.KeyTAB:
		return []byte("\t")
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return []byte{0x7f}
	case tcell.KeyCtrlC:
		return []byte{0x03}
	case tcell.KeyCtrlD:
		return []byte{0x04}
	case tcell.KeyEsc:
		return []byte{0x1b}
	case tcell.KeyUp:
		return []byte("\x1b[A")
	case tcell.KeyDown:
		return []byte("\x1b[B")
	case tcell.KeyRight:
		return []byte("\x1b[C")
	case tcell.KeyLeft:
		return []byte("\x1b[D")
	case tcell.KeyHome:
		return []byte("\x1b[H")
	case tcell.KeyEnd:
		return []byte("\x1b[F")
	case tcell.KeyPgUp:
		return []byte("\x1b[5~")
	case tcell.KeyPgDn:
		return []byte("\x1b[6~")
	case tcell.KeyDelete:
		return []byte("\x1b[3~")
	case tcell.KeyInsert:
		return []byte("\x1b[2~")
	}
	return nil
}

func (sv *SplitView) handleOutput(data []byte) {
	f, _ := os.OpenFile("agent_output.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	_, _ = f.Write(data)
	_ = f.Close()

	copyData := append([]byte(nil), data...)

	sv.vtMu.Lock()
	_, _ = sv.vt.Write(copyData)
	sv.vtMu.Unlock()

	sv.app.QueueUpdateDraw(func() {
	})
}

type terminalView struct {
	*tview.Box
	vtMu *sync.Mutex
	vt   vt10x.Terminal
}

func newTerminalView(vtMu *sync.Mutex, vt vt10x.Terminal) *terminalView {
	return &terminalView{
		Box:  tview.NewBox(),
		vtMu: vtMu,
		vt:   vt,
	}
}

func (tv *terminalView) Draw(screen tcell.Screen) {
	tv.DrawForSubclass(screen, tv)

	x, y, width, height := tv.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}

	tv.vtMu.Lock()
	tv.vt.Lock()
	defer func() {
		tv.vt.Unlock()
		tv.vtMu.Unlock()
	}()

	cols, rows := tv.vt.Size()
	if cols > width {
		cols = width
	}
	if rows > height {
		rows = height
	}

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			cell := tv.vt.Cell(col, row)
			r := cell.Char
			if r == 0 {
				r = ' '
			}
			style := tcell.StyleDefault.Foreground(termColor(cell.FG)).Background(termColor(cell.BG))
			screen.SetContent(x+col, y+row, r, nil, style)
		}
	}

	if tv.vt.CursorVisible() {
		cursor := tv.vt.Cursor()
		curX := x + cursor.X
		curY := y + cursor.Y
		if curX >= x && curX < x+width && curY >= y && curY < y+height {
			screen.ShowCursor(curX, curY)
		}
	}
}

func termColor(c vt10x.Color) tcell.Color {
	switch c {
	case vt10x.DefaultFG, vt10x.DefaultBG, vt10x.DefaultCursor:
		return tcell.ColorDefault
	}

	if c.ANSI() {
		switch c {
		case vt10x.Black:
			return tcell.ColorBlack
		case vt10x.Red:
			return tcell.ColorMaroon
		case vt10x.Green:
			return tcell.ColorGreen
		case vt10x.Yellow:
			return tcell.ColorOlive
		case vt10x.Blue:
			return tcell.ColorNavy
		case vt10x.Magenta:
			return tcell.ColorPurple
		case vt10x.Cyan:
			return tcell.ColorTeal
		case vt10x.LightGrey:
			return tcell.ColorSilver
		case vt10x.DarkGrey:
			return tcell.ColorGray
		case vt10x.LightRed:
			return tcell.ColorRed
		case vt10x.LightGreen:
			return tcell.ColorLime
		case vt10x.LightYellow:
			return tcell.ColorYellow
		case vt10x.LightBlue:
			return tcell.ColorBlue
		case vt10x.LightMagenta:
			return tcell.ColorFuchsia
		case vt10x.LightCyan:
			return tcell.ColorAqua
		case vt10x.White:
			return tcell.ColorWhite
		}
	}

	return tcell.PaletteColor(int(c))
}

func (sv *SplitView) statusWatcher() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-sv.quit:
			return
		case <-ticker.C:
			sv.updateSidebar()
		}
	}
}

func (sv *SplitView) updateSidebar() {
	text := sv.getSidebarText()
	sv.app.QueueUpdateDraw(func() {
		sv.sidebar.SetText(text)
	})
}

func (sv *SplitView) getSidebarText() string {
	var b strings.Builder

	b.WriteString("[cyan::b]ac2[-]\n")
	b.WriteString("[gray]────────────────────────[-]\n")
	b.WriteString("[white::b]Agents[-]\n")

	for _, agent := range sv.agents {
		indicator := "[gray]○[-]"
		nameColor := "[gray]"

		if agent.Found {
			nameColor = "[white]"
			if sv.entryAgent != nil && agent.Type == sv.entryAgent.Type {
				status := ""
				if sv.proxy != nil {
					switch sv.proxy.Status() {
					case pty.StatusRunning:
						status = "[green]●[-]"
					case pty.StatusStarting:
						status = "[yellow]●[-]"
					default:
						status = "[red]●[-]"
					}
				}
				if status != "" {
					indicator = status
				}
			}
		}

		name := agent.Name
		if !agent.Found {
			name += " (N/A)"
		}

		fmt.Fprintf(&b, " %s %s%s[-]\n", indicator, nameColor, name)
	}

	b.WriteString("[gray]────────────────────────[-]\n")
	b.WriteString("[white::b]Entry[-]\n")
	if sv.entryAgent != nil {
		fmt.Fprintf(&b, "[gray] Agent: [white]%s[-]\n", sv.entryAgent.Name)
	}
	if sv.proxy != nil {
		status := sv.proxy.Status()
		color := "red"
		switch status {
		case pty.StatusRunning:
			color = "green"
		case pty.StatusStarting:
			color = "yellow"
		}
		fmt.Fprintf(&b, "[gray] Status: [%s]%s[-]\n", color, status.String())
	}

	b.WriteString("[gray]────────────────────────[-]\n")
	b.WriteString("[white::b]Keys[-]\n")
	b.WriteString("[gray] Ctrl+Q quit app[-]\n")
	b.WriteString("[gray] Ctrl+C/D sent to agent[-]\n")

	return b.String()
}

func (sv *SplitView) resizePTY(screen tcell.Screen) {
	cols, rows := screen.Size()

	f, _ := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	_, _ = fmt.Fprintf(f, "[SplitView] resizePTY: screen size: %dx%d\n", cols, rows)
	_ = f.Close()

	left := sideWidth + 2 // sidebar width + border
	mainCols := cols - left
	if mainCols < 10 {
		mainCols = 10
	}
	mainRows := rows
	if mainRows < 5 {
		mainRows = 5
	}

	innerCols := mainCols - 2 // account for main border
	innerRows := mainRows - 2
	if innerCols < 1 {
		innerCols = 1
	}
	if innerRows < 1 {
		innerRows = 1
	}

	newRows := uint16(innerRows)
	newCols := uint16(innerCols)

	sv.sizeMu.Lock()
	same := sv.lastRows == newRows && sv.lastCols == newCols
	if !same {
		sv.lastRows = newRows
		sv.lastCols = newCols
	}
	sv.sizeMu.Unlock()

	if same {
		return
	}

	if sv.proxy != nil {
		_ = sv.proxy.Resize(newRows, newCols)
	}

	sv.vtMu.Lock()
	sv.vt.Resize(int(newCols), int(newRows))
	sv.vtMu.Unlock()
}

func (sv *SplitView) stop() {
	sv.stopOnce.Do(func() {
		close(sv.quit)
		if sv.proxy != nil {
			_ = sv.proxy.Stop()
		}
		if sv.app != nil {
			sv.app.Stop()
		}
	})
}
