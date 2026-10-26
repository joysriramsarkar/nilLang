package vm

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	compbuiltin "github.com/joysriramsarkar/nilLang/compiler/builtin"
	"github.com/joysriramsarkar/nilLang/compiler/code"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/runtime/concurrency"
)

const (
	// StackSize is the operand stack capacity in slots.
	//
	// It must be large enough that MaxFrames is always the binding constraint
	// rather than the stack, because which limit trips first determines the
	// error category. When StackSize bound first, deep recursion reported
	// StackOverflow while the tree-walking evaluator reported FrameOverflow for
	// the same program, breaking the cross-engine agreement that
	// ERROR_MODEL.md §4 requires.
	//
	// A frame consumes at least the callee slot plus one slot per local, so a
	// conservative worst case is a handful of slots per frame. 8x MaxFrames
	// leaves ample headroom for functions with many locals.
	StackSize = MaxFrames * 8

	GlobalsSize = 65536

	// MaxFrames is the call frame depth limit. It is deliberately the tighter of
	// the two limits: a program that recurses without bound should be told it
	// ran out of frames, not that it ran out of operand stack.
	MaxFrames = 1024
)

var (
	True  = object.TRUE
	False = object.FALSE
	Null  = object.NULL
)

type VM struct {
	constants []object.Object

	stack []object.Object
	sp    int // Always points to the next value. Top of stack is stack[sp-1]

	globals []object.Object

	// globalsMu guards the global store. Tasks run on their own goroutines
	// while sharing their parent's globals (L6.2: the same backing slice,
	// never a copy), so every global access takes this lock. A child VM
	// created for a task shares the pointer, which is what makes a write in
	// the task visible to the parent after the await.
	globalsMu *sync.Mutex

	// taskScope is the structured-concurrency scope tasks spawned by OpTask
	// belong to (L6.1). It is created lazily so that VMs deserialised from
	// older images, which never had one, still run. Every OpTask creates a
	// child scope under it; a faulted child cancels its siblings through the
	// same parent, and cancelling this scope cascades to the whole tree.
	taskScope *concurrency.Task

	// pureDepth counts the pure functions on the current call stack (L6.3).
	// A nonzero depth makes an effectful call a runtime E0202, the dynamic
	// half of the static check the type checker performs. Each VM tracks its
	// own depth because each runs on a single goroutine.
	pureDepth int

	// cancellable marks a VM that runs a task body. Its Run loop polls the
	// task scope's cancellation signal, which is what makes cascading and
	// sibling cancellation stop the work rather than merely recording that
	// it should. The main VM never sets this: polling on every instruction
	// would tax programs that cannot be cancelled.
	cancellable bool

	// cancelCounter spaces the cancellation polls: one non-blocking check
	// per 256 instructions keeps the steady-state cost to a counter bump.
	cancelCounter uint64

	// allocCounter spaces root-set refreshes the same way: rebuilding the
	// set on every allocation would rescan the globals on every allocation.
	allocCounter uint64

	// taskChildren records the scopes OpTask spawns, in spawn order. It
	// exists for the debugger and for tests: the scope is what carries the
	// tree relationship a fault or a cancellation travels through.
	// taskChildrenMu guards it because a test may read the scopes while the
	// VM that spawned them is still running.
	taskChildren   []*concurrency.Task
	taskChildrenMu sync.Mutex

	// globalNames maps global slot indices to debug names (may be empty).
	globalNames []string

	frames      []*Frame
	framesIndex int

	lastPoppedStackElem object.Object

	NativeCallHandler func(name string, args []object.Object) (object.Object, error)
	CapabilityChecker func(apiName string) error

	// Debug, when non-nil, enables source-level debugging (breakpoints,
	// stepping and inspection).
	Debug *DebugState

	// Heap, when non-nil, receives every heap allocation the VM makes and
	// supplies the root set at collection time.
	//
	// It is optional because the collector is conservative bookkeeping: it
	// makes the reachability model observable but does not free memory the Go
	// allocator owns. A host that embeds the VM sets it to get reclaim counts
	// and to prove that live structures are retained; the CLI leaves it nil to
	// avoid the bookkeeping cost on every push.
	Heap Heap

	// gcTracking is true when Heap is configured.
	gcTracking bool

	// resumeSameIP is set when Run returns ErrPaused so the next Run resumes
	// at the paused instruction instead of skipping it.
	resumeSameIP bool
}

// Heap is the interface the VM needs from a garbage collector.
//
// It is an interface rather than a concrete type so that compiler/vm does not
// import runtime/gc, which keeps the dependency direction the way
// tests/architecture_test.go expects: the runtime may know about the VM's object
// model, not the other way round.
type Heap interface {
	// Track registers an allocation.
	Track(obj object.Object)

	// SetRoots replaces the root set for the next collection.
	SetRoots(roots []object.Object)

	// Collect runs a cycle and returns how many objects were reclaimed.
	Collect() int

	// ObjectCount returns the number of tracked objects.
	ObjectCount() int
}

// EnableGC attaches a collector to the VM.
//
// Allocation tracking starts immediately, and a collection runs when the tracked
// object count crosses the heap's threshold.
func (vm *VM) EnableGC(heap Heap) {
	vm.Heap = heap
	vm.gcTracking = heap != nil
}

// DisableGC detaches the collector.
func (vm *VM) DisableGC() {
	vm.Heap = nil
	vm.gcTracking = false
}

// GCEnabled reports whether a collector is attached.
func (vm *VM) GCEnabled() bool {
	return vm.gcTracking
}

// allocateObject registers a freshly created heap object with the collector.
//
// Only genuinely heap allocated values are tracked. Integers, booleans and the
// like are values rather than references, and tracing them would grow the
// marked set without making anything reachable that was not already.
func (vm *VM) allocateObject(obj object.Object) {
	if !vm.gcTracking || obj == nil {
		return
	}
	// Threshold collections inside Track use the registered roots, so the set
	// is rebuilt periodically — same 256-cadence as the cancellation polls.
	// Fresh frames (calls) and freshly spawned task scopes otherwise enter the
	// tracked world invisible until the next manual collection.
	vm.allocCounter++
	if vm.allocCounter&0xFF == 0 {
		vm.Heap.SetRoots(vm.GCRoots())
	}
	vm.Heap.Track(obj)
}

// GCRoots builds the root set for a collection: live operand stack slots,
// globals, and the closures and cells held by active frames (MEMORY_MODEL.md §2).
func (vm *VM) GCRoots() []object.Object {
	roots := make([]object.Object, 0, vm.sp+vm.framesIndex+16)

	for i := 0; i < vm.sp && i < len(vm.stack); i++ {
		if obj := vm.stack[i]; obj != nil {
			roots = append(roots, obj)
		}
	}

	for i := 0; i < vm.framesIndex && i < len(vm.frames); i++ {
		frame := vm.frames[i]
		if frame == nil {
			continue
		}
		if frame.cl != nil {
			roots = append(roots, frame.cl)
			for _, free := range frame.cl.Free {
				roots = append(roots, free)
			}
		}
	}

	for i := 0; i < len(vm.globals); i++ {
		mu := vm.ensureGlobalsMu()
		mu.Lock()
		obj := vm.globals[i]
		mu.Unlock()
		if obj != nil {
			roots = append(roots, obj)
		}
	}

	if vm.lastPoppedStackElem != nil {
		roots = append(roots, vm.lastPoppedStackElem)
	}

	// In-flight tasks hold futures whose results may not be awaited yet. A
	// future referenced nowhere else — spawned and forgotten until joined —
	// is still live state, so the scheduler's whole tree is walked.
	if vm.taskScope != nil {
		for _, fut := range vm.taskScope.CollectFutures() {
			if fut != nil {
				roots = append(roots, fut)
			}
		}
	}

	return roots
}

// CollectGarbage runs a collection against the VM's current root set and returns
// the number of reclaimed objects.
func (vm *VM) CollectGarbage() int {
	if !vm.gcTracking {
		return 0
	}
	vm.Heap.SetRoots(vm.GCRoots())
	return vm.Heap.Collect()
}

// TrackedObjectCount returns the number of objects the collector is tracking.
func (vm *VM) TrackedObjectCount() int {
	if !vm.gcTracking {
		return 0
	}
	return vm.Heap.ObjectCount()
}

func New(bytecode *compiler.Bytecode) *VM {
	mainFn := &object.CompiledFunction{
		Instructions: bytecode.Instructions,
		Name:         "<main>",
		SourceFile:   bytecode.SourceFile,
		Positions:    bytecode.Positions,
	}
	mainClosure := &object.Closure{Fn: mainFn}
	mainFrame := NewFrame(mainClosure, 0)

	frames := make([]*Frame, MaxFrames)
	frames[0] = mainFrame

	return &VM{
		constants: bytecode.Constants,

		stack: make([]object.Object, StackSize),
		sp:    0,

		globals:     make([]object.Object, GlobalsSize),
		globalsMu:   &sync.Mutex{},
		taskScope:   concurrency.NewTaskScope(nil),
		globalNames: bytecode.GlobalNames,

		frames:      frames,
		framesIndex: 1,
	}
}

