package app

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/piqoni/vogte/internal/parser"
	"github.com/piqoni/vogte/internal/ui"
	"github.com/rivo/tview"
)

type Application struct {
	baseDir string
	app     *tview.Application
	ui      *ui.UI
	parser  *parser.Parser
}

func New(baseDir string) *Application {
	app := &Application{
		baseDir: baseDir,
		app:     tview.NewApplication(),
		parser:  parser.New(),
	}
	app.ui = ui.New(app.app, app.messageHandler)
	return app
}

func (a *Application) Run() error {

	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlC:
			a.app.Stop()
			return nil
		}
		return event
	})

	if err := a.app.SetRoot(a.ui.GetRoot(), true).ForceDraw().EnableMouse(true).Run(); err != nil {
		return fmt.Errorf("failed to run: %v", err)
	}

	return nil
}

func (a *Application) messageHandler(message string) {
	fmt.Println(message) //TODO
}
