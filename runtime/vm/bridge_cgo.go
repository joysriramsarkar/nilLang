//go:build (linux || darwin || onuron) && cgo

package vm

/*
#cgo LDFLAGS: -L${SRCDIR}/native/target/release -lnilang_native -lm -lpthread -ldl
#include <stdlib.h>

// Memory
extern void* nilang_alloc(size_t size);
extern void nilang_free(void* ptr, size_t size);

// String
extern char* nilang_string_new(const char* data, size_t len);
extern void nilang_string_free(char* ptr);
extern size_t nilang_string_len(const char* ptr);
extern char* nilang_string_concat(const char* a, const char* b);

// Math
extern long long nilang_math_add(long long a, long long b);
extern long long nilang_math_sub(long long a, long long b);
extern long long nilang_math_mul(long long a, long long b);
extern long long nilang_math_div(long long a, long long b);
extern long long nilang_math_mod(long long a, long long b);
extern double nilang_math_pow(double base, double exp);
extern double nilang_math_sqrt(double x);

// System
extern void nilang_sys_print(const char* msg);
extern void nilang_sys_println(const char* msg);
extern long long nilang_sys_time_ms();
extern void nilang_sys_sleep_ms(long long ms);

// Onuron OS
extern int nilang_onuron_battery_level();
extern char* nilang_onuron_device_model();
extern char* nilang_onuron_os_version();

// Array
extern long long nilang_array_sum_i64(const long long* arr, size_t len);
extern long long nilang_array_max_i64(const long long* arr, size_t len);

// Hash
extern unsigned long long nilang_hash_fnv1a(const char* data, size_t len);

// Version
extern char* nilang_native_version();
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// NativeBridge provides access to the Rust native library
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

// ============================================================
// Built-in functions that use the native library
// ============================================================

func GetNativeBuiltins() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"native_print": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return newError("native_print expects 1 argument, got %d", len(args))
				}
				str := fmt.Sprintf("%v", args[0].Inspect())
				cstr := C.CString(str)
				defer C.free(unsafe.Pointer(cstr))
				C.nilang_sys_print(cstr)
				return &object.Null{}
			},
		},

		"native_println": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return newError("native_println expects 1 argument, got %d", len(args))
				}
				str := fmt.Sprintf("%v", args[0].Inspect())
				cstr := C.CString(str)
				defer C.free(unsafe.Pointer(cstr))
				C.nilang_sys_println(cstr)
				return &object.Null{}
			},
		},

		"native_time": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 0 {
					return newError("native_time expects 0 arguments, got %d", len(args))
				}
				ms := C.nilang_sys_time_ms()
				return &object.Integer{Value: int64(ms)}
			},
		},

		"native_sleep": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return newError("native_sleep expects 1 argument, got %d", len(args))
				}
				ms, ok := args[0].(*object.Integer)
				if !ok {
					return newError("native_sleep expects an integer argument")
				}
				C.nilang_sys_sleep_ms(C.longlong(ms.Value))
				return &object.Null{}
			},
		},

		"native_battery": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 0 {
					return newError("native_battery expects 0 arguments, got %d", len(args))
				}
				level := C.nilang_onuron_battery_level()
				return &object.Integer{Value: int64(level)}
			},
		},

		"native_device": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 0 {
					return newError("native_device expects 0 arguments, got %d", len(args))
				}
				cstr := C.nilang_onuron_device_model()
				defer C.nilang_string_free(cstr)
				if cstr == nil {
					return &object.String{Value: "Unknown"}
				}
				goStr := C.GoString(cstr)
				return &object.String{Value: goStr}
			},
		},

		"native_os_version": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 0 {
					return newError("native_os_version expects 0 arguments, got %d", len(args))
				}
				cstr := C.nilang_onuron_os_version()
				defer C.nilang_string_free(cstr)
				if cstr == nil {
					return &object.String{Value: "Unknown"}
				}
				goStr := C.GoString(cstr)
				return &object.String{Value: goStr}
			},
		},

		"native_sqrt": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return newError("native_sqrt expects 1 argument, got %d", len(args))
				}

				switch arg := args[0].(type) {
				case *object.Integer:
					result := C.nilang_math_sqrt(C.double(float64(arg.Value)))
					return &object.Integer{Value: int64(result)}
				default:
					return newError("native_sqrt expects a numeric argument")
				}
			},
		},

		"native_hash": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return newError("native_hash expects 1 argument, got %d", len(args))
				}

				str, ok := args[0].(*object.String)
				if !ok {
					return newError("native_hash expects a string argument")
				}

				cstr := C.CString(str.Value)
				defer C.free(unsafe.Pointer(cstr))
				hash := C.nilang_hash_fnv1a(cstr, C.size_t(len(str.Value)))
				return &object.Integer{Value: int64(hash)}
			},
		},

		"native_version": {
			Fn: func(args ...object.Object) object.Object {
				cstr := C.nilang_native_version()
				defer C.nilang_string_free(cstr)
				if cstr == nil {
					return &object.String{Value: "unknown"}
				}
				goStr := C.GoString(cstr)
				return &object.String{Value: goStr}
			},
		},
	}
}

// Helper function for creating errors
func newError(format string, a ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}