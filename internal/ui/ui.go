package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ProjectState int

const (
	StateUnknown ProjectState = iota
	StateHealthy
	StateError
)

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
	chatBuffer   string
}

func New(app *tview.Application, onMessage func(string)) *UI {
	ui := &UI{
		app:         app,
		onMessage:   onMessage,
		currentMode: "AGENT",
	}
	ui.initComponents()
	ui.setupLayout()
	return ui
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

	statusText := fmt.Sprintf(
		"Model: %s | Status: %s | Mode: [\"ask\"]%s[\"ask\"] ↔ [\"agent\"]%s[\"agent\"] | Dir: %s ",
		"gpt-5",
		ui.currentState.Emojify(),
		askStyle,
		agentStyle,
		ui.baseDir,
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
					if ui.onModeChange != nil {
						ui.onModeChange("ASK")
					}
				} else if region == "agent" && ui.currentMode != "AGENT" {
					ui.SetMode("AGENT")
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
	ui.inputField.SetBorder(true).SetTitle("Message: (Ctrl+Enter to Submit) ")
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
