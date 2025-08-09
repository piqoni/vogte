package main

import (
	"flag"
	"log"
	"os"

	"github.com/piqoni/vogte/internal/app"
	"github.com/piqoni/vogte/internal/parser"
)

func main() {

	dirPtr := flag.String("dir", ".", "The directory to analyze")
	outputPtr := flag.String("output", "output.txt", "The output file")
	flag.Parse()

	dir := *dirPtr
	outputFile := *outputPtr

	application := app.New(*dirPtr)

	if err := application.Run(); err != nil {
		log.Printf("Aplication error: %w", err)
	}

	p := parser.New()

	result, err := p.ParseProject(dir)
	if err != nil {
		log.Fatalf("Could not parse the project: %v", err)
	}
	if err := os.WriteFile(outputFile, []byte(result), 0644); err != nil {
		log.Fatalf("Error writing to file %s: %v\n", outputFile, err)
	}

	log.Printf("Output written to %s\n", outputFile)

}
