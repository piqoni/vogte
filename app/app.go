package app

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/piqoni/vogte/config"
	"github.com/piqoni/vogte/llm"
	"github.com/piqoni/vogte/parser"
	"github.com/piqoni/vogte/patcher"
	"github.com/piqoni/vogte/ui"
	"github.com/rivo/tview"
)

type Application struct {
	baseDir    string
	outputFile string
	app        *tview.Application
	ui         *ui.UI
	parser     *parser.Parser
	patcher    *patcher.Patcher
	llm        *llm.Client
	Mode       string

	stateMu   sync.RWMutex
	state     ui.ProjectState
	stateCh   chan ui.ProjectState
	lastError error
}

func New(cfg *config.Config, baseDir string, outputFile string, mode string) *Application {
	app := &Application{
		baseDir:    baseDir,
		app:        tview.NewApplication(),
		parser:     parser.New(),
		patcher:    patcher.New(baseDir),
		llm:        llm.New(cfg),
		outputFile: outputFile,
		Mode:       mode,
		state:      ui.StateUnknown,
		stateCh:    make(chan ui.ProjectState, 1),
	}
	app.ui = ui.New(app.app, app.messageHandler)
	app.ui.SetModeChangeCallback(app.modeChangeHandler)
	app.ui.SetMode(app.Mode)
	app.ui.SetBaseDir(baseDir)
	app.ui.SetModelName(cfg.LLM.Model)
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
		return
	}

	a.ui.StartLoading()

	go func() {
		defer a.ui.StopLoading()

		structure, err := a.parser.ParseProject(a.baseDir)
		if err != nil {
			a.setState(ui.StateError)
			a.setError(fmt.Errorf("Could not parse the project: %w ", err))
			a.postSystemMessage("ERROR: Could not parse the project: " + err.Error())
			return
		}

		// Send to LLM
		response, err := a.llm.SendMessage(message, structure, a.Mode)
		if err != nil {
			a.setState(ui.StateError)
			a.setError(fmt.Errorf("LLM error: %w", err))
			a.postSystemMessage("ERROR: " + err.Error())
			return
		}
		// response := manualPatch

		a.postSystemMessage("Mode: " + a.Mode)
		a.postSystemMessage(response)

		// Append to .vogte/chatbot.log
		logDir := filepath.Join(".", ".vogte")
		if err := os.MkdirAll(logDir, 0755); err == nil {
			logPath := filepath.Join(logDir, "chatbot.log")
			if f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				_, _ = f.WriteString(message + "\n" + response + "\n\n\n\n\n")
				f.Close()
			}
		}
		if a.Mode == "AGENT" {
			if err := a.patcher.ParseAndApply(response); err != nil {
				a.setState(ui.StateError)
				a.setError(fmt.Errorf("patch apply error: %w", err))
				a.postSystemMessage("ERROR: Patch apply failed: " + err.Error())
				return
			}
			a.runSanityCheck()
		}
	}()
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

func (a *Application) runSanityCheck() {
	a.postSystemMessage("Running sanity check: go vet ./...")
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = a.baseDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	errOut := strings.TrimSpace(stderr.String())

	if err != nil {
		a.setState(ui.StateError)
		combined := strings.TrimSpace(strings.Join([]string{out, errOut}, "\n"))
		if combined == "" {
			combined = err.Error()
		}
		a.setError(fmt.Errorf("go vet failed: %s", combined))
		a.postSystemMessage("go vet failed:\n" + combined)
		return
	}

	a.setState(ui.StateHealthy)
	if out != "" || errOut != "" {
		a.postSystemMessage("go vet output:\n" + strings.TrimSpace(strings.Join([]string{out, errOut}, "\n")))
	} else {
		a.postSystemMessage("go vet passed with no issues.")
	}
}

// ReviewDiffAgainstBase collects the diff of uncommitted changes against the given base branch
// (default "main", with fallback to "master") and asks the LLM to review it.
func (a *Application) ReviewDiffAgainstBase(baseBranch, description string) (string, error) {
	diff, err := a.getDiffAgainstBase(baseBranch)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(diff) == "" {
		return fmt.Sprintf("No uncommitted changes detected against %s.", baseBranch), nil
	}
	return a.llm.ReviewDiff(diff, description)
}

// getDiffAgainstBase returns the git diff against the given baseBranch
func (a *Application) getDiffAgainstBase(baseBranch string) (string, error) {
	runDiff := func(branch string) (string, string, error) {
		cmd := exec.Command("git", "diff", branch, "--")
		cmd.Dir = a.baseDir
		var out, errOut bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errOut
		err := cmd.Run()
		return out.String(), errOut.String(), err
	}

	out, errOut, err := runDiff(baseBranch)
	if err != nil {
		return "", fmt.Errorf("git diff error: %s", strings.TrimSpace(errOut))
	}
	return out, nil
}
