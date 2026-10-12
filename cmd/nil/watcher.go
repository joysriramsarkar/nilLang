package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SourceWatcher polls a set of files and reports when their contents change.
//
// It polls rather than using a platform notification API, for two reasons that
// both matter to a developer tool: fsnotify needs cgo on some platforms, which
// would make `nil dev` unbuildable where CGO is disabled, and polling behaves
// identically on every OS, including network drives and container bind mounts
// where native notifications are unreliable or absent.
//
// The cost is one stat per file per interval, which is nothing at development
// scale: a project has tens of source files, not thousands.
type SourceWatcher struct {
	// interval is how often each file is checked.
	interval time.Duration

	// files are the paths being watched.
	files []string

	// digests maps a path to the content digest observed on the last check.
	digests map[string]string

	// onChange is called with the changed paths whenever a check finds one.
	onChange func(paths []string)

	mu sync.Mutex
}

// WatcherOptions configures a SourceWatcher.
type WatcherOptions struct {
	// Interval is the poll period. Zero or less uses 300ms, which is responsive
	// enough to feel immediate without spinning.
	Interval time.Duration

	// Paths are the files to watch. Directories are expanded recursively for
	// Nilang source and common resource types, so passing a project directory
	// works.
	Paths []string

	// OnChange receives the paths whose contents changed since the last check.
	OnChange func(paths []string)
}

// watchExtensions are the file types a project cares about reloading.
var watchExtensions = map[string]bool{
	".nil":   true,
	".json":  true,
	".css":   true,
	".js":    true,
	".html":  true,
	".svg":   true,
	".png":   true,
	".jpg":   true,
	".jpeg":  true,
	".webp":  true,
	".woff":  true,
	".woff2": true,
}

// NewSourceWatcher builds a watcher and takes an initial snapshot, so that the
// first poll after startup does not report every file as changed.
func NewSourceWatcher(opts WatcherOptions) (*SourceWatcher, error) {
	if opts.Interval <= 0 {
		opts.Interval = 300 * time.Millisecond
	}

	files, err := expandWatchPaths(opts.Paths)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no watchable files found in %v", opts.Paths)
	}

	watcher := &SourceWatcher{
		interval: opts.Interval,
		files:    files,
		digests:  make(map[string]string, len(files)),
		onChange: opts.OnChange,
	}
	watcher.snapshot()

	return watcher, nil
}

// expandWatchPaths resolves the configured paths into a concrete file list.
//
// A directory is walked recursively; skip directories are pruned so that
// node_modules or a build output directory does not turn a dev server into a
// disk scanner.
func expandWatchPaths(paths []string) ([]string, error) {
	var files []string
	seen := make(map[string]bool)

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			// A path that does not exist yet is not an error: a developer may
			// point at a file they are about to create.
			continue
		}

		if !info.IsDir() {
			addWatchFile(&files, seen, path)
			continue
		}

		err = filepath.Walk(path, func(p string, entry os.FileInfo, walkErr error) error {
			if walkErr != nil {
				// An unreadable entry should not abort the whole walk.
				return nil
			}
			if entry.IsDir() {
				if shouldSkipWatchDir(entry.Name()) && p != path {
					return filepath.SkipDir
				}
				return nil
			}
			addWatchFile(&files, seen, p)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return files, nil
}

func addWatchFile(files *[]string, seen map[string]bool, path string) {
	if seen[path] || !watchExtensions[strings.ToLower(filepath.Ext(path))] {
		return
	}
	seen[path] = true
	*files = append(*files, path)
}

func shouldSkipWatchDir(name string) bool {
	switch name {
	case ".git", "node_modules", "build", "dist", "bin", ".vscode", ".idea", "vendor":
		return true
	}
	return false
}

// Files returns the watched file list, which is useful in diagnostics output.
func (w *SourceWatcher) Files() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.files...)
}

// digestFile returns a content digest, or "" when the file is unreadable.
//
// Hashing content rather than comparing timestamps means a save that preserves
// the modification time still registers, which some editors and build tools do.
func digestFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// snapshot records the current digest of every watched file.
func (w *SourceWatcher) snapshot() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, path := range w.files {
		w.digests[path] = digestFile(path)
	}
}

// Poll checks every file once and returns the paths that changed.
//
// It is exported separately from Start so a test can drive it deterministically
// instead of sleeping and hoping.
func (w *SourceWatcher) Poll() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	var changed []string
	for _, path := range w.files {
		current := digestFile(path)
		previous, known := w.digests[path]

		if !known {
			// A file that appeared since startup is new, not changed.
			w.digests[path] = current
			changed = append(changed, path)
			continue
		}
		if current != previous {
			w.digests[path] = current
			changed = append(changed, path)
		}
	}
	return changed
}

// Start polls until stop is closed, invoking OnChange for each batch of changed
// paths. It blocks, so it is normally run in its own goroutine.
func (w *SourceWatcher) Start(stop <-chan struct{}) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if changed := w.Poll(); len(changed) > 0 && w.onChange != nil {
				w.onChange(changed)
			}
		}
	}
}

// RelativePaths renders changed paths relative to a base directory for display.
func RelativePaths(paths []string, base string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if rel, err := filepath.Rel(base, path); err == nil {
			out = append(out, rel)
			continue
		}
		out = append(out, path)
	}
	return out
}

// newDevWatcher builds the hot-reload watcher for `nil dev`.
//
// It watches the entry file plus its project root, so an edit to an imported
// module or to an asset triggers a rebuild rather than being missed. On change
// it re-reads the entry file, re-evaluates the program, and swaps the app into
// every existing browser session.
//
// A rebuild failure is reported and the previous app is kept. Half-applying a
// broken edit would leave a developer staring at an empty page with no way to
// tell that the real problem is in the file they just saved.
func newDevWatcher(entryFile string, sessions *declarativeUISessionStore) (*SourceWatcher, error) {
	entryDir := filepath.Dir(entryFile)

	paths := []string{entryFile}
	// Watch the project root when the entry lives in src/, which is the layout
	// `nil init` produces. Watching the parent of src/ catches edits to lib/
	// and other sibling directories without pulling in the whole tree.
	if filepath.Base(entryDir) == "src" {
		paths = append(paths, filepath.Dir(entryDir))
	}

	return NewSourceWatcher(WatcherOptions{
		Paths: paths,
		OnChange: func(changed []string) {
			reloadDeclarativeUI(entryFile, entryDir, sessions, changed)
		},
	})
}

// reloadDeclarativeUI rebuilds the app and swaps it into every session.
func reloadDeclarativeUI(entryFile, entryDir string, sessions *declarativeUISessionStore, changed []string) {
	source, err := os.ReadFile(entryFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  reload skipped, cannot read %s: %v\n", entryFile, err)
		return
	}

	app, err := loadDeclarativeUI(string(source), entryDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  reload failed, keeping the previous build: %v\n", err)
		return
	}

	swapped := sessions.replaceAllApps(app)
	sessions.SetSource(string(source))
	fmt.Printf("🔄 reloaded: %s (%d session(s) updated)\n",
		strings.Join(RelativePaths(changed, entryDir), ", "), swapped)
}
