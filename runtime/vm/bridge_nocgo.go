//go:build !cgo || (!linux && !darwin && !onuron)

package vm

// NativeBridge — pure-Go fallback (L8.2).
//
// Role: when cgo is off (notably every mobile cross-build: Android NDK and
// iOS toolchains never enable this package's cgo path), the VM still needs
// time/sleep/math/device answers. This file provides them in pure Go with
// identical signatures to bridge_cgo.go, so callers never branch on build
// tags. Behavioural parity (same Call names, same error strings) is the
// contract; `TestNativeBridgeParity` in the vm package guards it.
//
// Mobile note: the shipping artefact for phones is NOT this bridge — it is
// `libnilang_core.so` (see docs/spec/MOBILE_RUNTIME.md), a small C-ABI core
// built from runtime/core + runtime/vm/native. This fallback exists so the
// desktop VM, CI and device-preview hosts run without a Rust toolchain.

import (
	"fmt"
	"time"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// NativeBridge provides access to the native library (pure Go fallback when cgo is disabled)
type NativeBridge struct {
	initialized bool
}

var nativeBridge *NativeBridge

func GetNativeBridge() *NativeBridge {
	if nativeBridge == nil {
		nativeBridge = &NativeBridge{initialized: true}
	}
	return nativeBridge
}

func (nb *NativeBridge) IsAvailable() bool {
	return true
}

func (nb *NativeBridge) Version() string {
	return "0.1.0-purego"
}

func (nb *NativeBridge) MathAdd(a, b int64) int64 {
	return a + b
}

func (nb *NativeBridge) MathSub(a, b int64) int64 {
	return a - b
}

func (nb *NativeBridge) MathMul(a, b int64) int64 {
	return a * b
}

func (nb *NativeBridge) MathDiv(a, b int64) (int64, error) {
	if b == 0 {
		return 0, fmt.Errorf("division by zero")
	}
	return a / b, nil
}

func (nb *NativeBridge) MathMod(a, b int64) (int64, error) {
	if b == 0 {
		return 0, fmt.Errorf("modulo by zero")
	}
	return a % b, nil
}

func (nb *NativeBridge) TimeMs() int64 {
	return time.Now().UnixMilli()
}

func (nb *NativeBridge) SleepMs(ms int64) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

func (nb *NativeBridge) BatteryLevel() int {
	return 100
}

func (nb *NativeBridge) DeviceModel() string {
	return "Onuron Generic Device"
}

func (nb *NativeBridge) OSVersion() string {
	return "Onuron OS 1.0 (PureGo)"
}

func (nb *NativeBridge) HashFNV1a(data string) uint64 {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(data); i++ {
		h ^= uint64(data[i])
		h *= 1099511628211
	}
	return h
}

func (nb *NativeBridge) Call(name string, args []object.Object) (object.Object, error) {
	switch name {
	case "native.version":
		return &object.String{Value: nb.Version()}, nil
	case "native.time":
		return &object.Integer{Value: nb.TimeMs()}, nil
	case "onuron.battery":
		return &object.Integer{Value: int64(nb.BatteryLevel())}, nil
	case "onuron.device":
		return &object.String{Value: nb.DeviceModel()}, nil
	case "onuron.version":
		return &object.String{Value: nb.OSVersion()}, nil
	default:
		return nil, fmt.Errorf("unknown native function: %s", name)
	}
}
