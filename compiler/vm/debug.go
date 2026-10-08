package vm

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// ErrPaused is returned by Run/Step/Continue when execution stops at a
// breakpoint, on single-step, or because Pause was requested. It is not a
// program error.
var ErrPaused = errors.New("nilang vm: paused")

// Location is a source position in the running program.
type Location struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Function string `json:"function"`
	IP       int    `json:"ip"`
	Frame    int    `json:"frame"`
}

// Variable is a named debug value.
type Variable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}

// StackFrame is one entry of the call stack.
type StackFrame struct {
	Index    int    `json:"index"`
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

// DebugState holds breakpoints and stepping state for a VM.
type DebugState struct {
	mu          sync.Mutex
	enabled     bool
	breakpoints map[string]map[int]bool
	singleStep  bool
	skip        *Location
	pauseReq    bool
	last        Location
	onPause     func(Location)
}

// NewDebugState returns an enabled debugger state.
func NewDebugState() *DebugState {
	return &DebugState{enabled: true, breakpoints: map[string]map[int]bool{}}
}

// Enabled reports whether debugging is active.
func (d *DebugState) Enabled() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.enabled
}

// SetEnabled toggles debugging; when disabled the VM runs at full speed.
func (d *DebugState) SetEnabled(v bool) {
	d.mu.Lock()
	d.enabled = v
	d.mu.Unlock()
}

// OnPause registers a callback invoked (in a new goroutine) when the VM pauses.
func (d *DebugState) OnPause(fn func(Location)) {
	d.mu.Lock()
	d.onPause = fn
	d.mu.Unlock()
}

// SetBreakpoints replaces the breakpoints for a file.
func (d *DebugState) SetBreakpoints(file string, lines []int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	set := map[int]bool{}
	for _, ln := range lines {
		if ln > 0 {
			set[ln] = true
		}
	}
	if len(set) == 0 {
		delete(d.breakpoints, file)
		return
	}
	d.breakpoints[file] = set
}

// AddBreakpoint adds a single breakpoint.
func (d *DebugState) AddBreakpoint(file string, line int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.breakpoints[file] == nil {
		d.breakpoints[file] = map[int]bool{}
	}
	d.breakpoints[file][line] = true
}

// ClearBreakpoints removes all breakpoints.
func (d *DebugState) ClearBreakpoints() {
	d.mu.Lock()
	d.breakpoints = map[string]map[int]bool{}
	d.mu.Unlock()
}

// Breakpoints returns the current breakpoints as file -> sorted lines.
func (d *DebugState) Breakpoints() map[string][]int {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := map[string][]int{}
	for f, set := range d.breakpoints {
		lines := make([]int, 0, len(set))
		for ln := range set {
			lines = append(lines, ln)
		}
		sort.Ints(lines)
		out[f] = lines
	}
	return out
}

// Step arms single-step execution: it executes the paused instruction and
// stops at the next source location.
func (d *DebugState) Step() {
	d.mu.Lock()
	d.singleStep = true
	d.pauseReq = false
	loc := d.last
	d.skip = &loc
	d.mu.Unlock()
}

// Continue clears single-step and skips the current location so a breakpoint
// we are already stopped at does not immediately re-trigger.
func (d *DebugState) Continue() {
	d.mu.Lock()
	d.singleStep = false
	d.pauseReq = false
	loc := d.last
	d.skip = &loc
	d.mu.Unlock()
}

// Pause requests that the VM stop before its next instruction.
func (d *DebugState) Pause() {
	d.mu.Lock()
	d.pauseReq = true
	d.mu.Unlock()
}

// LastLocation returns the last observed location.
func (d *DebugState) LastLocation() Location {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.last
}

// check is called by Run before each instruction. It returns true when the VM
// should pause.
func (d *DebugState) check(vm *VM) bool {
	loc := vm.Location()

	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.enabled {
		return false
	}
	d.last = loc

	if d.pauseReq {
		d.pauseReq = false
		d.last = loc
		d.notify(loc)
		return true
	}
	if d.skip != nil && *d.skip == loc {
		d.skip = nil
		return false
	}
	if d.singleStep {
		d.singleStep = false
		d.notify(loc)
		return true
	}
	if lines, ok := d.breakpoints[loc.File]; ok && lines[loc.Line] {
		d.notify(loc)
		return true
	}
	return false
}

