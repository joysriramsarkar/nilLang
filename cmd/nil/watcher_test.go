package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// U0.5: the poll-based file watcher (cmd/nil/watcher.go) is the hot-reload
// trigger for `nil dev`. Polling (not fsnotify) is intentional: identical on
// every OS, no cgo, reliable on network/bind mounts.
func TestWatcherDetectsChange(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "main.nil")
	if err := os.WriteFile(f, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	var changed [][]string
	w, err := NewSourceWatcher(WatcherOptions{
		Paths:    []string{f},
		OnChange: func(p []string) { changed = append(changed, p) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Poll(); len(got) != 0 {
		t.Fatalf("fresh snapshot must be quiet, got %v", got)
	}
	if err := os.WriteFile(f, []byte("v2"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := w.Poll(); len(got) != 1 {
		t.Fatalf("edit must report one change, got %v", got)
	}
	if got := w.Poll(); len(got) != 0 {
		t.Fatalf("second poll must be quiet, got %v", got)
	}
}

func TestWatcherStartCallsOnChange(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.nil")
	os.WriteFile(f, []byte("1"), 0644)
	done := make(chan []string, 1)
	w, err := NewSourceWatcher(WatcherOptions{
		Interval: 10 * time.Millisecond,
		Paths:    []string{f},
		OnChange: func(p []string) { done <- p },
	})
	if err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	defer close(stop)
	go w.Start(stop)
	os.WriteFile(f, []byte("2"), 0644)
	select {
	case p := <-done:
		if len(p) != 1 {
			t.Fatalf("want 1 changed path, got %v", p)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watcher did not fire")
	}
}

func TestDevWatcherWatchesProject(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(src, "main.nil")
	os.WriteFile(entry, []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "lib.nil"), []byte("y"), 0644)
	store := newDeclarativeUISessionStore("x", src)
	w, err := newDevWatcher(entry, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Files()) < 1 {
		t.Fatalf("dev watcher must watch files, got %v", w.Files())
	}
}
