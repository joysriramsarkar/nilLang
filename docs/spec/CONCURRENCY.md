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

**Single-Resolution Invariant**: A task's terminal state is immutable and observed by `await` exactly once.

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

1. **Fault Isolation**: A panic or runtime exception occurring inside a task does not crash the host process or sibling tasks.
2. The runtime scheduler intercepts the exception, wraps it in a typed `TaskFaultError`, and transitions the task to the `Faulted` state.