func NewWithGlobalsStore(bytecode *compiler.Bytecode, s []object.Object) *VM {
	vm := New(bytecode)
	vm.globals = s
	return vm
}

// ensureTaskScope returns the VM's task scope, creating the root on first
// use for VMs that predate the field.
func (vm *VM) ensureTaskScope() *concurrency.Task {
	if vm.taskScope == nil {
		vm.taskScope = concurrency.NewTaskScope(nil)
	}
	return vm.taskScope
}

// ensureGlobalsMu returns the mutex guarding the global store, creating one
// for VMs that predate the field. VMs that share a store must share the
// pointer; this is only the fallback for a VM that has none.
func (vm *VM) ensureGlobalsMu() *sync.Mutex {
	if vm.globalsMu == nil {
		vm.globalsMu = &sync.Mutex{}
	}
	return vm.globalsMu
}

// TaskChildScopes returns a copy of the scopes OpTask has spawned so far,
// in spawn order. It exists for the debugger and for tests.
func (vm *VM) TaskChildScopes() []*concurrency.Task {
	vm.taskChildrenMu.Lock()
	defer vm.taskChildrenMu.Unlock()
	return append([]*concurrency.Task(nil), vm.taskChildren...)
}

func (vm *VM) StackTop() object.Object {
	if vm.sp == 0 {
		return nil
	}
	return vm.stack[vm.sp-1]
}

func (vm *VM) LastPoppedStackElem() object.Object {
	return vm.lastPoppedStackElem
}

// GetGlobal returns the value stored in a global slot, or nil when the slot is
// empty or out of range.
//
// Globals live in the VM's global array rather than on the operand stack, so
// observing a program's effects after Run requires reading a slot by index.
// The debugger, the conformance suite, and embedders all need this, and the
// field is otherwise unreachable from outside the package.
func (vm *VM) GetGlobal(index int) object.Object {
	if index < 0 || index >= len(vm.globals) {
		return nil
	}
	mu := vm.ensureGlobalsMu()
	mu.Lock()
	defer mu.Unlock()
	return vm.globals[index]
}

// SetGlobal writes a value into a global slot, ignoring out of range indices.
// It exists so that embedders and tests can seed global state before Run.
func (vm *VM) SetGlobal(index int, value object.Object) {
	if index < 0 || index >= len(vm.globals) {
		return
	}
	mu := vm.ensureGlobalsMu()
	mu.Lock()
	defer mu.Unlock()
	vm.globals[index] = value
}

// GlobalsSize reports how many global slots the VM provides.
func (vm *VM) GlobalsSize() int {
	return len(vm.globals)
}

// GlobalNames returns the debug names of global slots, indexed by slot.
//
// Names come from the compiler's symbol table and are empty when the bytecode
// carries no debug information.
func (vm *VM) GlobalNames() []string {
	return vm.globalNames
}

// GlobalByName reads a global slot by its source-level name.
//
// Reading by index is fragile: a program that declares one extra binding shifts
// every later slot, so an index-based read in a test or a debugger silently
// inspects the wrong variable. The name is what the developer wrote.
func (vm *VM) GlobalByName(name string) (object.Object, bool) {
	for slot, globalName := range vm.globalNames {
		if globalName == name {
			value := vm.GetGlobal(slot)
			return value, value != nil
		}
	}
	return nil, false
}

func (vm *VM) currentFrame() *Frame {
	return vm.frames[vm.framesIndex-1]
}

func (vm *VM) pushFrame(f *Frame) error {
	if vm.framesIndex >= MaxFrames {
		return errFrameOverflow("maximum call depth %d exceeded", MaxFrames)
	}
	vm.frames[vm.framesIndex] = f
	vm.framesIndex++
	return nil
}

func (vm *VM) popFrame() (*Frame, error) {
	if vm.framesIndex <= 0 {
		return nil, errInvalidBytecode("frame stack underflow")
	}
	vm.framesIndex--
	frame := vm.frames[vm.framesIndex]
	vm.frames[vm.framesIndex] = nil
	return frame, nil
}

func (vm *VM) push(o object.Object) error {
	if vm.sp >= StackSize {
		return errStackOverflow("evaluation stack exhausted")
	}

	vm.stack[vm.sp] = o
	vm.sp++

	return nil
}

func (vm *VM) pop() (object.Object, error) {
	if vm.sp <= 0 {
		return nil, errInvalidBytecode("stack underflow")
	}
	o := vm.stack[vm.sp-1]
	vm.truncateStack(vm.sp - 1)
	return o, nil
}

func (vm *VM) truncateStack(newSP int) {
	for index := newSP; index < vm.sp; index++ {
		vm.stack[index] = nil
	}
	vm.sp = newSP
}

// readU16 reads a two-byte operand at ip+1, refusing truncated bytecode
// instead of slicing past the end of the instruction stream.
func (vm *VM) readU16(ins code.Instructions, ip int) (uint16, error) {
	if ip < 0 || ip+2 >= len(ins) {
		return 0, errInvalidBytecode("truncated instruction operand")
	}
	return code.ReadUint16(ins[ip+1:]), nil
}

// readU8 reads a one-byte operand at ip+1, refusing truncated bytecode.
func (vm *VM) readU8(ins code.Instructions, ip int) (uint8, error) {
	if ip < 0 || ip+1 >= len(ins) {
		return 0, errInvalidBytecode("truncated instruction operand")
	}
	return code.ReadUint8(ins[ip+1:]), nil
}

// readU8At reads a one-byte operand at an absolute position, for instructions
// whose second operand does not sit at ip+1 (OpClosure, OpNativeCall).
func (vm *VM) readU8At(ins code.Instructions, at int) (uint8, error) {
	if at < 0 || at >= len(ins) {
		return 0, errInvalidBytecode("truncated instruction operand")
	}
	return code.ReadUint8(ins[at:]), nil
}

// constantAt fetches a constant by index. Hand-built bytecode can name any
// index, so this is a bounds check, not an assertion about the compiler.
func (vm *VM) constantAt(index int) (object.Object, error) {
	if index < 0 || index >= len(vm.constants) {
		return nil, errInvalidBytecode("constant index %d out of range (%d constants)", index, len(vm.constants))
	}
	c := vm.constants[index]
	if c == nil {
		return nil, errInvalidBytecode("constant %d is nil", index)
	}
	return c, nil
}

// globalAt reads a global slot by index.
func (vm *VM) globalAt(index int) (object.Object, error) {
	if index < 0 || index >= len(vm.globals) {
		return nil, errInvalidBytecode("global index %d out of range (%d globals)", index, len(vm.globals))
	}
	return vm.globals[index], nil
}

// checkJumpTarget refuses jump destinations outside the instruction stream.
// Without it a jump past the end silently succeeds and one before the start
// re-executes arbitrary bytes.
func (vm *VM) checkJumpTarget(pos, length int) error {
	if pos < 0 || pos >= length {
		return errInvalidBytecode("jump target %d out of range (%d bytes)", pos, length)
	}
	return nil
}

// typeName names an object for diagnostics without panicking on nil.
func typeName(obj object.Object) string {
	if obj == nil {
		return "nil"
	}
	return string(obj.Type())
}

// freeCellAt fetches a captured-variable cell from the current closure.
// Hand-built bytecode can name any index and run outside any closure.
func (vm *VM) freeCellAt(index int) (*object.CaptureCell, error) {
	cl := vm.currentFrame().cl
	if cl == nil {
		return nil, errInvalidBytecode("free variable outside a function")
	}
	if index < 0 || index >= len(cl.Free) {
		return nil, errInvalidBytecode("free index %d out of range (%d cells)", index, len(cl.Free))
	}
	cell := cl.Free[index]
	if cell == nil {
		return nil, errInvalidBytecode("free variable %d is nil", index)
	}
	return cell, nil
}

