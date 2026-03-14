package config

/*
Name: ingest/internal/config/watch.go
Description: Watches the source catalog file and triggers reload callbacks after a debounce delay.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-13
Revision History:
- 2026-03-07, Barrett Brown: Added standardized prologue documentation block.
- 2026-03-13, Barrett Brown: Added clearer watcher flow comments.
Preconditions:
- A valid source catalog path and reload callback are provided.
Acceptable Input Values/Types:
- Existing file paths and positive debounce durations.
Unacceptable Input Values/Types:
- Empty paths or nil callbacks.
Postconditions:
- Watches the source catalog directory and calls onChange after matching file updates.
Return Values/Types:
- WatchSourceCatalog: error
Error/Exception Conditions:
- Watcher creation failures, add failures, and callback errors.
Side Effects:
- Starts an fsnotify watcher and listens until context cancellation.
Invariants:
- Only the target source catalog path triggers the callback.
Known Faults:
- Rename and write storms still collapse into one debounced callback only.
*/

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// description: Watches one source catalog file and debounces reload callbacks.
// input: context, target file path, debounce duration, and reload callback.
// output: Returns nil on clean shutdown or an error on watcher failure.
func WatchSourceCatalog(ctx context.Context, path string, debounce time.Duration, onChange func() error) error {
	if path == "" {
		return fmt.Errorf("watch source catalog: path is required")
	}
	if onChange == nil {
		return fmt.Errorf("watch source catalog: onChange is required")
	}
	if debounce <= 0 {
		debounce = 200 * time.Millisecond
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("watch source catalog: create watcher: %w", err)
	}
	defer watcher.Close()

	dir := filepath.Dir(path)
	target := filepath.Clean(path)
	if err := watcher.Add(dir); err != nil {
		return fmt.Errorf("watch source catalog: watch %s: %w", dir, err)
	}

	var (
		timer   *time.Timer
		timerCh <-chan time.Time
	)

	schedule := func() {
		if timer == nil {
			timer = time.NewTimer(debounce)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(debounce)
		}
		timerCh = timer.C
	}

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return nil
		case <-timerCh:
			timerCh = nil
			if err := onChange(); err != nil {
				return err
			}
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if filepath.Clean(event.Name) != target {
				continue
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
				schedule()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return fmt.Errorf("watch source catalog: %w", err)
		}
	}
}
