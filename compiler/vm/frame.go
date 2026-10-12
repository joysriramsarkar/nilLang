package vm

import (
	"github.com/joysriramsarkar/nilLang/compiler/code"
	"github.com/joysriramsarkar/nilLang/compiler/object"
)

type Frame struct {
	cl          *object.Closure
	ip          int
	basePointer int

	// isPure records that the called function was declared `pure fn`. The
	// VM decrements its pure depth when this frame returns, which keeps the
	// depth exact across interleaved pure and effectful calls.
	isPure bool
}

func NewFrame(cl *object.Closure, basePointer int) *Frame {
	return &Frame{
		cl:          cl,
		ip:          -1,
		basePointer: basePointer,
	}
}

func (f *Frame) Instructions() code.Instructions {
	return f.cl.Fn.Instructions
}
