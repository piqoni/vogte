package app

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

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

	stateMu sync.RWMutex
	state   ui.ProjectState
	stateCh chan ui.ProjectState
}

func New(baseDir, outputFile string) *Application {
	app := &Application{
		baseDir:    baseDir,
		app:        tview.NewApplication(),
		parser:     parser.New(),
		outputFile: outputFile,
		Mode:       "AGENT",
		state:      ui.StateUnknown,
		stateCh:    make(chan ui.ProjectState, 1),
	}
	app.ui = ui.New(app.app, app.messageHandler)
	app.ui.SetModeChangeCallback(app.modeChangeHandler)
	app.ui.SetMode(app.Mode)
	app.ui.SetBaseDir(baseDir)
	return app
}

func (a *Application) Run() error {

	go a.stateMonitor()
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
	if strings.ToLower(message) == "quit" || message == "q" || strings.ToLower(message) == "exit" {
		a.app.Stop()
	}
	switch a.Mode {
	case "ASK":
		a.handleAskMode(message)
	case "AGENT":
		a.handleAgentMode(message)
	}
	go func() {
		structure, err := a.parser.ParseProject(a.baseDir)
		a.setState(ui.StateError)
		if err != nil {
			log.Fatalf("Could not parse the project: %v", err)
		}

		_ = structure // TODO

	}()
	a.postSystemMessage("this is a test") // TODO

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

func (a *Application) Parse() (string, error) {
	return a.parser.ParseProject(a.baseDir)
}

func (a *Application) stateMonitor() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case newState := <-a.stateCh:
			a.stateMu.RLock()
			a.state = newState
			a.stateMu.RUnlock()

			a.app.QueueUpdateDraw(func() {
				a.ui.SetState(newState)
			})

		case <-ticker.C:
			// TODO periodic check on health and update
			currentState := ui.StateHealthy
			a.app.QueueUpdateDraw(func() {
				a.ui.SetState(currentState)
			})
		}
	}
}
func (a *Application) setState(state ui.ProjectState) {
	select {
	case a.stateCh <- state:
	default:
		// Channel full, skip update
	}
}
