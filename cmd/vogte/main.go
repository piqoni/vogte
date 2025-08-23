package main

import (
	"flag"
	"log"

	"github.com/piqoni/vogte/internal/app"
	"github.com/piqoni/vogte/internal/cli"
)

func main() {

	dirPtr := flag.String("dir", ".", "The directory to analyze")
	outputPtr := flag.String("output", "vogte-output.txt", "The output file")
	flag.Parse()

	application := app.New(*dirPtr, *outputPtr)

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
