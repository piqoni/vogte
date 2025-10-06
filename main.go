package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/piqoni/vogte/app"
	"github.com/piqoni/vogte/cli"
	"github.com/piqoni/vogte/config"
)

func main() {
	Pwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}

	reviewPtr := flag.Bool("review", false, "Ask the LLM to review changes against base branch (default: main). Optionally provide a message after -review to be used as change description.")
	agentPtr := flag.Bool("agent", false, "Start in AGENT mode")
	configPtr := flag.String("config", "", "Path to config file. Example: vogte -config config.json ")
	dirPtr := flag.String("dir", pwd, "The directory to analyze")
	contextPtr := flag.Bool("generate-context", false, "Generate context file (vogte-context.txt)")
	modelPtr := flag.String("m0del", "", "LLM model name (overrides config)")
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

	contextFile := "vogte-context.txt"
	application := app.New(cfg, *dirPtr, contextFile, initialMode)

	// Review mode
	if *reviewPtr {
		desc := strings.TrimSpace(strings.Join(flag.Args(), " "))
		if err := cli.RunReview(application, contextFile, "main", desc); err != nil {
			log.Fatalf("Review error: %v", err)
		}
		return
	}

	// CLI mode
	if *contextPtr {
		if err := cli.Run(application, contextFile); err != nil {
			log.Fatalf("CLI error: %v", err)
		}
		return
	}

	// UI mode
	if err := application.Run(); err != nil {
		log.Printf("Application error: %v", err)
	}
}
