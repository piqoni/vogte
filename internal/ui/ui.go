package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type UI struct {
	app        *tview.Application
	root       *tview.Flex
	chatView   *tview.TextArea
	inputField *tview.InputField
	statusBar  *tview.TextView
	onMessage  func(string)
}

func New(app *tview.Application, onMessage func(string)) *UI {
	ui := &UI{
		app:       app,
		onMessage: onMessage,
	}
	ui.initComponents()
	ui.setupLayout()

	return ui
}

func (ui *UI) initComponents() {
	ui.statusBar = tview.NewTextView().SetText("Status: 🟢") // TODO
	ui.chatView = tview.NewTextArea().SetWrap(true)
	ui.chatView.SetBorder(true).SetTitle(" VOGTE ")
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
				// ui.chatView.ScrollToEnd()

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
 ██╗   ██╗  ██████╗   ██████╗ ████████╗███████╗
 ██║   ██║ ██╔═══██╗ ██╔════╝ ╚══██╔══╝██╔════╝
 ██║   ██║ ██║   ██║ ██║  ███╗   ██║   █████╗
 ╚██╗ ██╔╝ ██║   ██║ ██║   ██║   ██║   ██╔══╝
  ╚████╔╝  ╚██████╔╝ ╚███████║   ██║   ███████╗
   ╚═══╝    ╚═════╝   ╚══════╝   ╚═╝   ╚══════╝
	 `
	ui.chatView.SetText(logo, false)
}
