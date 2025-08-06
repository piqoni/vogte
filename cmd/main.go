package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/piqoni/vogte/internal/parser"
)

func main() {

	dirPtr := flag.String("dir", ".", "The directory to analyze")
	outputPtr := flag.String("output", "output.txt", "The output file")
	flag.Parse()

	dir := *dirPtr

	outputFile := *outputPtr
	// modulePath := ""

	parser := parser.New()

	result, _ := parser.ParseProject(dir)
	if err := os.WriteFile(outputFile, []byte(result), 0644); err != nil {
		fmt.Printf("Error writing to file %s: %v\n", outputFile, err)
		os.Exit(1)
	}

	fmt.Printf("Output written to %s\n", outputFile)

}
