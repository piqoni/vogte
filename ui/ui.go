package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ProjectState int

const (
	StateUnknown ProjectState = iota
	StateHealthy
	StateError
)

type UI struct {
	app          *tview.Application
	root         *tview.Flex
	chatView     *tview.TextView
	inputField   *tview.TextArea
	statusBar    *tview.TextView
	onMessage    func(string)
	onModeChange func(mode string) // "ASK" or "AGENT"
	currentMode  string
	currentState ProjectState
	baseDir      string
	modelName    string

	chatBuffer string
	// spinner animation
	isLoading      bool
	animationFrame int
	stopAnimation  chan struct{}
}

var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

func New(app *tview.Application, onMessage func(string)) *UI {
	ui := &UI{
		app:       app,
		onMessage: onMessage,
	}
	ui.initComponents()
	ui.setupLayout()
	return ui
}

func (s ProjectState) Emojify() string {
	switch s {
	case StateHealthy:
		return "🟢"
	case StateError:
		return "🔴"
	default:
		return "⚪️"
	}
}

func (ui *UI) StartLoading() {
	if ui.isLoading {
		return // Already loading
	}
	ui.isLoading = true
	ui.stopAnimation = make(chan struct{})

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ui.animationFrame = (ui.animationFrame + 1) % len(spinnerFrames)
				ui.app.QueueUpdateDraw(ui.RefreshStatusBar)
			case <-ui.stopAnimation:
				return
			}
		}
	}()
}

func (ui *UI) StopLoading() {
	if !ui.isLoading {
		return
	}
	ui.isLoading = false
	close(ui.stopAnimation)
	ui.app.QueueUpdateDraw(ui.RefreshStatusBar)
}

func (ui *UI) SetModeChangeCallback(callback func(mode string)) {
	ui.onModeChange = callback
}

func (ui *UI) SetMode(mode string) {
	ui.currentMode = mode
	ui.RefreshStatusBar()
}

func (ui *UI) SetBaseDir(dir string) {
	ui.baseDir = dir
	ui.RefreshStatusBar()
}
func (ui *UI) SetModelName(name string) {
	ui.modelName = name
	ui.RefreshStatusBar()
}

func (ui *UI) GetMode() string {
	return ui.currentMode
}
func (ui *UI) SetState(state ProjectState) {
	ui.currentState = state
	ui.RefreshStatusBar()
}
func (ui *UI) RefreshStatusBar() {
	askStyle := "ASK"
	agentStyle := "AGENT"

	if ui.currentMode == "ASK" {
		askStyle = "[::bu]ASK[::-]"
	} else {
		agentStyle = "[::bu]AGENT[::-]"
	}

	loadingIndicator := ""
	if ui.isLoading {
		loadingIndicator = " " + string(spinnerFrames[ui.animationFrame])
	}

	dirDisplay := ui.baseDir
	if dirDisplay != "" {
		dirDisplay = filepath.Base(dirDisplay)
	}

	modelDisplay := ui.modelName
	if strings.TrimSpace(modelDisplay) == "" {
		modelDisplay = "-"
	}

	if len(modelDisplay) > 20 {
		modelDisplay = modelDisplay[:17] + "..." // chop if name too long (bedrock string)
	}

	statusText := fmt.Sprintf(
		"%s Status: %s | Dir: %s | Model: %s | Mode: [\"ask\"]%s[\"ask\"] - [\"agent\"]%s[\"agent\"]",
		loadingIndicator,
		ui.currentState.Emojify(),
		dirDisplay,
		modelDisplay,
		askStyle,
		agentStyle,
	)

	ui.statusBar.SetText(statusText)
}

func (ui *UI) initComponents() {
	ui.statusBar = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false).
		SetHighlightedFunc(func(added, removed, remaining []string) {
			// Handle clicks on regions
			if len(added) > 0 {
				region := added[0]
				if region == "ask" && ui.currentMode != "ASK" {
					ui.SetMode("ASK")
					ui.AppendChatText("\nMode set to ASK")
					if ui.onModeChange != nil {
						ui.onModeChange("ASK")
					}
				} else if region == "agent" && ui.currentMode != "AGENT" {
					ui.SetMode("AGENT")
					ui.AppendChatText("\nMode set to AGENT")

					if ui.onModeChange != nil {
						ui.onModeChange("AGENT")
					}
				}
			}
		}).
		SetTextStyle(tcell.StyleDefault.Background(tcell.ColorBlack))

	// Initialize the status bar text
	ui.RefreshStatusBar()

	ui.chatView = tview.NewTextView().SetWrap(true).SetDynamicColors(true)
	ui.chatView.SetBorder(false)
	ui.addLogo()

	ui.inputField = tview.NewTextArea().SetWrap(true)
	ui.inputField.SetBorder(true).SetTitle("Message: ")
	ui.inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			if true { //FIXME event.Modifiers()&tcell.ModCtrl != 0 {
				message := ui.inputField.GetText()
				text := fmt.Sprintf("\n You: %s", message)
				ui.AppendChatText(text)
				ui.onMessage(message)
				ui.inputField.SetText("", false)
				return nil
			}
		}
		return event
	})
}

func (ui *UI) setupLayout() {
	chatArea := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.chatView, 0, 3, false).
		AddItem(ui.inputField, 8, 1, true)

	ui.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.statusBar, 1, 1, false).
		AddItem(chatArea, 0, 1, true)
}

func (ui *UI) GetRoot() tview.Primitive {
	return ui.root
}

func (ui *UI) UpdateState(state ProjectState) {
	ui.statusBar.SetText(state.Emojify())
}

func (ui *UI) addLogo() {
	logo := `
 ██╗   ██╗  ██████╗   ██████╗ ████████╗ ███████╗
 ██║   ██║ ██╔═══██╗ ██╔════╝ ╚══██╔══╝ ██╔════╝
 ██║   ██║ ██║   ██║ ██║  ███╗   ██║    █████╗
 ╚██╗ ██╔╝ ██║   ██║ ██║   ██║   ██║    ██╔══╝
  ╚████╔╝  ╚██████╔╝ ╚███████║   ██║    ███████╗
   ╚═══╝    ╚═════╝   ╚══════╝   ╚═╝    ╚══════╝
	 `
	ui.SetChatText(logo)
}

func (ui *UI) GetChatText() string {
	return ui.chatBuffer
}

func (ui *UI) colorizeText(text string) string {
	lines := strings.Split(text, "\n")
	var colorizedLines []string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "+") {
			colorizedLines = append(colorizedLines, "[black:green]+[-:-]"+line[1:])
		} else if strings.HasPrefix(trimmedLine, "-") {
			colorizedLines = append(colorizedLines, "[black:red]-[-:-]"+line[1:])
		} else {
			colorizedLines = append(colorizedLines, line)
		}
	}

	return strings.Join(colorizedLines, "\n")
}

func (ui *UI) SetChatText(text string) {
	ui.chatBuffer = text
	colorizedText := ui.colorizeText(text)
	ui.chatView.SetText(colorizedText)
}

func (ui *UI) AppendChatText(text string) {
	ui.chatBuffer += text
	colorizedText := ui.colorizeText(ui.chatBuffer)
	ui.chatView.SetText(colorizedText)
}
