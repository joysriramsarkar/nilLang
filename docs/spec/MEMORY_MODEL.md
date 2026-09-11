# NilLang Memory Model Specification
**Version:** 1.0.0-draft  
**Status:** Draft Normative Specification
**Conformance Level:** Nilang 0.1 GC design contract

---

## 0. Implementation Boundary

The repository contains a tracing mark-and-sweep collector with cycle tests
(`runtime/gc`). It is a standalone package today: neither the tree-walking
evaluator nor the stack VM routes its allocations through `Track`, so no
execution path is currently collected by it and the live root sets below are
not scanned at runtime.
Integration of every VM value, closure, task, component, and native handle into
one managed heap is not yet a Nilang 0.1 guarantee. Root kinds below are the
required integration checklist; a root is conforming only when backed by a GC
test.

## 1. Managed Execution & Memory Safety Guarantee

NilLang provides strict memory safety for all user-level code:
- **No Uninitialized Memory**: Every variable and struct field is guaranteed to be initialized before read access.
- **No Use-After-Free**: Object lifetimes are managed automatically; dangling pointers cannot be created in safe code.
- **No Buffer Overruns**: Array access is bounds-checked at runtime.

---

## 2. Tracing Garbage Collection Architecture

NilLang utilizes an exact tracing garbage collector for managing dynamic heap objects (arrays, hashes, closures, strings, components, and tasks).

### 2.1 Root Set
The garbage collector identifies live objects by traversing all references originating from the root set:
1. **VM Stacks**: All active values in the operand stack (`vm.stack[0:vm.sp]`).
2. **Call Frame Closures**: The active closure contexts and local variables in each frame (`vm.frames[0:vm.framesIndex]`).
3. **Global Symbol Registry**: All allocated entries in the global symbol array (`vm.globals`).
4. **Task & Concurrency Scheduler**: Active asynchronous tasks, pending futures, and queued microtasks.
5. **Component Trees**: Active reactive UI component state nodes in the Alap runtime.
6. **Native Handle Registry**: External host references explicitly anchored across the FFI boundary.

### 2.2 Heap Objects
Objects allocated on the managed heap:
- `Array`: Contiguous, dynamically resizable slice of object references.
- `Hash`: Hash map with open-addressing or chained buckets.
- `Closure`: Function bytecode pointer paired with an array of captured environment cells.
- `String`: Immutable UTF-8 byte buffer.

---

## 3. Native Interoperability & FFI Handle Retention

The future native-handle boundary must obey these rules when Nilang objects
cross into host code (Go runtime, C bridge, or Rust FFI):
1. **Explicit Retention (`RetainHandle`)**: Prevents the GC from reclaiming the target object while held by external foreign code.
2. **Explicit Release (`ReleaseHandle`)**: Unregisters the foreign root, making the object eligible for future garbage collection cycles.
3. Unmanaged pointer manipulation is strictly sequestered inside `unsafe` blocks.