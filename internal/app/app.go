package app

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/piqoni/vogte/internal/config"
	"github.com/piqoni/vogte/internal/llm"
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
	llm        *llm.Client
	Mode       string

	stateMu   sync.RWMutex
	state     ui.ProjectState
	stateCh   chan ui.ProjectState
	lastError error
}

func New(cfg *config.Config, baseDir string, outputFile string) *Application {
	app := &Application{
		baseDir:    baseDir,
		app:        tview.NewApplication(),
		parser:     parser.New(),
		llm:        llm.New(cfg),
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
	// switch a.Mode {
	// case "ASK":
	// a.handleAskMode(message)
	// case "AGENT":
	// a.handleAgentMode(message)
	// }

	var wg sync.WaitGroup

	wg.Go(func() {
		structure, err := a.parser.ParseProject(a.baseDir)
		// a.setState(ui.StateError) // remove me, just a test
		if err != nil {
			a.setState(ui.StateError)
			a.setError(fmt.Errorf("Could not parse the project: %w ", err))
		}

		// Send to LLM
		response, err := a.llm.SendMessage(message, structure, a.Mode)
		if err != nil {
			a.setState(ui.StateError)
			a.setError(fmt.Errorf("LLM error: %w", err))
			a.postSystemMessage(err.Error())
		}
		a.postSystemMessage(a.Mode)
		a.postSystemMessage(response)
	})

	// wg.Wait()
	// if a.lastError != nil {
	// a.postSystemMessage("ERROR: " + a.lastError.Error())
	// }

}

func (app *Application) modeChangeHandler(newMode string) {
	app.Mode = newMode

	// app.postSystemMessage(fmt.Sprintf("Mode changed to: %s", newMode))
}

func (app *Application) postSystemMessage(message string) {
	// Schedule the UI update to run on the main UI thread.
	app.app.QueueUpdateDraw(func() {
		systemMessage := fmt.Sprintf("\n System: %s", message)
		// currentText := app.ui.GetChatText()
		app.ui.AppendChatText(systemMessage)
	})
}

func (app *Application) SetMode(mode string) {
	if app.Mode != mode {
		app.Mode = mode
		app.ui.SetMode(mode)
		// app.postSystemMessage(fmt.Sprintf("Mode changed to: %s", mode))
	}
}

func (app *Application) GetMode() string {
	return app.Mode
}

// func (app *Application) handleAskMode(message string) { // todo
// 	response := fmt.Sprintf("\n Assistant (ASK): %s", message)
// 	currentText := app.ui.GetChatText()
// 	app.ui.SetChatText(currentText + response)
// }

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
			a.stateMu.Lock()
			a.state = newState
			a.stateMu.Unlock()

			a.app.QueueUpdateDraw(func() {
				a.ui.SetState(newState)
			})

		case <-ticker.C:
			// TODO periodic check on health and update
			// currentState := ui.StateHealthy
			// a.app.QueueUpdateDraw(func() {
			// a.ui.SetState(currentState)
			// })
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

func (a *Application) setError(err error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.lastError = err
}

func (a *Application) getLastError() error {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.lastError
}