func (vm *VM) Run() (err error) {
	// Hostile or corrupted bytecode must surface as an error, never as a Go
	// panic in the host process. Every known indexing site below is checked,
	// and this backstop converts anything missed into invalid-bytecode.
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok && e != nil {
				err = errInvalidBytecode("invalid bytecode: %s", e.Error())
			} else {
				err = errInvalidBytecode("invalid bytecode: %v", r)
			}
		}
	}()

	var ip int
	var ins code.Instructions
	var op code.Opcode

	for vm.framesIndex > 0 && vm.currentFrame().ip < len(vm.currentFrame().Instructions())-1 {
		if vm.cancellable {
			vm.cancelCounter++
			if vm.cancelCounter&0xFF == 0 && vm.taskScope != nil {
				select {
				case <-vm.taskScope.Done():
					return vmError(object.CategoryTaskCancelled, "task cancelled")
				default:
				}
			}
		}
		if vm.resumeSameIP {
			vm.resumeSameIP = false
		} else {
			vm.currentFrame().ip++
		}

		ip = vm.currentFrame().ip
		ins = vm.currentFrame().Instructions()
		if ip < 0 || ip >= len(ins) {
			return errInvalidBytecode("instruction pointer %d out of range (%d bytes)", ip, len(ins))
		}
		op = code.Opcode(ins[ip])

		if vm.Debug != nil {
			if vm.Debug.check(vm) {
				vm.resumeSameIP = true
				return ErrPaused
			}
		}

		switch op {
		case code.OpConstant:
			constIndex, operandErr := vm.readU16(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			vm.currentFrame().ip += 2

			constant, err := vm.constantAt(int(constIndex))
			if err != nil {
				return err
			}
			if err := vm.push(constant); err != nil {
				return err
			}

		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv, code.OpMod:
			err := vm.executeBinaryOperation(op)
			if err != nil {
				return err
			}

		case code.OpPop:
			val, err := vm.pop()
			if err != nil {
				return err
			}
			vm.lastPoppedStackElem = val

		case code.OpTrue:
			err := vm.push(True)
			if err != nil {
				return err
			}

		case code.OpFalse:
			err := vm.push(False)
			if err != nil {
				return err
			}

		case code.OpEqual, code.OpNotEqual, code.OpGreaterThan, code.OpGreaterThanEqual, code.OpLessThan, code.OpLessThanEqual:
			err := vm.executeComparison(op)
			if err != nil {
				return err
			}

		case code.OpAnd:
			right, err := vm.pop()
			if err != nil {
				return err
			}
			left, err := vm.pop()
			if err != nil {
				return err
			}
			if err := vm.push(nativeBoolToBooleanObject(isTruthy(left) && isTruthy(right))); err != nil {
				return err
			}

		case code.OpOr:
			right, err := vm.pop()
			if err != nil {
				return err
			}
			left, err := vm.pop()
			if err != nil {
				return err
			}
			if err := vm.push(nativeBoolToBooleanObject(isTruthy(left) || isTruthy(right))); err != nil {
				return err
			}

		case code.OpBang:
			err := vm.executeBangOperator()
			if err != nil {
				return err
			}

		case code.OpMinus:
			err := vm.executeMinusOperator()
			if err != nil {
				return err
			}

		case code.OpJump:
			jumpPos, operandErr := vm.readU16(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			if err := vm.checkJumpTarget(int(jumpPos), len(ins)); err != nil {
				return err
			}
			vm.currentFrame().ip = int(jumpPos) - 1

		case code.OpJumpNotTruthy:
			jumpPos, operandErr := vm.readU16(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			vm.currentFrame().ip += 2

			condition, err := vm.pop()
			if err != nil {
				return err
			}
			if !isTruthy(condition) {
				if err := vm.checkJumpTarget(int(jumpPos), len(ins)); err != nil {
					return err
				}
				vm.currentFrame().ip = int(jumpPos) - 1
			}

		case code.OpNull:
			err := vm.push(Null)
			if err != nil {
				return err
			}

		case code.OpSetGlobal:
			globalIndex, operandErr := vm.readU16(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			vm.currentFrame().ip += 2
			if int(globalIndex) >= len(vm.globals) {
				return errInvalidBytecode("global index out of range")
			}

			val, err := vm.pop()
			if err != nil {
				return err
			}
			mu := vm.ensureGlobalsMu()
			mu.Lock()
			// A loop-body `let` at top level stores a fresh cell per pass
			// (OpSetGlobalFresh). An assignment to that name must go through
			// the current cell, mirroring OpSetLocal, or the closure holding
			// it would never observe the write.
			if cell, ok := vm.globals[globalIndex].(*object.CaptureCell); ok {
				cell.Value = val
			} else {
				vm.globals[globalIndex] = val
			}
			mu.Unlock()

		case code.OpGetGlobal:
			globalIndex, operandErr := vm.readU16(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			vm.currentFrame().ip += 2
			if int(globalIndex) >= len(vm.globals) {
				return errInvalidBytecode("global index out of range")
			}

			mu := vm.ensureGlobalsMu()
			mu.Lock()
			global := vm.globals[globalIndex]
			mu.Unlock()
			// Unwrap per-pass loop cells transparently: arithmetic and calls
			// must see the value, never the cell. Only loop-body `let`
			// bindings at top level ever hold cells here.
			if cell, ok := global.(*object.CaptureCell); ok {
				global = cell.Value
			}
			err := vm.push(global)
			if err != nil {
				return err
			}

		case code.OpArray:
			numElementsRaw, operandErr := vm.readU16(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			numElements := int(numElementsRaw)
			vm.currentFrame().ip += 2

			if numElements < 0 || numElements > vm.sp {
				return errInvalidBytecode("array of %d elements with only %d values on the stack", numElements, vm.sp)
			}
			array := vm.buildArray(vm.sp-numElements, vm.sp)
			vm.truncateStack(vm.sp - numElements)

			// Track before pushing: the array is not yet reachable from the root
			// set, so a collection between allocation and push would sweep it
			// while it is still referenced by this instruction's own stack.
			vm.allocateObject(array)

			err := vm.push(array)
			if err != nil {
				return err
			}

		case code.OpHash:
			numHashElementsRaw, operandErr := vm.readU16(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			numElements := int(numHashElementsRaw)
			vm.currentFrame().ip += 2

			if numElements < 0 || numElements > vm.sp || numElements%2 != 0 {
				return errInvalidBytecode("hash of %d elements with %d values on the stack", numElements, vm.sp)
			}
			hash, err := vm.buildHash(vm.sp-numElements, vm.sp)
			if err != nil {
				return err
			}
			vm.truncateStack(vm.sp - numElements)

			vm.allocateObject(hash)

			err = vm.push(hash)
			if err != nil {
				return err
			}

		case code.OpIndex:
			index, err := vm.pop()
			if err != nil {
				return err
			}
			left, err := vm.pop()
			if err != nil {
				return err
			}

			err = vm.executeIndexExpression(left, index)
			if err != nil {
				return err
			}

		case code.OpGetField:
			member, err := vm.pop()
			if err != nil {
				return err
			}
			left, err := vm.pop()
			if err != nil {
				return err
			}

			err = vm.executeGetField(left, member)
			if err != nil {
				return err
			}

		case code.OpSetIndex:
			val, err := vm.pop()
			if err != nil {
				return err
			}
			index, err := vm.pop()
			if err != nil {
				return err
			}
			target, err := vm.pop()
			if err != nil {
				return err
			}
			err = vm.executeSetIndex(target, index, val)
			if err != nil {
				return err
			}

		case code.OpCall:
			numArgs, operandErr := vm.readU8(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			vm.currentFrame().ip += 1

			err := vm.executeCall(int(numArgs))
			if err != nil {
				return err
			}

		case code.OpReturnValue:
			returnValue, err := vm.pop()
			if err != nil {
				return err
			}

			frame, err := vm.popFrame()
			if err != nil {
				return err
			}
			if frame.isPure {
				vm.pureDepth--
			}
			if frame.basePointer > 0 {
				vm.truncateStack(frame.basePointer - 1)
			} else {
				vm.truncateStack(0)
			}

			if err := vm.push(returnValue); err != nil {
				return err
			}

		case code.OpReturn:
			frame, err := vm.popFrame()
			if err != nil {
				return err
			}
			if frame.isPure {
				vm.pureDepth--
			}
			if frame.basePointer > 0 {
				vm.truncateStack(frame.basePointer - 1)
			} else {
				vm.truncateStack(0)
			}

			if err := vm.push(Null); err != nil {
				return err
			}

		case code.OpSetLocal:
			localIndex, operandErr := vm.readU8(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			vm.currentFrame().ip += 1

			val, err := vm.pop()
			if err != nil {
				return err
			}
			frame := vm.currentFrame()
			addr := frame.basePointer + int(localIndex)
			if addr >= StackSize {
				return errInvalidBytecode("local index out of bounds")
			}
			// A boxed local holds a cell, so the assignment must go through it
			// for the closure to observe the write.
			if cell, ok := vm.stack[addr].(*object.CaptureCell); ok {
				cell.Value = val
			} else {
				vm.stack[addr] = val
			}

		case code.OpGetLocal:
			localIndex, operandErr := vm.readU8(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			vm.currentFrame().ip += 1

			frame := vm.currentFrame()
			addr := frame.basePointer + int(localIndex)
			if addr >= StackSize {
				return errInvalidBytecode("local index out of bounds")
			}

			// A boxed local holds a cell rather than a value; reading it means
			// reading through the cell.
			value := vm.stack[addr]
			if cell, ok := value.(*object.CaptureCell); ok {
				value = cell.Value
			}

			err := vm.push(value)
			if err != nil {
				return err
			}

		case code.OpBoxLocal:
			localIndex, operandErr := vm.readU8(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			vm.currentFrame().ip += 1

			frame := vm.currentFrame()
			addr := frame.basePointer + int(localIndex)
			if addr >= StackSize {
				return errInvalidBytecode("local index out of bounds")
			}

			// Box in place so the enclosing frame and every closure that
			// captures this local share one cell. Re-boxing an already boxed slot
			// would silently split the two, so the existing cell is kept.
			//
			// The cell is then pushed, because OpClosure consumes one stack value
			// per captured variable and pushClosure expects to find either a cell
			// or a bare value there.
			cell, alreadyBoxed := vm.stack[addr].(*object.CaptureCell)
			if !alreadyBoxed {
				cell = &object.CaptureCell{Value: vm.stack[addr]}
				vm.allocateObject(cell)
				vm.stack[addr] = cell
			}

			if err := vm.push(cell); err != nil {
				return err
			}

		case code.OpSetLocalFresh:
			localIndex, operandErr := vm.readU8(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			vm.currentFrame().ip += 1

			val, err := vm.pop()
			if err != nil {
				return err
			}
			frame := vm.currentFrame()
			addr := frame.basePointer + int(localIndex)
			if addr >= StackSize {
				return errInvalidBytecode("local index out of bounds")
			}
			// A `let` inside a loop body declares a fresh binding on every
			// pass (SEMANTICS.md §4.2 rule 2). If a closure captured the
			// previous pass's cell, writing through it would merge all
			// passes into one binding — so a held cell is replaced with a
			// new one, while a plain value stores exactly like OpSetLocal.
			if _, ok := vm.stack[addr].(*object.CaptureCell); ok {
				cell := &object.CaptureCell{Value: val}
				vm.allocateObject(cell)
				vm.stack[addr] = cell
			} else {
				vm.stack[addr] = val
			}

		case code.OpSetGlobalFresh:
			globalIndex, operandErr := vm.readU16(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			vm.currentFrame().ip += 2
			if int(globalIndex) >= len(vm.globals) {
				return errInvalidBytecode("global index out of range")
			}

			val, err := vm.pop()
			if err != nil {
				return err
			}
			mu := vm.ensureGlobalsMu()
			mu.Lock()
			if _, ok := vm.globals[globalIndex].(*object.CaptureCell); ok {
				cell := &object.CaptureCell{Value: val}
				vm.allocateObject(cell)
				vm.globals[globalIndex] = cell
			} else {
				vm.globals[globalIndex] = val
			}
			mu.Unlock()

		case code.OpBoxGlobal:
			globalIndex, operandErr := vm.readU16(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			vm.currentFrame().ip += 2
			if int(globalIndex) >= len(vm.globals) {
				return errInvalidBytecode("global index out of range")
			}

			mu := vm.ensureGlobalsMu()
			mu.Lock()
			// Mirror OpBoxLocal: box in place so the slot and every closure
			// created this pass share one cell. Keeping an existing cell is
			// what keeps same-pass siblings shared.
			cell, alreadyBoxed := vm.globals[globalIndex].(*object.CaptureCell)
			if !alreadyBoxed {
				cell = &object.CaptureCell{Value: vm.globals[globalIndex]}
				vm.allocateObject(cell)
				vm.globals[globalIndex] = cell
			}
			mu.Unlock()
			if err := vm.push(cell); err != nil {
				return err
			}

		case code.OpGetBuiltin:
			builtinIndex, operandErr := vm.readU8(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			vm.currentFrame().ip += 1

			names := compiler.GetBuiltinNamesSorted()
			if int(builtinIndex) >= len(names) {
				return errInvalidBytecode("builtin index %d out of range (%d builtins)", builtinIndex, len(names))
			}
			definition, _ := evaluator.GetBuiltin(names[builtinIndex])

			err := vm.push(definition)
			if err != nil {
				return err
			}

		case code.OpClosure:
			constIndex, operandErr := vm.readU16(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			numFreeByte, operandErr := vm.readU8At(ins, ip+3)
			if operandErr != nil {
				return operandErr
			}
			numFree := int(numFreeByte)
			vm.currentFrame().ip += 3

			if numFree < 0 || numFree > vm.sp {
				return errInvalidBytecode("closure captures %d values with only %d on the stack", numFree, vm.sp)
			}
			err := vm.pushClosure(int(constIndex), numFree)
			if err != nil {
				return err
			}

		case code.OpGetFree:
			freeIndex, operandErr := vm.readU8(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			vm.currentFrame().ip += 1

			freeCell, err := vm.freeCellAt(int(freeIndex))
			if err != nil {
				return err
			}
			if err := vm.push(freeCell.Value); err != nil {
				return err
			}

		case code.OpGetFreeCell:
			// Pushes the raw *CaptureCell pointer (not the unwrapped value).
			// Used only in the free-variable setup before OpClosure so that
			// the inner closure can share the same heap cell as the outer one.
			freeIndex, operandErr := vm.readU8(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			vm.currentFrame().ip += 1

			freeCell, err := vm.freeCellAt(int(freeIndex))
			if err != nil {
				return err
			}
			err = vm.push(freeCell) // push the cell itself
			if err != nil {
				return err
			}

		case code.OpSetFree:
			freeIndex, operandErr := vm.readU8(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			vm.currentFrame().ip += 1

			// Write the new value back into the shared mutable cell.
			// All closures sharing this cell will see the updated value.
			newVal, err := vm.pop()
			if err != nil {
				return err
			}
			freeCell, err := vm.freeCellAt(int(freeIndex))
			if err != nil {
				return err
			}
			freeCell.Value = newVal

		case code.OpCurrentClosure:
			currentClosure := vm.currentFrame().cl
			if currentClosure == nil {
				return errInvalidBytecode("current closure is nil outside a function")
			}
			err := vm.push(currentClosure)
			if err != nil {
				return err
			}

		case code.OpTask:
			rawClosure, err := vm.pop()
			if err != nil {
				return err
			}
			closure, ok := rawClosure.(*object.Closure)
			if !ok {
				return errTypeMismatch("task body is not a function")
			}
			// The task joins the structured tree under this VM's scope: a
			// faulted task cancels its siblings through the shared parent,
			// and cancelling the scope cascades to every task in it
			// (CONCURRENCY.md §3.1). The language-level future below is what
			// the program awaits; the scope owns the lifecycle.
			future := object.NewFuture()
			vm.allocateObject(future)
			scope := concurrency.NewTaskScope(vm.ensureTaskScope())
			vm.taskChildrenMu.Lock()
			vm.taskChildren = append(vm.taskChildren, scope)
			vm.taskChildrenMu.Unlock()
			if err := vm.push(future); err != nil {
				return err
			}
			go func() {
				defer func() {
					// A panic inside a task must not take down the process.
					// It becomes a faulted future, which CONCURRENCY.md §4
					// requires, and faults its siblings through the scope.
					if r := recover(); r != nil {
						fault := errTaskFaulted("task panicked: %v", r)
						scope.Fail(fault)
						future.Complete(nil, fault)
					}
				}()

				// Cooperative cancellation before starting: a task spawned
				// into an already-cancelled tree never runs.
				select {
				case <-scope.Done():
					scope.Cancel()
					cancelled := vmError(object.CategoryTaskCancelled, "task cancelled before it started")
					future.Complete(nil, cancelled)
					return
				default:
				}

				value, err := vm.runClosureShared(closure, scope)
				if err != nil {
					fault := asTaskFault(err)
					scope.Fail(fault)
					future.Complete(nil, fault)
					return
				}
				if value == nil {
					value = Null
				}
				future.Complete(value, nil)
			}()

		case code.OpAwait:
			rawFuture, err := vm.pop()
			if err != nil {
				return err
			}
			future, ok := rawFuture.(*object.Future)
			if !ok {
				return errTypeMismatch("cannot await a non-task value")
			}
			result, err := future.Await()
			if err != nil {
				// A typed fault travels unchanged: the category is the
				// cross-engine contract (ERROR_MODEL.md §4), and both
				// engines report TaskFaulted for a failed task. The fault
				// message already names the failure, so wrapping it would
				// only reword what the other engine says.
				if _, ok := object.CategoryOf(err); ok {
					return err
				}
				return vmErrorWrap(object.CategoryTaskFaulted, err, "task failed: %s", err)
			}
			if result == nil {
				result = Null
			}
			if err := vm.push(result); err != nil {
				return err
			}

		case code.OpToString:
			val, err := vm.pop()
			if err != nil {
				return err
			}
			if err := vm.push(&object.String{Value: val.Inspect()}); err != nil {
				return err
			}

		case code.OpStringConcat:
			right, errR := vm.pop()
			if errR != nil {
				return errR
			}
			left, errL := vm.pop()
			if errL != nil {
				return errL
			}
			leftStr, ok := left.(*object.String)
			if !ok {
				return errTypeMismatch("string concatenation on %s", typeName(left))
			}
			rightStr, ok := right.(*object.String)
			if !ok {
				return errTypeMismatch("string concatenation on %s", typeName(right))
			}
			if err := vm.push(&object.String{Value: leftStr.Value + rightStr.Value}); err != nil {
				return err
			}

		case code.OpNativeCall:
			nativeIndex, operandErr := vm.readU16(ins, ip)
			if operandErr != nil {
				return operandErr
			}
			numArgsByte, operandErr := vm.readU8At(ins, ip+3)
			if operandErr != nil {
				return operandErr
			}
			numArgs := int(numArgsByte)
			vm.currentFrame().ip += 3

			nameConst, err := vm.constantAt(int(nativeIndex))
			if err != nil {
				return err
			}
			nameStr, ok := nameConst.(*object.String)
			if !ok {
				return errInvalidBytecode("native call name is %s, want string", typeName(nameConst))
			}
			name := nameStr.Value
			if numArgs < 0 || numArgs > vm.sp {
				return errInvalidBytecode("native call with %d args and %d values on the stack", numArgs, vm.sp)
			}
			args := make([]object.Object, numArgs)
			for i := numArgs - 1; i >= 0; i-- {
				arg, err := vm.pop()
				if err != nil {
					return err
				}
				args[i] = arg
			}

			if vm.CapabilityChecker != nil {
				if err := vm.CapabilityChecker(name); err != nil {
					// Wrap rather than flatten: the category satisfies the
					// cross-engine contract while the cause keeps the operation
					// name and missing capabilities reachable via errors.As.
					return vmErrorWrap(object.CategoryCapabilityDenied, err, "%s", err)
				}
			}

			// A pure function never reaches a native operation: every
			// OpNativeCall names a capability-gated, therefore effectful,
			// operation, so reaching one with a pure frame on the stack is
			// a runtime E0202 (L6.3). It runs after the capability check so
			// that existing denial behaviour is unchanged; with no pure
			// frame on the stack the check is skipped entirely.
			if vm.pureDepth > 0 {
				return vmError(object.CategoryTypeError,
					"E0202: cannot call effectful operation %q inside a pure function", name)
			}

			// Dispatch: an embedder supplied handler takes precedence, then the
			// language's own builtin, and only a genuinely absent operation
			// yields null. Falling straight through to null would turn every
			// gated builtin into a silent no-op once the compiler started
			// emitting OpNativeCall for it.
			switch {
			case vm.NativeCallHandler != nil:
				res, err := vm.NativeCallHandler(name, args)
				if err != nil {
					return err
				}
				if res == nil {
					res = Null
				}
				if err := vm.push(res); err != nil {
					return err
				}
				if vmErr, failed := topError(res); failed {
					return vmErr
				}

			default:
				builtinFn, ok := evaluator.GetBuiltin(name)
				if !ok {
					return vmError(object.CategoryUnknownOperation, "native operation not implemented: %s", name)
				}
				result := builtinFn.Fn(args...)
				if result == nil {
					result = Null
				}
				if err := vm.push(result); err != nil {
					return err
				}
				if vmErr, failed := topError(result); failed {
					return vmErr
				}
			}

		default:
			return errInvalidBytecode("unknown opcode %d", int(op))
		}
	}

	return nil
}

func (vm *VM) executeBinaryOperation(op code.Opcode) error {
	right, err := vm.pop()
	if err != nil {
		return err
	}
	left, err := vm.pop()
	if err != nil {
		return err
	}

	if left == nil || right == nil {
		return errTypeMismatch("uninitialized variable or nil operand in binary operation")
	}

	leftType := left.Type()
	rightType := right.Type()

	switch {
	case leftType == object.INTEGER_OBJ && rightType == object.INTEGER_OBJ:
		return vm.executeBinaryIntegerOperation(op, left, right)
	case isNumeric(left) && isNumeric(right):
		return vm.executeBinaryFloatOperation(op, left, right)
	case leftType == object.STRING_OBJ && rightType == object.STRING_OBJ:
		return vm.executeBinaryStringOperation(op, left, right)
	case op == code.OpAdd && (leftType == object.STRING_OBJ || rightType == object.STRING_OBJ):
		// Mirrors the tree-walking evaluator: "+" with exactly one string
		// operand concatenates, coercing the other side with Inspect().
		return vm.push(&object.String{Value: left.Inspect() + right.Inspect()})
	default:
		return errTypeMismatch("unsupported types for binary operation: %s %s", leftType, rightType)
	}
}

func (vm *VM) executeBinaryFloatOperation(op code.Opcode, left, right object.Object) error {
	leftValue := numericValue(left)
	rightValue := numericValue(right)

	var result float64
	switch op {
	case code.OpAdd:
		result = leftValue + rightValue
	case code.OpSub:
		result = leftValue - rightValue
	case code.OpMul:
		result = leftValue * rightValue
	case code.OpDiv:
		result = leftValue / rightValue
	case code.OpMod:
		if rightValue == 0 {
			return errModuloByZero()
		}
		result = math.Mod(leftValue, rightValue)
	default:
		return errTypeMismatch("unknown float operator: %d", op)
	}
	return vm.push(&object.Float{Value: result})
}

func (vm *VM) executeBinaryIntegerOperation(op code.Opcode, left, right object.Object) error {
	leftValue := left.(*object.Integer).Value
	rightValue := right.(*object.Integer).Value

	var result int64

	switch op {
	case code.OpAdd:
		result = leftValue + rightValue
	case code.OpSub:
		result = leftValue - rightValue
	case code.OpMul:
		result = leftValue * rightValue
	case code.OpDiv:
		if rightValue == 0 {
			return errDivisionByZero()
		}
		result = leftValue / rightValue
	case code.OpMod:
		if rightValue == 0 {
			return errModuloByZero()
		}
		result = leftValue % rightValue
	default:
		return errTypeMismatch("unknown integer operator: %d", op)
	}

	return vm.push(&object.Integer{Value: result})
}

func (vm *VM) executeBinaryStringOperation(op code.Opcode, left, right object.Object) error {
	if op != code.OpAdd {
		return errTypeMismatch("unknown string operator: %d", op)
	}

	leftValue := left.(*object.String).Value
	rightValue := right.(*object.String).Value

	return vm.push(&object.String{Value: leftValue + rightValue})
}

func (vm *VM) executeComparison(op code.Opcode) error {
	right, err := vm.pop()
	if err != nil {
		return err
	}
	left, err := vm.pop()
	if err != nil {
		return err
	}

	if left == nil || right == nil {
		return errTypeMismatch("uninitialized variable or nil operand in comparison")
	}

	if left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ {
		return vm.executeIntegerComparison(op, left, right)
	}
	if isNumeric(left) && isNumeric(right) {
		return vm.executeFloatComparison(op, left, right)
	}
	if left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ {
		leftVal := left.(*object.String).Value
		rightVal := right.(*object.String).Value
		switch op {
		case code.OpEqual:
			return vm.push(nativeBoolToBooleanObject(leftVal == rightVal))
		case code.OpNotEqual:
			return vm.push(nativeBoolToBooleanObject(leftVal != rightVal))
		case code.OpGreaterThan:
			return vm.push(nativeBoolToBooleanObject(leftVal > rightVal))
		case code.OpGreaterThanEqual:
			return vm.push(nativeBoolToBooleanObject(leftVal >= rightVal))
		case code.OpLessThan:
			return vm.push(nativeBoolToBooleanObject(leftVal < rightVal))
		case code.OpLessThanEqual:
			return vm.push(nativeBoolToBooleanObject(leftVal <= rightVal))
		default:
			return errTypeMismatch("unknown operator: %d (%s %s)", op, left.Type(), right.Type())
		}
	}

	if left.Type() == object.STRING_OBJ || right.Type() == object.STRING_OBJ {
		// Mirrors the tree-walking evaluator: equality with exactly one string
		// operand compares the printed form of both sides. Relational
		// operators are still rejected below.
		leftVal := left.Inspect()
		rightVal := right.Inspect()
		switch op {
		case code.OpEqual:
			return vm.push(nativeBoolToBooleanObject(leftVal == rightVal))
		case code.OpNotEqual:
			return vm.push(nativeBoolToBooleanObject(leftVal != rightVal))
		}
	}

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(objectsEqual(right, left)))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(!objectsEqual(right, left)))
	default:
		return errTypeMismatch("unknown operator: %d (%s %s)", op, left.Type(), right.Type())
	}
}

// objectsEqual compares two values for `==` when neither is a number, a string,
// or a collection with its own comparison.
//
// It matches on type and value rather than pointer identity. Comparing the Go
// interfaces directly would report `isOk(x) == false` as unequal, because the
// boolean a builtin returned is a different pointer from this package's `False`.
// Booleans and null are the values with no `Inspect` distinction worth relying
// on, so they are matched structurally; everything else falls back to the
// printed form, which is what the branch above already does.
func objectsEqual(left, right object.Object) bool {
	if left.Type() != right.Type() {
		return false
	}

	switch left := left.(type) {
	case *object.Boolean:
		rightBool, ok := right.(*object.Boolean)
		return ok && left.Value == rightBool.Value
	case *object.Null:
		return true
	default:
		return left.Inspect() == right.Inspect()
	}
}

func (vm *VM) executeFloatComparison(op code.Opcode, left, right object.Object) error {
	leftValue := numericValue(left)
	rightValue := numericValue(right)
	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue == rightValue))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue != rightValue))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBooleanObject(leftValue > rightValue))
	case code.OpGreaterThanEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue >= rightValue))
	case code.OpLessThan:
		return vm.push(nativeBoolToBooleanObject(leftValue < rightValue))
	case code.OpLessThanEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue <= rightValue))
	default:
		return fmt.Errorf("unknown operator: %d", op)
	}
}

func (vm *VM) executeIntegerComparison(op code.Opcode, left, right object.Object) error {
	leftValue := left.(*object.Integer).Value
	rightValue := right.(*object.Integer).Value

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(rightValue == leftValue))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(rightValue != leftValue))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBooleanObject(leftValue > rightValue))
	case code.OpGreaterThanEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue >= rightValue))
	case code.OpLessThan:
		return vm.push(nativeBoolToBooleanObject(leftValue < rightValue))
	case code.OpLessThanEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue <= rightValue))
	default:
		return fmt.Errorf("unknown operator: %d", op)
	}
}

