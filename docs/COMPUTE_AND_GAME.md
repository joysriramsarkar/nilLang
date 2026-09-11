# Compute, Game, Data, and AI Foundation

NilLang provides a shared numerical runtime for real-time game logic, data-science workloads, and AI model code. The same API executes under the tree-walking evaluator and bytecode VM.

## Dense tensors

Dense tensors use contiguous CPU storage with row-major shapes. The current runtime computes through a `float64` buffer while enforcing each tensor's logical dtype at construction and result boundaries.

| Function | Purpose |
| --- | --- |
| `tensor(data, shape[, dtype])` | Create a dense tensor; dtype defaults to `float64` |
| `tensorShape(value)` | Return tensor dimensions |
| `tensorDtype(value)` | Return `float64`, `float32`, or `int32` |
| `tensorCast(value, dtype)` | Create an independent tensor converted to a dtype |
| `tensorGet(value, indices)` | Read one element |
| `tensorAdd(left, right)` | Element-wise addition with right-aligned broadcasting |
| `tensorMul(left, right)` | Element-wise multiplication with right-aligned broadcasting |
| `tensorSlice(value, starts, ends)` | Copy a zero-based, end-exclusive dense slice |
| `tensorDot(left, right)` | Dot product of equal-length vectors |
| `tensorMatmul(left, right)` | Rank-2 matrix multiplication |
| `tensorSum(value)` | Reduce all elements to a scalar |

Rank-2 matrix multiplication uses a cache-local CPU kernel. Large workloads are partitioned into bounded, contiguous row ranges based on `GOMAXPROCS`; small workloads remain serial to avoid scheduling overhead. Each output cell retains deterministic inner-dimension accumulation order.

## Dtypes

`float64` preserves current values. `float32` rounds values through IEEE-754 binary32 at tensor boundaries. `int32` truncates finite fractional values toward zero and rejects values outside the signed 32-bit range, NaN, and infinity.

Binary tensor operations promote `int32` with `float32` to `float32`; an operation involving `float64` produces `float64`. Matching operands preserve their dtype. Slices preserve dtype, while casts always create an independent copy. CPU arithmetic and reductions currently accumulate in `float64`, then quantize tensor results to their promoted dtype. This avoids integer wrapping and defines stable behavior while native dtype-specific CPU and GPU kernels are developed.

Run the cross-domain example:

```powershell
go run ./cmd/nil run examples/compute-platform/src/main.nil
go run ./cmd/nil run -vm examples/compute-platform/src/main.nil
```

The example performs batched point transforms, entity selection and scaling, a data reduction, and a dense neural-network layer over a sliced batch.

## Game profile

Projects can select `"profile": "game"`. The profile permits GPU, audio, asset filesystem, network, process, sensor, AI, and cryptographic capabilities. Camera, location, Bluetooth, and database access remain restricted.

Create a ready Android/Godot/Unity/Unreal starter:

```powershell
go run ./cmd/nil game init my-game --engine all
cd my-game
nil game doctor
nil build android
```

See [GAME_ENGINE_READY.md](GAME_ENGINE_READY.md) for the engine adapter layout and export workflow.

## Current boundary

This foundation makes numerical game systems, simulation logic, data transformations, and small CPU inference workloads practical in NilLang. It does not by itself constitute a complete AAA engine or production ML framework.

Production AAA and large-model workloads still require:

- Real Vulkan, Direct3D 12, or Metal command submission instead of the current simulated renderer backends.
- SIMD, broader multithreaded tensor kernels, GPU compute dispatch, device memory, and asynchronous synchronization.
- Physics, animation graphs, audio mixing, input, scene streaming, asset cooking, and editor tooling.
- Additional dtypes and native dtype-specific storage, automatic differentiation, optimizers, model serialization, and ONNX interoperability.

The contiguous tensor object is the stable buffer boundary for implementing those native and accelerator backends without changing Nil source programs.
