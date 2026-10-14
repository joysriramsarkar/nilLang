package lower

import (
	"fmt"
)

// Error is a lowering failure: the program parsed and checked, but a
// construct cannot be translated into executable form (`break` outside a
// loop, a match arm with no body).
//
// It carries the source line because that is what turns "something is wrong
// somewhere" into a diagnostic with a span: the frontend renders Message
// beside the offending line instead of reprinting the line number it already
// shows. Error() keeps the historical "line N: ..." rendering so message
// assertions and logs read exactly as before.
type Error struct {
	Line int
	Msg  string
}

func (e *Error) Error() string {
	if e == nil {
		return "lowering failed"
	}
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

// ErrorLine recovers the source line from a lowering error, or -1 when the
// error carries none (a nil program has no line to name).
func ErrorLine(err error) int {
	if err == nil {
		return -1
	}
	if as, ok := err.(*Error); ok && as != nil {
		return as.Line
	}
	return -1
}

// ErrorMessage is the message without the "line N: " prefix, for renderers
// that show the span themselves.
func ErrorMessage(err error) string {
	if as, ok := err.(*Error); ok && as != nil {
		return as.Msg
	}
	if err == nil {
		return ""
	}
	return err.Error()
}