func (vm *VM) executeBangOperator() error {
	operand, err := vm.pop()
	if err != nil {
		return err
	}

	switch value := operand.(type) {
	case *object.Boolean:
		if value.Value {
			return vm.push(False)
		}
		return vm.push(True)
	case *object.Null:
		return vm.push(True)
	case *object.Integer:
		if value.Value == 0 {
			return vm.push(True)
		}
		return vm.push(False)
	default:
		if operand == nil {
			return vm.push(True)
		}
		return vm.push(False)
	}
}

func (vm *VM) executeMinusOperator() error {
	operand, err := vm.pop()
	if err != nil {
		return err
	}
	if operand == nil {
		return errTypeMismatch("negation of nil")
	}

	switch operand := operand.(type) {
	case *object.Integer:
		return vm.push(&object.Integer{Value: -operand.Value})
	case *object.Float:
		return vm.push(&object.Float{Value: -operand.Value})
	default:
		return errTypeMismatch("unsupported type for negation: %s", operand.Type())
	}
}

func isNumeric(value object.Object) bool {
	return value.Type() == object.INTEGER_OBJ || value.Type() == object.FLOAT_OBJ
}

func numericValue(value object.Object) float64 {
	if integer, ok := value.(*object.Integer); ok {
		return float64(integer.Value)
	}
	return value.(*object.Float).Value
}

