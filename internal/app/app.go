package app

import (
	"fmt"
	"log"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/piqoni/vogte/internal/parser"
	"github.com/piqoni/vogte/internal/ui"
	"github.com/rivo/tview"
)

type Application struct {
	baseDir    string
	outputFile string
	app        *tview.Application
	ui         *ui.UI
	parser     *parser.Parser
}

func New(baseDir, outputFile string) *Application {
	app := &Application{
		baseDir:    baseDir,
		app:        tview.NewApplication(),
		parser:     parser.New(),
		outputFile: outputFile,
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
	p := parser.New()

	result, err := p.ParseProject(a.baseDir)
	if err != nil {
		log.Fatalf("Could not parse the project: %v", err)
	}
	if err := os.WriteFile(a.outputFile, []byte(result), 0644); err != nil {
		log.Fatalf("Error writing to file %s: %v\n", a.outputFile, err)
	}

	log.Printf("Output written to %s\n", a.outputFile)
}
