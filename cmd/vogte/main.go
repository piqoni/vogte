package main

import (
	"flag"
	"log"

	"github.com/piqoni/vogte/internal/app"
)

func main() {

	dirPtr := flag.String("dir", ".", "The directory to analyze")
	outputPtr := flag.String("output", "output.txt", "The output file")
	flag.Parse()

	application := app.New(*dirPtr, *outputPtr)

	if err := application.Run(); err != nil {
		log.Printf("Aplication error: %v", err)
	}

}