func (vm *VM) executeIndexExpression(left, index object.Object) error {
	if left == nil || index == nil {
		return errTypeMismatch("indexing with nil operand")
	}
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return vm.executeArrayIndex(left, index)
	case left.Type() == object.STRING_OBJ && index.Type() == object.INTEGER_OBJ:
		return vm.executeStringIndex(left, index)
	case left.Type() == object.HASH_OBJ:
		return vm.executeHashIndex(left, index)
	default:
		return errTypeMismatch("index operator not supported on %s", left.Type())
	}
}

// executeGetField reads a named member, and reports a member that is not there.
//
// # Why a miss is an error here but not in executeHashIndex
//
// `h["k"]` and `h.k` look alike and are not the same question. Indexing asks
// "what is at this key", and a key that is not there has the answer null — that
// is what a lookup miss means. Dot access asks for a member by name, and a
// member that does not exist is a typo, a refactor that was not finished, or a
// call to a method that needs parentheses. Answering null for any of those
// moves the failure to whatever line happens to use the value, which is nowhere
// near the mistake.
//
// This is why they compile to different instructions. They used to compile to
// the same one, and the VM therefore answered null for a dot access the
// evaluator refused — the two engines disagreed about the same program.
func (vm *VM) executeGetField(left, member object.Object) error {
	if left == nil || member == nil {
		return errTypeMismatch("member access with nil operand")
	}
	name, ok := member.(*object.String)
	if !ok {
		return errTypeMismatch("a field name must be a string, got %s", member.Type())
	}

	hashObject, isHash := left.(*object.Hash)
	if !isHash {
		// The wording matches the evaluator's. Both engines refuse a member on a
		// primitive or an array, but they were describing it in different terms —
		// "property access .length not supported on type STRING" against "index
		// operator not supported on STRING" — so a reader comparing the two
		// outputs had to work out whether they described one problem or two.
		//
		// The second message is the one that was wrong: this is a dot access, not
		// an index, and saying otherwise sent the reader looking at brackets that
		// are not in their program.
		return errTypeMismatch("property access .%s not supported on type %s", name.Value, left.Type())
	}

	pair, present := hashObject.Pairs[name.HashKey()]
	if !present {
		// The wording matches the evaluator's on purpose. Two engines reporting
		// the same mistake in different words is a divergence a conformance suite
		// cannot express as agreement, and a reader comparing the two outputs
		// would have to work out whether they describe one problem or two.
		//
		// The category is extra, not a difference: the evaluator raises an
		// uncategorised error for this and the VM always categorises. What the
		// suite compares is the category, and "the operation does not exist" is
		// what this is.
		return errUnknownOperation("unknown method %s on object", name.Value)
	}
	return vm.push(pair.Value)
}

