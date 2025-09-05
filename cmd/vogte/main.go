package main

// Vogte is a developer tool that analyzes a target directory to generate structured
// insights about the codebase. It can operate in two complementary modes:
// - AGENT: autonomous analysis that explores the repository and produces a report
// - ASK: interactive Q&A mode for targeted questions about the codebase
//
// Interfaces:
// - CLI: runs when an explicit -output flag is provided; results are written to the file
// - UI: runs when -output is not provided; results are presented in the application UI
//
// Inputs:
// - -dir points to the root directory to analyze (default ".")
// - -config specifies the path to a configuration file used by the analysis engine
//
// Examples:
// - vogte -dir . -config config.yaml
// - vogte -ask
// - vogte -dir ./repo -output report.txt
//
// Internals:
// - internal/app orchestrates runtime behavior and modes
// - internal/cli provides command-line workflow
// - internal/config loads and validates configuration

import (
	"flag"
	"log"

	"github.com/piqoni/vogte/internal/app"
	"github.com/piqoni/vogte/internal/cli"
	"github.com/piqoni/vogte/internal/config"
)

func main() {
	askPtr := flag.Bool("ask", false, "Start in ASK mode")
	configPtr := flag.String("config", "", "Path to config file")
	dirPtr := flag.String("dir", ".", "The directory to analyze")
	outputPtr := flag.String("output", "vogte-output.txt", "The output file")
	flag.Parse()

	cfg := config.Load(*configPtr)
	if cfg == nil {
		log.Fatal("Failed to load configuration")
	}
	application := app.New(cfg, *dirPtr, *outputPtr)

	if *askPtr {
		application.SetMode("ASK")
	} else {
		application.SetMode("AGENT")
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
