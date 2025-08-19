package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type UI struct {
	app          *tview.Application
	root         *tview.Flex
	chatView     *tview.TextArea
	inputField   *tview.InputField
	statusBar    *tview.TextView
	onMessage    func(string)
	onModeChange func(mode string) // "ASK" or "AGENT"
	currentMode  string
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
	ui.updateStatusBar()
}

func (ui *UI) GetMode() string {
	return ui.currentMode
}

func (ui *UI) updateStatusBar() {
	askStyle := "ASK"
	agentStyle := "AGENT"

	if ui.currentMode == "ASK" {
		askStyle = "[::bu]ASK[::-]"
	} else {
		agentStyle = "[::bu]AGENT[::-]"
	}

	statusText := fmt.Sprintf("Model: gpt-5 | dir: vogte | status: 🟢 | Mode: [\"ask\"]%s[\"ask\"] | [\"agent\"]%s[\"agent\"]",
		askStyle, agentStyle)

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
	ui.updateStatusBar()

	ui.chatView = tview.NewTextArea().SetWrap(true)
	ui.chatView.SetBorder(false)
	ui.addLogo()

	ui.inputField = tview.NewInputField().SetLabel("Message: ")
	ui.inputField.SetBorder(true)
	ui.inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			message := ui.inputField.GetText()
			if message != "" {
				text := fmt.Sprintf("\n You: %s", message)
				currentText := ui.chatView.GetText()
				ui.chatView.SetText(currentText+text, true)
				ui.onMessage(message)
				ui.inputField.SetText("")
			}
		}
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

func (ui *UI) addLogo() {
	logo := `
 ██╗   ██╗  ██████╗   ██████╗ ████████╗ ███████╗
 ██║   ██║ ██╔═══██╗ ██╔════╝ ╚══██╔══╝ ██╔════╝
 ██║   ██║ ██║   ██║ ██║  ███╗   ██║    █████╗
 ╚██╗ ██╔╝ ██║   ██║ ██║   ██║   ██║    ██╔══╝
  ╚████╔╝  ╚██████╔╝ ╚███████║   ██║    ███████╗
   ╚═══╝    ╚═════╝   ╚══════╝   ╚═╝    ╚══════╝
	 `
	ui.chatView.SetText(logo, false)
}

// GetChatText returns the current text in the chat view
func (ui *UI) GetChatText() string {
	return ui.chatView.GetText()
}

// SetChatText sets the text in the chat view
func (ui *UI) SetChatText(text string) {
	ui.chatView.SetText(text, true)
}

// AppendChatText appends text to the chat view
func (ui *UI) AppendChatText(text string) {
	currentText := ui.chatView.GetText()
	ui.chatView.SetText(currentText+text, true)
}
