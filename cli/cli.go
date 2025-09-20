package cli

import (
	"fmt"
	"os"

	"github.com/piqoni/vogte/app"
)

func Run(application *app.Application, outputPath string) error {
	structure, err := application.Parse()
	if err != nil {
		return fmt.Errorf("could not parse project: %w", err)
	}

	if err := os.WriteFile(outputPath, []byte(structure), 0644); err != nil {
		return fmt.Errorf("error writing to file %s: %w", outputPath, err)
	}

	fmt.Printf("✅ Written result to %s\n", outputPath)
	return nil
}
