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

// RunReview runs a review of uncommitted changes against the given base branch,
// prints the result, and optionally writes it to the output file if provided.
func RunReview(application *app.Application, outputPath, baseBranch, description string) error {
	result, err := application.ReviewDiffAgainstBase(baseBranch, description)
	if err != nil {
		return err
	}
	fmt.Println(result)
	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(result), 0644); err != nil {
			return fmt.Errorf("error writing review to file %s: %w", outputPath, err)
		}
	}
	return nil
}
