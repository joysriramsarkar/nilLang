package object

import (
	"errors"
	"fmt"
)

// ErrorCategory is the closed set of runtime failure kinds defined by
// ERROR_MODEL.md §4.
//
// Every execution engine must report the same category for the same fault. The
// category is what makes "both engines halt identically" checkable, and it is
// what a mobile host uses to decide between a crash report and a user facing
// message.
type ErrorCategory string

const (
	// CategoryDivisionByZero is integer division or modulo by zero.
	CategoryDivisionByZero ErrorCategory = "DivisionByZero"

	// CategoryStackOverflow means the evaluation stack exceeded its allocation.
	CategoryStackOverflow ErrorCategory = "StackOverflow"

	// CategoryFrameOverflow means the call frame depth exceeded MaxFrames.
	CategoryFrameOverflow ErrorCategory = "FrameOverflow"

	// CategoryIndexOutOfBounds is an array index below 0 or at/above length.
	CategoryIndexOutOfBounds ErrorCategory = "IndexOutOfBounds"

	// CategoryInvalidBytecode is an unknown opcode or a malformed instruction
	// sequence.
	CategoryInvalidBytecode ErrorCategory = "InvalidBytecode"

	// CategoryCapabilityDenied is a native operation refused for want of a
	// capability. See CAPABILITIES.md.
	CategoryCapabilityDenied ErrorCategory = "CapabilityDenied"

	// CategoryTaskCancelled is a task cancelled by its parent scope or a
	// timeout. See CONCURRENCY.md.
	CategoryTaskCancelled ErrorCategory = "TaskCancelled"

	// CategoryTaskFaulted is a task that terminated with an error.
	CategoryTaskFaulted ErrorCategory = "TaskFaulted"

	// CategoryTypeError is an operation applied to a value of the wrong type.
	CategoryTypeError ErrorCategory = "TypeError"

	// CategoryUnknownOperation is a native operation with no implementation.
	CategoryUnknownOperation ErrorCategory = "UnknownOperation"

	// CategoryUnsupported is a valid construct the current backend cannot
	// execute, such as a WebAssembly backend meeting a float operation it does
	// not yet implement.
	CategoryUnsupported ErrorCategory = "Unsupported"
)

// validCategories is the closed set. A constructor refuses to mint an error
// outside it, so a typo cannot silently create an unreportable category.
var validCategories = map[ErrorCategory]bool{
	CategoryDivisionByZero:   true,
	CategoryStackOverflow:    true,
	CategoryFrameOverflow:    true,
	CategoryIndexOutOfBounds: true,
	CategoryInvalidBytecode:  true,
	CategoryCapabilityDenied: true,
	CategoryTaskCancelled:    true,
	CategoryTaskFaulted:      true,
	CategoryTypeError:        true,
	CategoryUnknownOperation: true,
	CategoryUnsupported:      true,
}

// IsValidCategory reports whether c is part of the closed set.
func IsValidCategory(c ErrorCategory) bool {
	return validCategories[c]
}

// RuntimeError is a runtime failure with a machine readable category.
//
// The tree-walking evaluator returns one of these as an ordinary Object, since
// it unwinds by returning values. The stack VM returns it as a Go error, since
// it unwinds by returning from Run. Both carry the same category, which is the
// property the conformance suite asserts.
type RuntimeError struct {
	// Category is the closed-set failure kind.
	Category ErrorCategory

	// Message is human readable text, without the category prefix.
	Message string

	// Engine names the execution engine that raised the error ("evaluator",
	// "vm", "wasm"). It is diagnostic context, not part of the contract: the
	// two engines must agree on Category even though Engine necessarily differs.
	Engine string

	// cause is an underlying error kept for inspection, such as a
	// capability.DeniedError. The VM surfaces a capability refusal as a
	// RuntimeError so that every failure carries a category, and wrapping
	// rather than discarding the cause is what keeps the structured detail
	// reachable. See Unwrap.
	cause error
}

// NewRuntimeError builds a RuntimeError with the default engine label.
func NewRuntimeError(category ErrorCategory, message string) *RuntimeError {
	return &RuntimeError{Category: category, Message: message, Engine: "nilang"}
}

// WrapRuntimeError builds a RuntimeError that preserves an underlying cause.
func WrapRuntimeError(category ErrorCategory, message string, cause error) *RuntimeError {
	return &RuntimeError{
		Category: category,
		Message:  message,
		Engine:   "nilang",
		cause:    cause,
	}
}

// Unwrap exposes the underlying cause so that errors.As and errors.Is reach
// through the category wrapper to a more specific error type.
func (e *RuntimeError) Unwrap() error {
	return e.cause
}

func (e *RuntimeError) Type() ObjectType { return ERROR_OBJ }

// Inspect renders the error as "ERROR: Category: message", matching the shape
// the conformance suite and CLI output already expect.
func (e *RuntimeError) Inspect() string {
	return "ERROR: " + string(e.Category) + ": " + e.Message
}

// Error makes RuntimeError usable as a Go error, which is how the VM surfaces
// it. Returning it directly from Run means a host can inspect Category without
// parsing a message.
func (e *RuntimeError) Error() string {
	return e.Inspect()
}

// RuntimeErrorf builds a RuntimeError with a formatted message.
func RuntimeErrorf(category ErrorCategory, format string, args ...interface{}) *RuntimeError {
	return NewRuntimeError(category, fmt.Sprintf(format, args...))
}

// RuntimeErrorWrapf builds a categorised error that preserves a cause.
func RuntimeErrorWrapf(category ErrorCategory, cause error, format string, args ...interface{}) *RuntimeError {
	return WrapRuntimeError(category, fmt.Sprintf(format, args...), cause)
}

// CategoryOf extracts the category from an object or Go error.
//
// It returns false when the value carries no category, which lets a caller
// distinguish "an error with a category" from "a bare failure". The
// tree-walking evaluator's legacy *Error is the common case of the latter, and
// reporting false for it keeps the difference visible rather than inventing a
// category.
func CategoryOf(value interface{}) (ErrorCategory, bool) {
	switch v := value.(type) {
	case *RuntimeError:
		return v.Category, true
	case nil:
		return "", false
	}

	// Anything implementing error may wrap a *RuntimeError, either directly or
	// through fmt.Errorf("%w"), so unwrap rather than type asserting.
	if err, ok := value.(error); ok {
		var runtimeErr *RuntimeError
		if errors.As(err, &runtimeErr) {
			return runtimeErr.Category, true
		}
	}

	return "", false
}
