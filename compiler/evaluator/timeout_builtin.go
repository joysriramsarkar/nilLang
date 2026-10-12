package evaluator

import (
	"time"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// withTimeout is registered in init rather than in the Builtins literal.
//
// Its body calls applyFunction, and a reference to applyFunction inside the
// package-level Builtins initializer closes a genuine initialization cycle
// (Builtins -> applyFunction -> checkBuiltinCall -> BuiltinName -> Builtins)
// that the Go toolchain rejects at build time. Registering from init runs
// after all package-level variables are initialized, so the cycle cannot
// form. The compiler's builtin-name cache is populated lazily at first use,
// after init has run, so the late registration is visible to it.
func init() {
	Builtins["withTimeout"] = &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("wrong number of arguments to `withTimeout`. got=%d, want=2", len(args))
			}
			timeout, ok := args[0].(*object.Integer)
			if !ok {
				return newError("first argument to `withTimeout` must be INTEGER milliseconds, got %s", args[0].Type())
			}
			fn, ok := args[1].(*object.Function)
			if !ok {
				return newError("second argument to `withTimeout` must be a function, got %s", args[1].Type())
			}
			// The timed call runs off-goroutine because the language has no
			// preemption: the deadline is observed by giving up waiting, not
			// by stopping the work. An abandoned call keeps running in the
			// background; a panic in it faults the result rather than the
			// process, matching task fault isolation (CONCURRENCY.md §4).
			done := make(chan object.Object, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						done <- object.RuntimeErrorf(object.CategoryTaskFaulted, "task panicked: %v", r)
					}
				}()
				done <- applyFunction(fn, []object.Object{}, dotenv(fn))
			}()
			select {
			case result := <-done:
				return result
			case <-time.After(time.Duration(timeout.Value) * time.Millisecond):
				return object.RuntimeErrorf(object.CategoryTaskCancelled,
					"withTimeout: task timeout exceeded after %dms", timeout.Value)
			}
		},
	}
}
