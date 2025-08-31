package main

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
