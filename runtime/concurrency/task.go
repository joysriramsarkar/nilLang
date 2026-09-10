package concurrency

import (
	"context"
	"errors"
	"fmt"
	"sync"
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
	child.status = StatusRunning

	go func() {
		defer func() {
			if r := recover(); r != nil {
				child.mu.Lock()
				child.status = StatusFaulted
				child.mu.Unlock()
				child.future.Complete(nil, fmt.Errorf("task panicked: %v", r))
			}
		}()

		// Check if already cancelled
		select {
		case <-child.ctx.Done():
			child.mu.Lock()
			child.status = StatusCancelled
			child.mu.Unlock()
			child.future.Complete(nil, ErrTaskCancelled)
			return
		default:
		}

		result, err := action(child.ctx)
		child.mu.Lock()
		defer child.mu.Unlock()

		if child.ctx.Err() != nil {
			child.status = StatusCancelled
			child.future.Complete(nil, ErrTaskCancelled)
			return
		}

		if err != nil {
			child.status = StatusFaulted
			child.future.Complete(nil, err)
		} else {
			child.status = StatusFulfilled
			child.future.Complete(result, nil)
		}
	}()

	return child
}

// Cancel cancels this task and propagates cancellation recursively to all children
func (t *Task) Cancel() {
	t.mu.Lock()
	t.status = StatusCancelled
	children := append([]*Task(nil), t.children...)
	t.mu.Unlock()

	t.cancel()

	for _, child := range children {
		child.Cancel()
	}

	t.future.Complete(nil, ErrTaskCancelled)
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
func WithTimeout(parent *Task, timeout time.Duration, action func(ctx context.Context) (object.Object, error)) (object.Object, error) {
	scope := NewTaskScope(parent)
	timer := time.AfterFunc(timeout, func() {
		scope.Cancel()
	})
	defer timer.Stop()

	child := scope.Spawn(action)
	val, err := child.Await()
	if errors.Is(err, ErrTaskCancelled) && scope.ctx.Err() != nil {
		return nil, ErrTaskTimeout
	}
	return val, err
}
