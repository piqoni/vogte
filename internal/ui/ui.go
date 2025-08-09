package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type UI struct {
	app        *tview.Application
	root       *tview.Flex
	chatView   *tview.TextView
	inputField *tview.InputField
}

func New(app *tview.Application) *UI {
	ui := &UI{
		app: app,
	}
	ui.initComponents()
	ui.setupLayout()

	return ui
}

func (ui *UI) initComponents() {
	ui.chatView = tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWrap(true)
	ui.chatView.SetBorder(true).SetTitle(" VOGTE ")
	ui.inputField = tview.NewInputField().SetLabel("Message: ")

	ui.addLogo()
}

func (ui *UI) setupLayout() {
	chatArea := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.chatView, 0, 3, false).
		AddItem(ui.inputField, 3, 0, true)
	ui.root = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(chatArea, 0, 1, true)

	ui.root = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(chatArea, 0, 1, true)

}

func (ui *UI) GetRoot() tview.Primitive {
	return ui.root
}

func (ui *UI) addLogo() {
	logo := `██╗   ██╗ ██████╗  ██████╗ ████████╗███████╗
██║   ██║██╔═══██╗██╔════╝ ╚══██╔══╝██╔════╝
██║   ██║██║   ██║██║  ███╗   ██║   █████╗
╚██╗ ██╔╝██║   ██║██║   ██║   ██║   ██╔══╝
 ╚████╔╝ ╚██████╔╝╚███████║   ██║   ███████╗
  ╚═══╝   ╚═════╝  ╚══════╝   ╚═╝   ╚══════╝
  `
	ui.chatView.SetText(logo).SetTextColor(tcell.ColorLightYellow)
}
