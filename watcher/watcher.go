package watcher

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const maxWatchedFileSize = 1 << 20

// Config holds watch-mode configuration.
type Config struct {
	Dirs           []string
	DebounceMs     int
	TriggerKeyword string
	EndMarker      string
}

// FileHandler is called when a file change passes debounce and trigger checks.
type FileHandler interface {
	Handle(filePath string) error
}

// Watcher monitors directories and dispatches file events.
type Watcher struct {
	cfg        Config
	handler    FileHandler
	watcher    *fsnotify.Watcher
	mu         sync.Mutex
	timers     map[string]*time.Timer
	watchedDir map[string]struct{}
}

func New(cfg Config, handler FileHandler) (*Watcher, error) {
	if handler == nil {
		return nil, fmt.Errorf("watcher handler is required")
	}

	if cfg.DebounceMs <= 0 {
		cfg.DebounceMs = 500
	}
	if strings.TrimSpace(cfg.TriggerKeyword) == "" {
		cfg.TriggerKeyword = "@vogte"
	}
	if strings.TrimSpace(cfg.EndMarker) == "" {
		cfg.EndMarker = "/v/"
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	return &Watcher{
		cfg:        cfg,
		handler:    handler,
		watcher:    fsWatcher,
		timers:     make(map[string]*time.Timer),
		watchedDir: make(map[string]struct{}),
	}, nil
}

// Run blocks, watching all configured dirs recursively. Returns on ctx cancellation.
func (w *Watcher) Run(ctx context.Context) error {
	defer w.watcher.Close()
	defer w.stopTimers()

	for _, dir := range w.cfg.Dirs {
		info, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("watch dir %q: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("watch dir %q is not a directory", dir)
		}
		if err := w.addDir(dir); err != nil {
			return err
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-w.watcher.Events:
			if !ok {
				return nil
			}
			w.onEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return nil
			}
			return fmt.Errorf("watcher error: %w", err)
		}
	}
}

func (w *Watcher) stopTimers() {
	w.mu.Lock()
	defer w.mu.Unlock()

	for path, timer := range w.timers {
		timer.Stop()
		delete(w.timers, path)
	}
}

// addDir recursively adds a directory tree to the underlying fsnotify watcher.
func (w *Watcher) addDir(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}

		cleanPath := filepath.Clean(path)

		w.mu.Lock()
		_, exists := w.watchedDir[cleanPath]
		if !exists {
			w.watchedDir[cleanPath] = struct{}{}
		}
		w.mu.Unlock()

		if exists {
			return nil
		}

		if err := w.watcher.Add(cleanPath); err != nil {
			return fmt.Errorf("watch directory %q: %w", cleanPath, err)
		}
		return nil
	})
}

// onEvent is the internal fsnotify event callback. Applies debounce per file path.
func (w *Watcher) onEvent(event fsnotify.Event) {
	if event.Op&fsnotify.Create != 0 {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			if err := w.addDir(event.Name); err != nil {
				log.Printf("watcher: add new directory %s: %v", event.Name, err)
			}
			return
		}
	}

	if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
		return
	}

	info, err := os.Stat(event.Name)
	if err == nil && info.IsDir() {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if timer, ok := w.timers[event.Name]; ok {
		timer.Stop()
	}

	path := event.Name
	w.timers[path] = time.AfterFunc(time.Duration(w.cfg.DebounceMs)*time.Millisecond, func() {
		w.mu.Lock()
		delete(w.timers, path)
		w.mu.Unlock()

		shouldProcess, err := w.shouldProcess(path)
		if err != nil {
			if os.IsNotExist(err) {
				return
			}
			log.Printf("watcher: inspect %s: %v", path, err)
			return
		}
		if !shouldProcess {
			return
		}
		if err := w.handler.Handle(path); err != nil {
			if os.IsNotExist(err) {
				log.Printf("watcher: file disappeared during processing: %s", path)
				return
			}
			log.Printf("watcher: handle %s: %v", path, err)
		}
	})
}

// shouldProcess returns true when the file contains @vogte without a following /v/.
func (w *Watcher) shouldProcess(filePath string) (bool, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return false, err
	}
	if info.Size() > maxWatchedFileSize {
		log.Printf("watcher: skipping large file %s (%d bytes)", filePath, info.Size())
		return false, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	header := make([]byte, 512)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		return false, err
	}
	if strings.IndexByte(string(header[:n]), 0) >= 0 {
		return false, nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}

	blocks := findUnansweredBlocksInContent(string(content), w.cfg.TriggerKeyword, w.cfg.EndMarker)
	return len(blocks) > 0, nil
}
