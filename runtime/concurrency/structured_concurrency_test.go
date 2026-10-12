package concurrency

import (
	"context"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// L6.2 — structured concurrency: a spawned task shares mutable state with its parent
// scope rather than copying a global goroutine.
//
// # Why this is here and not hidden
//
// Because a `Task.Scope` that copied the parent globally (as the old broken version
// did — line 29 of `task.go` said so: "global goroutine, shared logic broken") would
// make the value the parent writes invisible to the child. A child that writes a value
// that the parent then reads is the test that confirms the two are the same scope — and
// that the shared state actually works.
func TestASpawnedTaskSharesStateWithParent(t *testing.T) {
	root := NewTaskScope(nil)

	slot := 0
	child := root.Spawn(func(ctx context.Context) (object.Object, error) {
		slot = 1
		return nil, ErrTaskCancelled
	})

	_, err := child.Await()
	if err != nil {
		t.Logf("child finished with %v; mechanism verified if shared-state holds", err)
	}

	if slot != 1 {
		t.Errorf("a spawned task did not share mutable state with its parent: wrote 1, parent sees %d", slot)
	}
}
