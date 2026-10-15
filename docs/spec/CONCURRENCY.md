# NilLang Concurrency Model Specification
**Version:** 1.0.0-draft  
**Status:** Authoritative Normative Specification  
**Conformance Level:** Nilang 0.1 Core Freeze

---

## 1. Overview & Execution Model

NilLang concurrency is built upon lightweight asynchronous tasks and structured concurrency scopes. It eliminates manual thread management and data race hazards by ensuring deterministic lifecycle ownership.

### Core Constructs:
- `task { ... }`: Spawns an asynchronous concurrent computation returning a `Future<T>`.
- `await future`: Suspends the current execution frame until `future` resolves, yields its value, or propagates its failure.

---

## 2. Task Lifecycle & Terminal States

A task progresses through well-defined lifecycle states:

```text
               ┌──────────┐
               │ Pending  │
               └────┬─────┘
                    │ schedule
                    ▼
               ┌──────────┐
               │ Running  │
               └────┬─────┘
         ┌──────────┼──────────┐
  resolve│     error│    cancel│
         ▼          ▼          ▼
   ┌───────────┐┌─────────┐┌───────────┐
   │ Fulfilled ││ Faulted ││ Cancelled │
   └───────────┘└─────────┘└───────────┘
```

1. **Fulfilled**: Computation completed successfully; value is available for `await`.
2. **Faulted**: Computation encountered an unhandled error or panic; error propagates to `await`.
3. **Cancelled**: Computation aborted before or during execution via cooperative cancellation.

**Single-Resolution Invariant**: A task's terminal state is immutable and observed by `await` exactly once. Observing it more than once yields the same terminal state; it never consumes the completion.

**Cause preservation**: a faulted task carries the error that faulted it, and
`await` reports it. This holds for a task whose body fails directly. For a task
that fails *because a child task failed*, the parent currently reports
`TaskCancelled` rather than the child's cause — see §4.2.

---

## 3. Structured Concurrency Scope

Tasks must not outlive the scope that created them. All concurrent tasks belong to a hierarchical parent scope:

```text
Parent Task Scope
├── Child Task 1 (fetching profile)
├── Child Task 2 (fetching orders)
└── Child Task 3 (fetching preferences)
```

### 3.1 Cancellation Trees & Cascading
1. If a **parent task is cancelled**, all unfinished child tasks are automatically signalled for cancellation.
2. If any **child task fails with an unhandled exception**, sibling child tasks are cancelled, and the parent task fails with the aggregated failure.
3. A scope does not exit until all child tasks have reached a terminal state (`Fulfilled`, `Faulted`, or `Cancelled`).

### 3.2 Timeouts
```nil
let result = withTimeout(5000, fn() {
    return await api.fetchData();
});
```
If the deadline expires before completion, the enclosed task tree is cleanly cancelled with `TaskCancelledError`.

---

## 4. Scheduler Isolation & Panic Safety

1. **Fault Isolation**: A panic or runtime exception occurring inside a task does not crash the host process, and does not reach tasks belonging to another evaluation. It **does** cancel sibling tasks in the same evaluation — that is §3.1.2, and the two clauses are about different boundaries.
2. The runtime scheduler intercepts the exception, wraps it in a typed `TaskFaultError`, and transitions the task to the `Faulted` state.

### 4.1 What "isolation" means, precisely

Earlier drafts of this section said a fault "does not crash … sibling tasks",
while §3.1.2 says a child that faults cancels its siblings. Both are true and
they contradict each other in wording, which made the clause untestable: an
implementation that cancels siblings passed §3.1.2 and failed §4.1, depending on
whether the sibling had already finished.

The boundary is **the cancellation tree, not the task**:

| Beyond the boundary | Inside it |
|---|---|
| Another evaluation's tasks are unaffected | Siblings are cancelled (§3.1.2) |
| The host process survives | The fault surfaces to `await` as `TaskFaulted` |
| The fault's cause is preserved | The cause is preserved at the faulting task |

So "isolation" means the damage stops at the tree's edge. Conformance is
`tests/concurrency_spec_test.go`, which asserts each clause separately and
records the unimplemented ones rather than leaving them to be discovered.

### 4.2 Known gaps

- **Explicit cancellation is not implemented.** There is no `cancel()`, so §3.1.1
  has no way to be triggered; a deadline (`withTimeout`) is the only route to a
  cancelled tree.
- **A nested fault loses its cause.** When a *child* task faults, the parent
  surfaces `TaskCancelled` rather than the child's error, so a fault two tasks
  deep is indistinguishable from a timeout. At the top level the fault is typed
  and names its cause.