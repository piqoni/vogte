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
	Mode       string
}

func New(baseDir, outputFile string) *Application {
	app := &Application{
		baseDir:    baseDir,
		app:        tview.NewApplication(),
		parser:     parser.New(),
		outputFile: outputFile,
		Mode:       "AGENT",
	}
	app.ui = ui.New(app.app, app.messageHandler)
	app.ui.SetModeChangeCallback(app.modeChangeHandler)
	app.ui.SetMode(app.Mode)
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
	switch a.Mode {
	case "ASK":
		a.handleAskMode(message)
	case "AGENT":
		a.handleAgentMode(message)
	}
	go func() {
		p := parser.New()

		result, err := p.ParseProject(a.baseDir)
		if err != nil {
			log.Fatalf("Could not parse the project: %v", err)
		}
		if err := os.WriteFile(a.outputFile, []byte(result), 0644); err != nil {
			log.Fatalf("Error writing to file %s: %v\n", a.outputFile, err)
		}

		log.Printf("Output written to %s\n", a.outputFile)
	}()
}

func (app *Application) modeChangeHandler(newMode string) {
	app.Mode = newMode

	app.postSystemMessage(fmt.Sprintf("Mode changed to: %s", newMode))
}

func (app *Application) postSystemMessage(message string) {
	systemMessage := fmt.Sprintf("\n System: %s", message)
	currentText := app.ui.GetChatText()
	app.ui.SetChatText(currentText + systemMessage)
}

func (app *Application) SetMode(mode string) {
	if app.Mode != mode {
		app.Mode = mode
		app.ui.SetMode(mode)
		app.postSystemMessage(fmt.Sprintf("Mode changed to: %s", mode))
	}
}

func (app *Application) GetMode() string {
	return app.Mode
}

func (app *Application) handleAskMode(message string) { // todo
	response := fmt.Sprintf("\n Assistant (ASK): %s", message)
	currentText := app.ui.GetChatText()
	app.ui.SetChatText(currentText + response)
}

func (app *Application) handleAgentMode(message string) { // todo
	response := fmt.Sprintf("\n Agent (AGENT): %s", message)
	currentText := app.ui.GetChatText()
	app.ui.SetChatText(currentText + response)
}
