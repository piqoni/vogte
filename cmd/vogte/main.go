package main

import (
	"flag"
	"log"
	"os"

	"github.com/piqoni/vogte/internal/app"
	"github.com/piqoni/vogte/internal/cli"
	"github.com/piqoni/vogte/internal/config"
)

func main() {
	pwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}

	agentPtr := flag.Bool("agent", false, "Start in AGENT mode")
	configPtr := flag.String("config", "", "Path to config file")
	dirPtr := flag.String("dir", pwd, "The directory to analyze")
	outputPtr := flag.String("output", "vogte-output.txt", "The output file")
	modelPtr := flag.String("model", "", "LLM model name (overrides config)")
	flag.Parse()

	cfg := config.Load(*configPtr)
	if cfg == nil {
		log.Fatal("Failed to load configuration")
	}

	if *modelPtr != "" {
		cfg.LLM.Model = *modelPtr
	}

	application := app.New(cfg, *dirPtr, *outputPtr)

	if *agentPtr {
		application.SetMode("AGENT")
	} else {
		application.SetMode("ASK")
	}

	outputFlag := flag.Lookup("output")
	wasOutputPassed := outputFlag.Value.String() != outputFlag.DefValue

	// CLI mode
	if wasOutputPassed {
		if err := cli.Run(application, *outputPtr); err != nil {
			log.Fatalf("CLI error: %v", err)
		}
		return
	}

	// UI mode
	if err := application.Run(); err != nil {
		log.Printf("Application error: %v", err)
	}
}
