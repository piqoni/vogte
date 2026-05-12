package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/piqoni/vogte/app"
	"github.com/piqoni/vogte/watcher"
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

func RunWatch(ctx context.Context, application *app.Application) error {
	cfg := application.LLMClient().Config()
	if len(cfg.Watch.Dirs) == 0 {
		return fmt.Errorf("watch mode requires at least one directory in config.watch.dirs")
	}

	dirs := make([]string, 0, len(cfg.Watch.Dirs))
	for _, dir := range cfg.Watch.Dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("resolve watch dir %q: %w", dir, err)
		}
		dirs = append(dirs, absDir)
	}

	debounceMs := cfg.Watch.DebounceMs
	if debounceMs <= 0 {
		debounceMs = 500
	}

	watchCfg := watcher.Config{
		Dirs:           dirs,
		DebounceMs:     debounceMs,
		TriggerKeyword: "@vogte",
		EndMarker:      "/v/",
	}

	responder := watcher.NewResponder(application.LLMClient(), application.Parser(), watchCfg)
	w, err := watcher.New(watchCfg, responder)
	if err != nil {
		return err
	}

	fmt.Printf("watch mode started\n")
	fmt.Printf("watching: %v\n", dirs)
	fmt.Printf("trigger: %s\n", watchCfg.TriggerKeyword)
	fmt.Printf("end marker: %s\n", watchCfg.EndMarker)

	return w.Run(ctx)
}