func (vm *VM) executeStringIndex(str, index object.Object) error {
	strVal := str.(*object.String).Value
	i := index.(*object.Integer).Value
	runes := []rune(strVal)
	max := int64(len(runes) - 1)

	if i < 0 || i > max {
		return vm.push(Null)
	}

	return vm.push(&object.String{Value: string(runes[i])})
}

func (vm *VM) executeSetIndex(target, index, val object.Object) error {
	if target == nil || index == nil {
		return errTypeMismatch("index-assignment with nil operand")
	}
	switch t := target.(type) {
	case *object.Array:
		idxObj, ok := index.(*object.Integer)
		if !ok {
			return errTypeMismatch("array index must be an integer, got %s", index.Type())
		}
		idx := idxObj.Value
		if idx < 0 || idx >= int64(len(t.Elements)) {
			return errIndexOutOfBounds("index %d out of bounds", idx)
		}
		t.Elements[idx] = val
		return nil
	case *object.Hash:
		if _, ok := index.(object.Hashable); !ok {
			return errTypeMismatch("unusable as hash key: %s", index.Type())
		}
		// Through `SetPair`, never by writing `Pairs` directly — see the note on
		// `Hash.Pairs`. Writing the lookup table by hand leaves the written-order
		// view stale, and the symptom is a value that prints in an order this engine
		// and the others disagree about.
		t.SetPair(index, val)
		return nil
	default:
		return errTypeMismatch("index-assignment not supported on: %s", target.Type())
	}
}

func (vm *VM) executeArrayIndex(array, index object.Object) error {
	arrayObject := array.(*object.Array)
	i := index.(*object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)

	if i < 0 || i > max {
		return vm.push(Null)
	}

	return vm.push(arrayObject.Elements[i])
}

func (vm *VM) executeHashIndex(hash, index object.Object) error {
	hashObject := hash.(*object.Hash)

	key, ok := index.(object.Hashable)
	if !ok {
		return errTypeMismatch("unusable as hash key: %s", index.Type())
	}

	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return vm.push(Null)
	}

	return vm.push(pair.Value)
}

func (vm *VM) buildArray(startIndex, endIndex int) object.Object {
	elements := make([]object.Object, endIndex-startIndex)

	for i := startIndex; i < endIndex; i++ {
		elements[i-startIndex] = vm.stack[i]
	}

	return &object.Array{Elements: elements}
}

// buildHash assembles a hash from key/value pairs pushed onto the stack.
//
// The pairs arrive in source order and are recorded through SetPair, so the hash
// keeps that order. Building the map directly would leave the order empty and
// fall back to sorted keys, which made a struct literal print its fields in
// alphabetical order under `--vm` while the evaluator printed them as written.
func (vm *VM) buildHash(startIndex, endIndex int) (object.Object, error) {
	pairs := (endIndex - startIndex) / 2
	hash := object.NewHash(pairs)

	for i := startIndex; i < endIndex; i += 2 {
		key := vm.stack[i]
		value := vm.stack[i+1]

		if _, ok := key.(object.Hashable); !ok {
			return nil, errTypeMismatch("unusable as hash key: %s", key.Type())
		}

		hash.SetPair(key, value)
	}

	return hash, nil
}

