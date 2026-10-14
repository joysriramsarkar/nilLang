package concurrency

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

var (
	ErrTaskCancelled = errors.New("task cancelled")
	ErrTaskTimeout   = errors.New("task timeout exceeded")
)

type TaskStatus string

const (
	StatusPending   TaskStatus = "PENDING"
	StatusRunning   TaskStatus = "RUNNING"
	StatusFulfilled TaskStatus = "FULFILLED"
	StatusFaulted   TaskStatus = "FAULTED"
	StatusCancelled TaskStatus = "CANCELLED"
)

// Task represents a managed concurrent unit of execution in a structured tree
type Task struct {
	ID       string
	ctx      context.Context
	cancel   context.CancelFunc
	parent   *Task
	children []*Task
	mu       sync.Mutex
	status   TaskStatus
	future   *object.Future
}

// NewTaskScope creates a root or child task scope
func NewTaskScope(parent *Task) *Task {
	var ctx context.Context
	var cancel context.CancelFunc

	if parent != nil {
		ctx, cancel = context.WithCancel(parent.ctx)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}

	t := &Task{
		ctx:      ctx,
		cancel:   cancel,
		parent:   parent,
		children: []*Task{},
		status:   StatusPending,
		future:   object.NewFuture(),
	}

	if parent != nil {
		parent.mu.Lock()
		parent.children = append(parent.children, t)
		parent.mu.Unlock()
	}

	return t
}

// Spawn executes a computation in a goroutine managed by this task scope
func (t *Task) Spawn(action func(ctx context.Context) (object.Object, error)) *Task {
	child := NewTaskScope(t)
	child.mu.Lock()
	child.status = StatusRunning
	child.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				child.Fail(fmt.Errorf("task panicked: %v", r))
			}
		}()

		// Check if already cancelled
		select {
		case <-child.ctx.Done():
			child.Cancel()
			return
		default:
		}

		result, err := action(child.ctx)
		if err != nil {
			// An action that reports cancellation, or that finished after
			// its scope was cancelled, is cancelled rather than faulted:
			// the distinction is whether the work failed or the tree did.
			if errors.Is(err, ErrTaskCancelled) || child.ctx.Err() != nil {
				child.Cancel()
			} else {
				child.Fail(err)
			}
			return
		}

		if child.ctx.Err() != nil {
			child.Cancel()
			return
		}

		child.mu.Lock()
		child.status = StatusFulfilled
		child.mu.Unlock()
		child.future.Complete(result, nil)
	}()

	return child
}

// Cancel cancels this task and cascades the cancellation to all descendants.
//
// CollectFutures returns the futures of this scope and all descendants.
//
// The VM's root set includes them so an in-flight task's future — and through
// it, a completed result not yet awaited — survives collection (MEMORY_MODEL.md
// §2.1 item 4). Children are snapshotted under lock and walked without holding
// it, the same discipline as Cancel: a collection must never deadlock against
// a task completing concurrently.
func (t *Task) CollectFutures() []*object.Future {
	t.mu.Lock()
	out := []*object.Future{t.future}
	children := append([]*Task(nil), t.children...)
	t.mu.Unlock()

	for _, child := range children {
		out = append(out, child.CollectFutures()...)
	}
	return out
}

// Cancellation flows strictly downward: a parent cancels its children, never
// the reverse. An earlier version propagated a child's Cancel to its parent,
// which re-cancelled the child, which re-cancelled the parent — unbounded
// mutual recursion that exhausted the goroutine stack and crashed the host.
// A faulted child cancels its siblings through Fail, not through Cancel.
func (t *Task) Cancel() {
	t.mu.Lock()
	if t.status == StatusCancelled {
		t.mu.Unlock()
		return
	}
	t.status = StatusCancelled
	children := append([]*Task(nil), t.children...)
	t.mu.Unlock()

	t.cancel()

	for _, child := range children {
		child.Cancel()
	}

	t.future.Complete(nil, ErrTaskCancelled)
}

// Fail marks the task faulted, cancels its own subtree and its siblings, and
// completes the future with err.
//
// This is the sibling-cancel-on-fault half of the structured-concurrency
// contract (CONCURRENCY.md §3.1): a fault in one branch must not leave the
// others running. Unlike Cancel it does not mark the parent itself — the
// parent observes the failure through its children's terminal states.
func (t *Task) Fail(err error) {
	if err == nil {
		err = fmt.Errorf("task failed")
	}
	t.mu.Lock()
	switch t.status {
	case StatusFaulted, StatusCancelled, StatusFulfilled:
		t.mu.Unlock()
		return
	}
	t.status = StatusFaulted
	children := append([]*Task(nil), t.children...)
	parent := t.parent
	t.mu.Unlock()

	t.cancel()

	for _, child := range children {
		child.Cancel()
	}
	if parent != nil {
		parent.cancelSiblings(t)
	}

	t.future.Complete(nil, err)
}

// cancelSiblings cancels every child except the faulted one, without marking
// the scope itself. It is what keeps one branch's failure from leaving the
// other branches running while leaving the parent's own state to its owner.
func (t *Task) cancelSiblings(except *Task) {
	t.mu.Lock()
	siblings := append([]*Task(nil), t.children...)
	t.mu.Unlock()
	for _, sibling := range siblings {
		if sibling != except {
			sibling.Cancel()
		}
	}
}

// Done exposes the task's cancellation signal for hosts that manage their
// own goroutine, such as the VM's OpTask, instead of using Spawn.
func (t *Task) Done() <-chan struct{} {
	return t.ctx.Done()
}

// Await waits for this task to resolve, return its result, or propagate error
func (t *Task) Await() (object.Object, error) {
	return t.future.Await()
}

// Status returns the current lifecycle status
func (t *Task) Status() TaskStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.status
}

// WithTimeout runs a task with a deadline; cancels if deadline expires
//
// On expiry the enclosing scope is cancelled, not just the child: a timeout
// that left the parent running would let siblings outlive the deadline that
// was supposed to bound them. The caller observes ErrTaskTimeout; work that
// was already cancelled for another reason keeps its own error.
func WithTimeout(parent *Task, timeout time.Duration, action func(ctx context.Context) (object.Object, error)) (object.Object, error) {
	scope := NewTaskScope(parent)
	var fired atomic.Bool
	timer := time.AfterFunc(timeout, func() {
		fired.Store(true)
		if parent != nil {
			parent.Cancel()
		} else {
			scope.Cancel()
		}
	})
	defer timer.Stop()

	child := scope.Spawn(action)
	val, err := child.Await()
	if errors.Is(err, ErrTaskCancelled) && fired.Load() {
		return nil, ErrTaskTimeout
	}
	return val, err
}
