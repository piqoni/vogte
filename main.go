package main

import (
	"flag"
	"log"
	"os"

	"github.com/piqoni/vogte/app"
	"github.com/piqoni/vogte/cli"
	"github.com/piqoni/vogte/config"
)

func main() {
	pwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}

	agentPtr := flag.Bool("agent", false, "Start in AGENT mode")
	configPtr := flag.String("config", "", "Path to config file. Example: vogte -config config.json ")
	dirPtr := flag.String("dir", pwd, "The directory to analyze")
	outputPtr := flag.String("output", "vogte-output.txt", "The output file")
	modelPtr := flag.String("model", "", "LLM model name (overrides config)")
	flag.Parse()

	cfg := config.Load(*configPtr)
	if cfg == nil {
		log.Fatal("Failed to load configuration")
	}

	if *modelPtr != "" {
		cfg.SetModel(*modelPtr)
	}

	initialMode := "ASK"
	if *agentPtr {
		initialMode = "AGENT"
	}
	application := app.New(cfg, *dirPtr, *outputPtr, initialMode)

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