func (vm *VM) executeCall(numArgs int) error {
	if numArgs < 0 || vm.sp-1-numArgs < 0 {
		return errInvalidBytecode("call with %d args and %d values on the stack", numArgs, vm.sp)
	}
	callee := vm.stack[vm.sp-1-numArgs]
	if callee == nil {
		return errInvalidBytecode("call target is nil")
	}
	switch callee := callee.(type) {
	case *object.Closure:
		return vm.callClosure(callee, numArgs)
	case *object.Builtin:
		return vm.callBuiltin(callee, numArgs)
	default:
		return errTypeMismatch("calling non-function and non-built-in: %s", callee.Type())
	}
}

func (vm *VM) callClosure(cl *object.Closure, numArgs int) error {
	if numArgs != cl.Fn.NumParameters {
		// Arity is checked statically (E0105), so reaching here means hand-built
		// bytecode or a direct API call. TypeError keeps the closed category
		// set closed; there is deliberately no ArityMismatch runtime category.
		return errTypeMismatch("wrong number of arguments: want=%d, got=%d",
			cl.Fn.NumParameters, numArgs)
	}

	// A non-pure function called while a pure one is on the stack
	// contradicts the pure declaration: the call may perform an effect the
	// declaration promised it would not (runtime E0202, L6.3).
	if vm.pureDepth > 0 && !cl.Fn.IsPure {
		return vmError(object.CategoryTypeError,
			"E0202: cannot call side-effecting function inside a pure function")
	}

	// Frame depth is checked before stack capacity so that unbounded recursion
	// reports FrameOverflow, matching the evaluator's call depth bound. See the
	// StackSize comment for why the ordering matters.
	if vm.framesIndex >= MaxFrames {
		return errFrameOverflow("maximum call depth %d exceeded", MaxFrames)
	}

	frame := NewFrame(cl, vm.sp-numArgs)
	frame.isPure = cl.Fn.IsPure
	if err := vm.pushFrame(frame); err != nil {
		return err
	}
	if frame.isPure {
		vm.pureDepth++
	}

	newSp := frame.basePointer + cl.Fn.NumLocals
	if newSp > StackSize {
		// A single frame needing more slots than the whole stack is a genuine
		// stack exhaustion, not recursion depth, so it keeps StackOverflow.
		// The pushed frame is unwound first so the pure depth stays exact.
		if popped, popErr := vm.popFrame(); popErr == nil && popped.isPure {
			vm.pureDepth--
		}
		return errStackOverflow("call frame of %d slots exceeds the %d slot stack",
			cl.Fn.NumLocals, StackSize)
	}
	vm.sp = newSp

	return nil
}

func (vm *VM) callBuiltin(builtin *object.Builtin, numArgs int) error {
	args := vm.stack[vm.sp-numArgs : vm.sp]

	// The capability and effect gates apply to calls through a variable
	// exactly as they do to direct ones: without this a denied operation
	// would only need one alias to escape the sandbox, and a pure function
	// only one indirection to hide its effect.
	if name, ok := evaluator.BuiltinName(builtin); ok {
		if err := vm.checkBuiltinCall(name); err != nil {
			vm.truncateStack(vm.sp - numArgs - 1)
			return err
		}
		if name == "withTimeout" {
			return vm.callWithTimeout(args, numArgs)
		}
	}

	result := builtin.Fn(args...)

	vm.truncateStack(vm.sp - numArgs - 1)

	if result != nil {
		vm.push(result)
	} else {
		vm.push(Null)
	}

	// A builtin signals failure by returning an error object rather than by
	// raising anything, and the evaluator stops there because every one of its
	// Eval calls propagates the value. Pushing the object and carrying on made
	// `assert(false, "x")` a no-op under `--vm`: the program reached the next
	// statement and the assertion never happened. ERROR_MODEL.md §4 requires
	// both engines to fail on the same programs.
	if vmErr, failed := topError(result); failed {
		return vmErr
	}

	return nil
}

// checkBuiltinCall enforces the pure context (L6.3) and the capability
// sandbox (L6.4) on a builtin call reached through OpCall, which is the
// indirect form the OpNativeCall instrumentation cannot see. It returns nil
// when the call may run.
func (vm *VM) checkBuiltinCall(name string) error {
	if vm.pureDepth > 0 && compbuiltin.IsEffectful(name) {
		return vmError(object.CategoryTypeError,
			"E0202: cannot call effectful builtin %q inside a pure function", name)
	}
	if vm.CapabilityChecker != nil {
		if class, known := compbuiltin.Classify(name); known && class.Gated {
			if err := vm.CapabilityChecker(name); err != nil {
				return vmErrorWrap(object.CategoryCapabilityDenied, err, "%s", err)
			}
		}
	}
	return nil
}

// runClosureShared runs a closure on a child VM that shares this VM's global
// store, constants and capability hooks (L6.1/L6.2).
//
// The globals slice and its mutex are shared, never copied, so a write the
// task makes is visible to the parent after the await. The child inherits
// the capability checker: without that a task could reach a host operation
// its parent was never authorised for, which would make the sandbox
// trivially bypassable by wrapping the work in a task. The child
// deliberately does not inherit GC tracking or debugging: the parent owns
// the heap, and a second collector over a shared store would reclaim objects
// the parent still references.
func (vm *VM) runClosureShared(closure *object.Closure, scope *concurrency.Task) (object.Object, error) {
	child := &VM{
		constants:         vm.constants,
		stack:             make([]object.Object, StackSize),
		globals:           vm.globals,
		globalsMu:         vm.ensureGlobalsMu(),
		globalNames:       vm.globalNames,
		frames:            make([]*Frame, MaxFrames),
		taskScope:         scope,
		cancellable:       true,
		NativeCallHandler: vm.NativeCallHandler,
		CapabilityChecker: vm.CapabilityChecker,
		// A task body allocates into the same managed heap: without this a
		// program under collection would have its task allocations invisible
		// to the collector while the parent's are tracked.
		Heap:       vm.Heap,
		gcTracking: vm.gcTracking,
	}
	if err := child.push(closure); err != nil {
		return nil, err
	}
	if err := child.callClosure(closure, 0); err != nil {
		return nil, err
	}
	if err := child.Run(); err != nil {
		return nil, err
	}
	return child.StackTop(), nil
}

// callWithTimeout runs a zero-argument closure under a millisecond deadline
// (CONCURRENCY.md §3.2, L6.1). The closure runs on a child VM with the
// shared global store; an expired deadline reports TaskCancelled. The call
// itself is effectful, so it is refused in a pure context like any other
// gated operation — the check in checkBuiltinCall already ran before this.
func (vm *VM) callWithTimeout(args []object.Object, numArgs int) error {
	var result object.Object
	switch {
	case len(args) != 2:
		result = object.RuntimeErrorf(object.CategoryTypeError,
			"wrong number of arguments to `withTimeout`. got=%d, want=2", len(args))
	case !isInteger(args[0]):
		result = object.RuntimeErrorf(object.CategoryTypeError,
			"first argument to `withTimeout` must be INTEGER milliseconds, got %s", args[0].Type())
	default:
		closure, ok := args[1].(*object.Closure)
		if !ok {
			result = object.RuntimeErrorf(object.CategoryTypeError,
				"second argument to `withTimeout` must be a function, got %s", args[1].Type())
		} else {
			result = vm.runWithTimeout(args[0].(*object.Integer).Value, closure)
		}
	}

	vm.truncateStack(vm.sp - numArgs - 1)

	if result != nil {
		vm.push(result)
	} else {
		vm.push(Null)
	}

	if vmErr, failed := topError(result); failed {
		return vmErr
	}

	return nil
}

// timeoutOutcome is the result of a runWithTimeout goroutine: the value the
// closure produced, or the error it failed with.
type timeoutOutcome struct {
	value   object.Object
	err     error
	globals []object.Object
}

