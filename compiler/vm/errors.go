package vm

import (
	"fmt"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// vmError builds a categorised runtime error for this engine.
//
// The VM unwinds by returning Go errors, so every failure site needs a category
// rather than free text. The evaluator raises the same category for the same
// fault, which is what lets the conformance suite assert that the two engines
// agree on what went wrong even though they report it through different
// channels.
func vmError(category object.ErrorCategory, format string, args ...interface{}) error {
	err := object.NewRuntimeError(category, fmt.Sprintf(format, args...))
	err.Engine = "vm"
	return err
}

// vmErrorWrap builds a categorised error that preserves a cause.
//
// Capability refusals use this: the category makes the failure uniform across
// engines, while the wrapped *capability.DeniedError keeps the operation name
// and the missing capabilities reachable through errors.As.
func vmErrorWrap(category object.ErrorCategory, cause error, format string, args ...interface{}) error {
	err := object.RuntimeErrorWrapf(category, cause, format, args...)
	err.Engine = "vm"
	return err
}

// The named constructors below exist so that call sites read as the
// specification does rather than repeating a category string.

func errDivisionByZero() error {
	return vmError(object.CategoryDivisionByZero, "division by zero")
}

func errModuloByZero() error {
	return vmError(object.CategoryDivisionByZero, "modulo by zero")
}

func errStackOverflow(format string, args ...interface{}) error {
	return vmError(object.CategoryStackOverflow, format, args...)
}

func errFrameOverflow(format string, args ...interface{}) error {
	return vmError(object.CategoryFrameOverflow, format, args...)
}

func errIndexOutOfBounds(format string, args ...interface{}) error {
	return vmError(object.CategoryIndexOutOfBounds, format, args...)
}

func errInvalidBytecode(format string, args ...interface{}) error {
	return vmError(object.CategoryInvalidBytecode, format, args...)
}

func errTypeMismatch(format string, args ...interface{}) error {
	return vmError(object.CategoryTypeError, format, args...)
}

func errUnsupported(format string, args ...interface{}) error {
	return vmError(object.CategoryUnsupported, format, args...)
}

func errUnknownOperation(format string, args ...interface{}) error {
	return vmError(object.CategoryUnknownOperation, format, args...)
}

func errTaskCancelled(format string, args ...interface{}) error {
	return vmError(object.CategoryTaskCancelled, format, args...)
}

func errTaskFaulted(format string, args ...interface{}) error {
	return vmError(object.CategoryTaskFaulted, format, args...)
}