func (d *DebugState) notify(loc Location) {
	if d.onPause != nil {
		go d.onPause(loc)
	}
}

// Location returns the current source location of the VM.
func (vm *VM) Location() Location {
	if vm.framesIndex == 0 {
		return Location{}
	}
	f := vm.currentFrame()
	fn := f.cl.Fn
	line, col := fn.LineAt(f.ip)
	return Location{
		File:     fn.SourceFile,
		Line:     line,
		Column:   col,
		Function: fn.Name,
		IP:       f.ip,
		Frame:    vm.framesIndex - 1,
	}
}

// CallStack returns the call stack, innermost frame first.
func (vm *VM) CallStack() []StackFrame {
	out := make([]StackFrame, 0, vm.framesIndex)
	for i := vm.framesIndex - 1; i >= 0; i-- {
		f := vm.frames[i]
		if f == nil || f.cl == nil || f.cl.Fn == nil {
			continue
		}
		line, col := f.cl.Fn.LineAt(f.ip)
		out = append(out, StackFrame{
			Index:    i,
			Function: f.cl.Fn.Name,
			File:     f.cl.Fn.SourceFile,
			Line:     line,
			Column:   col,
		})
	}
	return out
}

// Locals returns the local variables of the current frame.
func (vm *VM) Locals() []Variable {
	if vm.framesIndex == 0 {
		return nil
	}
	f := vm.currentFrame()
	fn := f.cl.Fn
	out := make([]Variable, 0, fn.NumLocals)
	for i := 0; i < fn.NumLocals; i++ {
		addr := f.basePointer + i
		if addr < 0 || addr >= len(vm.stack) {
			break
		}
		out = append(out, Variable{Name: fn.LocalName(i), Value: inspectObject(vm.stack[addr]), Type: objectType(vm.stack[addr])})
	}
	return out
}

// Globals returns the non-nil globals.
func (vm *VM) Globals() []Variable {
	out := []Variable{}
	for i, o := range vm.globals {
		if o == nil {
			continue
		}
		name := fmt.Sprintf("global[%d]", i)
		if i < len(vm.globalNames) && vm.globalNames[i] != "" {
			name = vm.globalNames[i]
		}
		out = append(out, Variable{Name: name, Value: inspectObject(o), Type: objectType(o)})
	}
	return out
}

// FreeVars returns the captured variables of the current closure.
func (vm *VM) FreeVars() []Variable {
	if vm.framesIndex == 0 {
		return nil
	}
	cl := vm.currentFrame().cl
	out := make([]Variable, 0, len(cl.Free))
	for i, cell := range cl.Free {
		if cell == nil {
			continue
		}
		out = append(out, Variable{Name: fmt.Sprintf("free[%d]", i), Value: inspectObject(cell.Value), Type: objectType(cell.Value)})
	}
	return out
}

// EvaluateName resolves a simple name against locals, then globals.
func (vm *VM) EvaluateName(name string) (Variable, bool) {
	for i, v := range vm.Locals() {
		if v.Name == name {
			_ = i
			return v, true
		}
	}
	for _, v := range vm.Globals() {
		if v.Name == name {
			return v, true
		}
	}
	for _, v := range vm.FreeVars() {
		if v.Name == name {
			return v, true
		}
	}
	return Variable{}, false
}

// Step executes exactly one instruction and returns when paused or finished.
// A program error is returned as-is.
func (vm *VM) Step() error {
	if vm.Debug == nil {
		vm.Debug = NewDebugState()
	}
	vm.Debug.Step()
	err := vm.Run()
	if errors.Is(err, ErrPaused) {
		return nil
	}
	return err
}

// Continue resumes execution until the next breakpoint, pause, or program end.
func (vm *VM) Continue() error {
	if vm.Debug == nil {
		vm.Debug = NewDebugState()
	}
	vm.Debug.Continue()
	err := vm.Run()
	if errors.Is(err, ErrPaused) {
		return nil
	}
	return err
}

func inspectObject(o object.Object) string {
	if o == nil {
		return "nil"
	}
	return o.Inspect()
}

func objectType(o object.Object) string {
	if o == nil {
		return ""
	}
	return string(o.Type())
}