// runWithTimeout executes the closure off-goroutine and waits up to the
// deadline for it. There is no preemption: an expired deadline abandons the
// wait, not the work, and a panic in the work faults the result rather than
// the process. When the deadline expires the task's scope is cancelled so the
// VM stops polling instructions and the work cannot keep running with access
// to shared state. The goroutine is not awaited: a task blocked inside a
// builtin (for example `receive(Channel(0))`) may never observe cancellation,
// and waiting would deadlock this call. The buffered channel lets the
// goroutine send its outcome when it eventually returns.
func (vm *VM) runWithTimeout(timeoutMs int64, closure *object.Closure) object.Object {
	done := make(chan timeoutOutcome, 1)
	scope := concurrency.NewTaskScope(vm.ensureTaskScope())
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- timeoutOutcome{value: nil, err: errTaskFaulted("task panicked: %v", r)}
			}
		}()
		value, err, globals := vm.runClosureTransactional(closure, scope)
		done <- timeoutOutcome{value, err, globals}
	}()

	select {
	case out := <-done:
		if out.err == nil && out.globals != nil {
			mu := vm.ensureGlobalsMu()
			mu.Lock()
			copy(vm.globals, out.globals)
			mu.Unlock()
		}
		return vm.finishTimeout(out)
	case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
		scope.Cancel()
		return object.RuntimeErrorf(object.CategoryTaskCancelled,
			"withTimeout: task timeout exceeded after %dms", timeoutMs)
	}
}

// runClosureTransactional gives a timed closure a private global snapshot.
// Effects are committed only when the closure finishes before its deadline;
// a late or cancelled worker therefore cannot mutate the parent VM.
func (vm *VM) runClosureTransactional(closure *object.Closure, scope *concurrency.Task) (object.Object, error, []object.Object) {
	mu := vm.ensureGlobalsMu()
	mu.Lock()
	globals := append([]object.Object(nil), vm.globals...)
	mu.Unlock()
	child := &VM{
		constants: vm.constants, stack: make([]object.Object, StackSize), globals: globals,
		globalsMu: &sync.Mutex{}, globalNames: vm.globalNames, frames: make([]*Frame, MaxFrames),
		taskScope: scope, cancellable: true, NativeCallHandler: vm.NativeCallHandler,
		CapabilityChecker: vm.CapabilityChecker,
	}
	if err := child.push(closure); err != nil {
		return nil, err, nil
	}
	if err := child.callClosure(closure, 0); err != nil {
		return nil, err, nil
	}
	if err := child.Run(); err != nil {
		return nil, err, nil
	}
	return child.StackTop(), nil, child.globals
}

// finishTimeout converts a runWithTimeout goroutine result into the value or
// error the caller sees. A cancelled task reports CategoryTaskCancelled.
func (vm *VM) finishTimeout(out timeoutOutcome) object.Object {
	if out.err != nil {
		var runtimeErr *object.RuntimeError
		if errors.As(out.err, &runtimeErr) {
			return runtimeErr
		}
		return object.RuntimeErrorf(object.CategoryTaskFaulted, "task failed: %s", out.err.Error())
	}
	if out.value == nil {
		return Null
	}
	return out.value
}

// asTaskFault converts a task body's failure into the fault the awaiting
// program observes (L6.5). Every failure inside a task becomes a TaskFaulted
// fault naming its cause; an error that already is one travels unchanged,
// and cancellation travels as TaskCancelled rather than as a fault.
func asTaskFault(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, concurrency.ErrTaskCancelled) {
		return vmErrorWrap(object.CategoryTaskCancelled, err, "task cancelled")
	}
	if category, ok := object.CategoryOf(err); ok {
		if category == object.CategoryTaskFaulted || category == object.CategoryTaskCancelled {
			return err
		}
	}
	var runtimeErr *object.RuntimeError
	if errors.As(err, &runtimeErr) {
		return vmErrorWrap(object.CategoryTaskFaulted, err, "task failed: %s", runtimeErr.Message)
	}
	return vmErrorWrap(object.CategoryTaskFaulted, err, "task failed: %s", err.Error())
}

// isInteger reports whether the value is an integer object.
func isInteger(obj object.Object) bool {
	_, ok := obj.(*object.Integer)
	return ok
}

// topError reports whether an object's value is a runtime error, and if so
// returns it as a Go error the VM can stop with.
//
// A categorised RuntimeError keeps its category, which is what ERROR_MODEL.md §4
// requires both engines to agree on. A bare Error has none, so it is reported as
// the plain message it is rather than being given a category here — inventing one
// in the VM would make the two engines print different text for the same fault.
func topError(obj object.Object) (error, bool) {
	switch value := obj.(type) {
	case *object.RuntimeError:
		return value, true
	case *object.Error:
		// Inspect() is what the evaluator's front end prints for this object, so
		// reusing it is what makes the two engines report the same text.
		return uncategorisedError{message: value.Inspect()}, true
	default:
		return nil, false
	}
}

// uncategorisedError carries a message with no category, for the errors that
// never had one to begin with.
type uncategorisedError struct {
	message string
}

func (e uncategorisedError) Error() string { return e.message }

func (vm *VM) pushClosure(constIndex, numFree int) error {
	constant, err := vm.constantAt(constIndex)
	if err != nil {
		return err
	}
	function, ok := constant.(*object.CompiledFunction)
	if !ok {
		return errInvalidBytecode("not a function: %+v", constant)
	}

	// Wrap each captured value in a new CaptureCell (or re-use existing cell
	// if the captured variable is itself a free variable in the current closure).
	// This way, all closures that capture the same variable share a pointer to
	// the same heap-allocated cell, enabling mutable by-reference semantics.
	free := make([]*object.CaptureCell, numFree)
	for i := 0; i < numFree; i++ {
		raw := vm.stack[vm.sp-numFree+i]
		// If the raw stack value is already a CaptureCell (because it was
		// loaded from the outer closure's Free slice via OpGetFree and we
		// stored the cell itself — see compiler emit path), share it directly.
		if cell, ok := raw.(*object.CaptureCell); ok {
			free[i] = cell
		} else {
			free[i] = &object.CaptureCell{Value: raw}
			vm.allocateObject(free[i])
		}
	}
	vm.truncateStack(vm.sp - numFree)

	closure := &object.Closure{Fn: function, Free: free}
	vm.allocateObject(closure)
	return vm.push(closure)
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return True
	}
	return False
}

// isTruthy reports whether a value counts as true.
//
// The comparison is by type and value, never by pointer identity. A boolean
// produced by a Go builtin crosses the engine boundary as an ordinary
// *object.Boolean that is not this package's `True` or `False` singleton, so a
// `switch obj { case False: ... }` would fall through to the default branch and
// call every builtin result truthy. That made `if (isOk(x)) { ... } else { ... }`
// take the wrong branch under `--vm` while the evaluator took the right one.
func isTruthy(obj object.Object) bool {
	switch value := obj.(type) {
	case nil:
		return false
	case *object.Boolean:
		return value.Value
	case *object.Null:
		return false
	case *object.Integer:
		return value.Value != 0
	default:
		return true
	}
}

func Disassemble(ins code.Instructions) string {
	return ins.String()
}

// DisassembleWithConstants renders instructions alongside the constant pool,
// resolving OpConstant operands to their values.
//
// The plain Disassemble form prints operand numbers, which is compact but
// unreadable for a program whose behaviour depends on which operation a
// OpNativeCall names. Debugging capability refusals in particular needs to see
// the operation name in the listing.
func DisassembleWithConstants(ins code.Instructions, constants []object.Object) string {
	var out strings.Builder

	i := 0
	for i < len(ins) {
		def, err := code.Lookup(ins[i])
		if err != nil {
			fmt.Fprintf(&out, "ERROR: %s\n", err)
			i++
			continue
		}

		need := 0
		for _, w := range def.OperandWidths {
			need += w
		}
		if len(ins)-(i+1) < need {
			fmt.Fprintf(&out, "ERROR: %s at %d is truncated: need %d operand bytes, have %d\n",
				def.Name, i, need, len(ins)-(i+1))
			break
		}
		operands, read := code.ReadOperands(def, ins[i+1:])

		operandText := make([]string, len(operands))
		for idx, operand := range operands {
			text := strconv.Itoa(operand)
			if referencesConstant(def.Name, operandWidthAt(def, idx)) && operand >= 0 && operand < len(constants) {
				text += " (" + constants[operand].Inspect() + ")"
			}
			operandText[idx] = text
		}

		line := def.Name
		if len(operandText) > 0 {
			line += " " + strings.Join(operandText, " ")
		}
		fmt.Fprintf(&out, "%04d %s\n", i, line)

		i += 1 + read
	}

	return out.String()
}

// operandWidthAt returns the byte width of the operand at position idx.
func operandWidthAt(def *code.Definition, idx int) int {
	if idx < 0 || idx >= len(def.OperandWidths) {
		return 0
	}
	return def.OperandWidths[idx]
}

// referencesConstant reports whether an operand is a constant pool index.
//
// Two operand widths appear in the bytecode: one byte for locals, builtins and
// call arities, and two bytes for constants, globals and jumps. Globals and
// jumps are also two bytes wide, so width alone is not enough to tell them
// apart; the opcode decides.
func referencesConstant(opName string, width int) bool {
	if width != 2 {
		return false
	}
	switch opName {
	case "OpConstant", "OpNativeCall", "OpClosure":
		return true
	}
	return false
}
