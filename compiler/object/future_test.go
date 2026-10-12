package object

import (
	"sync"
	"testing"
)

// A non-blocking Resolved must not steal the completion: Await afterwards
// still answers, and so does a second Resolved. The old channel-based design
// consumed the buffered result on the first Resolved, after which Await
// blocked forever — a hang the collector's mark phase could trigger by
// tracing a completed-but-unawaited task.
func TestResolvedDoesNotConsumeTheCompletion(t *testing.T) {
	f := NewFuture()
	f.Complete(&Integer{Value: 7}, nil)

	v, done := f.Resolved()
	if !done {
		t.Fatal("a completed future reports it is not done")
	}
	if v == nil || v.Inspect() != "7" {
		t.Fatalf("resolved value = %v, wanted 7", v)
	}

	v, done = f.Resolved()
	if !done || v == nil || v.Inspect() != "7" {
		t.Fatalf("second Resolved = (%v, %v), wanted (7, true)", v, done)
	}

	got, err := f.Await()
	if err != nil || got == nil || got.Inspect() != "7" {
		t.Fatalf("Await after Resolved = (%v, %v), wanted (7, nil)", got, err)
	}
}

func TestResolvedOnAPendingFutureIsFalse(t *testing.T) {
	f := NewFuture()
	if v, done := f.Resolved(); done || v != nil {
		t.Fatalf("pending Resolved = (%v, %v), wanted (nil, false)", v, done)
	}
	f.Complete(TRUE, nil)
	if _, err := f.Await(); err != nil {
		t.Fatalf("Await = %v, wanted nil", err)
	}
}

func TestOnlyTheFirstCompletionCounts(t *testing.T) {
	f := NewFuture()
	f.Complete(&Integer{Value: 1}, nil)
	f.Complete(&Integer{Value: 2}, nil)
	got, _ := f.Await()
	if got == nil || got.Inspect() != "1" {
		t.Fatalf("Await = %v, wanted 1", got)
	}
}

func TestConcurrentAwaitersAllAnswer(t *testing.T) {
	f := NewFuture()
	const waiters = 8
	got := make([]Object, waiters)
	var wg sync.WaitGroup
	for i := 0; i < waiters; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, err := f.Await()
			if err != nil {
				t.Errorf("waiter %d: %v", i, err)
				return
			}
			got[i] = v
		}(i)
	}
	f.Complete(&Integer{Value: 9}, nil)
	wg.Wait()
	for i, v := range got {
		if v == nil || v.Inspect() != "9" {
			t.Fatalf("waiter %d = %v, wanted 9", i, v)
		}
	}
}
