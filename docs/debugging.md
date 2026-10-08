# Debugging Nilang

The NABC virtual machine has first-class debug support: the compiler emits a
source-position table and the VM can break, step and be inspected.

## What the compiler emits

`compiler/compiler` records, for every AST node, an entry mapping the current
bytecode offset to `file:line:column` (see `object.SourcePos`). Function
literals additionally carry their **name**, **source file**, **position table**
and **local slot names** (`object.CompiledFunction`). The top-level bytecode
also carries `Positions`, `SourceFile` and `GlobalNames`.

## What the VM exposes

`compiler/vm/debug.go`:

| API | Purpose |
|---|---|
| `vm.NewDebugState()` | create breakpoint/step state |
| `ds.SetBreakpoints(file, []int)` | set breakpoints by source line |
| `ds.Step()` / `ds.Continue()` / `ds.Pause()` | stepping control |
| `vm.Run()` | returns `vm.ErrPaused` at a stop |
| `vm.Location()` | current `file:line:column`, function, ip, frame |
| `vm.CallStack()` | frames, innermost first |
| `vm.Locals()` / `vm.Globals()` / `vm.FreeVars()` | variable inspection |
| `vm.EvaluateName(name)` | resolve a variable by name |
| `vm.Step()` / `vm.Continue()` | convenience wrappers that run |

Example:

```go
comp := compiler.New()
comp.SourceFile = "main.nil"
_ = comp.Compile(program)

machine := vm.New(comp.Bytecode())
ds := vm.NewDebugState()
ds.SetBreakpoints("main.nil", []int{3})
machine.Debug = ds

for machine.Run() == vm.ErrPaused {
    loc := machine.Location()
    fmt.Printf("paused at %s:%d in %s\n", loc.File, loc.Line, loc.Function)
    fmt.Println("locals:", machine.Locals())
    ds.Step()
}
```

## Debug Adapter Protocol

`nil dap <file.nil>` starts a standard DAP server over stdio, backed by the VM
hooks above. Editors (VS Code, Neovim, …) and the NilOS Studio emulator speak
DAP to it.

```powershell
nil dap examples/hello.nil
```

The server supports `initialize`, `launch`/`attach`, `setBreakpoints`,
`configurationDone`, `threads`, `stackTrace`, `scopes`, `variables`,
`continue`, `next`, `stepIn`, `stepOut`, `pause`, `evaluate` and `disconnect`.

## Tests

- `compiler/vm/debug_test.go` — position table, breakpoints, stepping, locals
  and function frames.
- `pkg/dap/dap_test.go` — full DAP flow against the real VM.
