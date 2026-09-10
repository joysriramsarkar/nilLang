package concurrency

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

func TestTaskNormalExecution(t *testing.T) {
	root := NewTaskScope(nil)
	task := root.Spawn(func(ctx context.Context) (object.Object, error) {
		return &object.Integer{Value: 100}, nil
	})

	val, err := task.Await()
	if err != nil {
		t.Fatalf("unexpected task error: %v", err)
	}

	intObj, ok := val.(*object.Integer)
	if !ok || intObj.Value != 100 {
		t.Fatalf("expected 100, got %v", val)
	}

	if task.Status() != StatusFulfilled {
		t.Fatalf("expected fulfilled, got %s", task.Status())
	}
}

func TestStructuredCancellationPropagation(t *testing.T) {
	root := NewTaskScope(nil)

	child1 := root.Spawn(func(ctx context.Context) (object.Object, error) {
		select {
		case <-ctx.Done():
			return nil, ErrTaskCancelled
		case <-time.After(2 * time.Second):
			return &object.String{Value: "completed"}, nil
		}
	})

	child2 := root.Spawn(func(ctx context.Context) (object.Object, error) {
		select {
		case <-ctx.Done():
			return nil, ErrTaskCancelled
		case <-time.After(2 * time.Second):
			return &object.String{Value: "completed"}, nil
		}
	})

	// Cancel root task
	root.Cancel()

	_, err1 := child1.Await()
	if !errors.Is(err1, ErrTaskCancelled) {
		t.Fatalf("expected child1 to be cancelled, got %v", err1)
	}

	_, err2 := child2.Await()
	if !errors.Is(err2, ErrTaskCancelled) {
		t.Fatalf("expected child2 to be cancelled, got %v", err2)
	}
}

func TestTaskTimeout(t *testing.T) {
	root := NewTaskScope(nil)
	_, err := WithTimeout(root, 50*time.Millisecond, func(ctx context.Context) (object.Object, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
			return &object.Integer{Value: 1}, nil
		}
	})

	if !errors.Is(err, ErrTaskTimeout) && !errors.Is(err, ErrTaskCancelled) {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestTaskPanicIsolation(t *testing.T) {
	root := NewTaskScope(nil)
	task := root.Spawn(func(ctx context.Context) (object.Object, error) {
		panic("simulated unhandled runtime panic in task")
	})

	_, err := task.Await()
	if err == nil {
		t.Fatal("expected panic to be converted into task error")
	}

	if task.Status() != StatusFaulted {
		t.Fatalf("expected status Faulted, got %s", task.Status())
	}
}

func TestConcurrentChildTaskAggregation(t *testing.T) {
	root := NewTaskScope(nil)
	const count = 20
	tasks := make([]*Task, count)

	for i := 0; i < count; i++ {
		val := int64(i)
		tasks[i] = root.Spawn(func(ctx context.Context) (object.Object, error) {
			// Do work
			time.Sleep(10 * time.Millisecond)
			return &object.Integer{Value: val}, nil
		})
	}

	var sum int64
	for _, task := range tasks {
		res, err := task.Await()
		if err != nil {
			t.Fatalf("task failed: %v", err)
		}
		intVal, ok := res.(*object.Integer)
		if !ok {
			t.Fatalf("expected *object.Integer, got %T", res)
		}
		sum += intVal.Value
	}

	// Sum 0..19 = (19 * 20) / 2 = 190
	expectedSum := int64(190)
	if sum != expectedSum {
		t.Fatalf("expected sum %d, got %d", expectedSum, sum)
	}
}
