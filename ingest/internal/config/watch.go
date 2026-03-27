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
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/fsnotify/fsnotify"
)

// description: Returns a stable signature for one file's current contents or error state.
// input: filesystem path to hash.
// output: Returns a sha256 digest string or a stable error marker string.
func FileContentSignature(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return "error:" + err.Error()
	}
	sum := sha256.Sum256(content)
	return fmt.Sprintf("%x", sum)
}

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

// description: Watches a fixed set of files and triggers the callback only when one or more file content hashes change.
// input: context, file paths, debounce duration, and reload callback.
// output: Returns nil on clean shutdown or an error on watcher failure.
func WatchFilesByHash(ctx context.Context, paths []string, debounce time.Duration, onChange func() error) error {
	if onChange == nil {
		return fmt.Errorf("watch files by hash: onChange is required")
	}
	if debounce <= 0 {
		debounce = 200 * time.Millisecond
	}

	targetPaths := make([]string, 0, len(paths))
	seenPaths := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		cleaned := filepath.Clean(path)
		if cleaned == "." || cleaned == "" {
			continue
		}
		if _, seen := seenPaths[cleaned]; seen {
			continue
		}
		seenPaths[cleaned] = struct{}{}
		targetPaths = append(targetPaths, cleaned)
	}
	if len(targetPaths) == 0 {
		<-ctx.Done()
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("watch files by hash: create watcher: %w", err)
	}
	defer watcher.Close()

	dirSet := make(map[string]struct{}, len(targetPaths))
	for _, path := range targetPaths {
		dirSet[filepath.Dir(path)] = struct{}{}
	}
	watchDirs := make([]string, 0, len(dirSet))
	for dir := range dirSet {
		watchDirs = append(watchDirs, dir)
	}
	slices.Sort(watchDirs)
	for _, dir := range watchDirs {
		if err := watcher.Add(dir); err != nil {
			return fmt.Errorf("watch files by hash: watch %s: %w", dir, err)
		}
	}

	targetSet := make(map[string]struct{}, len(targetPaths))
	hashes := make(map[string]string, len(targetPaths))
	for _, path := range targetPaths {
		targetSet[path] = struct{}{}
		hashes[path] = FileContentSignature(path)
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
			changed := false
			for _, path := range targetPaths {
				nextHash := FileContentSignature(path)
				if hashes[path] == nextHash {
					continue
				}
				hashes[path] = nextHash
				changed = true
			}
			if changed {
				if err := onChange(); err != nil {
					return err
				}
			}
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if _, ok := targetSet[filepath.Clean(event.Name)]; !ok {
				continue
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove) {
				schedule()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return fmt.Errorf("watch files by hash: %w", err)
		}
	}
}
