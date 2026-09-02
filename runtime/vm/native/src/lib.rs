//! Nilang Native Runtime Library
//! Provides C ABI functions for the Nilang VM to call into native Rust code.
//! This is the bridge between Nilang and Onuron OS.

use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_int, c_void};
use std::ptr;
use std::slice;

// ============================================================
// Memory Management
// ============================================================

/// Allocate memory on the native heap
#[no_mangle]
pub extern "C" fn nilang_alloc(size: usize) -> *mut c_void {
    let mut buffer = Vec::with_capacity(size);
    let ptr = buffer.as_mut_ptr();
    std::mem::forget(buffer);
    ptr as *mut c_void
}

/// Free memory allocated by nilang_alloc
#[no_mangle]
pub extern "C" fn nilang_free(ptr: *mut c_void, size: usize) {
    if !ptr.is_null() {
        unsafe {
            let _ = Vec::from_raw_parts(ptr, 0, size);
        }
    }
}

// ============================================================
// String Operations
// ============================================================

/// Create a new CString from a byte slice
#[no_mangle]
pub extern "C" fn nilang_string_new(data: *const c_char, len: usize) -> *mut c_char {
    if data.is_null() {
        return ptr::null_mut();
    }

    let bytes = unsafe { slice::from_raw_parts(data as *const u8, len) };
    match CString::new(bytes) {
        Ok(cstring) => cstring.into_raw(),
        Err(_) => ptr::null_mut(),
    }
}

/// Free a CString
#[no_mangle]
pub extern "C" fn nilang_string_free(ptr: *mut c_char) {
    if !ptr.is_null() {
        unsafe {
            let _ = CString::from_raw(ptr);
        }
    }
}

/// Get the length of a CString
#[no_mangle]
pub extern "C" fn nilang_string_len(ptr: *const c_char) -> usize {
    if ptr.is_null() {
        return 0;
    }

    let cstr = unsafe { CStr::from_ptr(ptr) };
    cstr.to_bytes().len()
}

/// Concatenate two strings
#[no_mangle]
pub extern "C" fn nilang_string_concat(
    a: *const c_char,
    b: *const c_char,
) -> *mut c_char {
    if a.is_null() || b.is_null() {
        return ptr::null_mut();
    }

    let a_str = unsafe { CStr::from_ptr(a) };
    let b_str = unsafe { CStr::from_ptr(b) };

    let mut result = a_str.to_bytes().to_vec();
    result.extend_from_slice(b_str.to_bytes());

    match CString::new(result) {
        Ok(cstring) => cstring.into_raw(),
        Err(_) => ptr::null_mut(),
    }
}

// ============================================================
// Math Operations (Optimized)
// ============================================================

#[no_mangle]
pub extern "C" fn nilang_math_add(a: i64, b: i64) -> i64 {
    a.wrapping_add(b)
}

#[no_mangle]
pub extern "C" fn nilang_math_sub(a: i64, b: i64) -> i64 {
    a.wrapping_sub(b)
}

#[no_mangle]
pub extern "C" fn nilang_math_mul(a: i64, b: i64) -> i64 {
    a.wrapping_mul(b)
}

#[no_mangle]
pub extern "C" fn nilang_math_div(a: i64, b: i64) -> i64 {
    if b == 0 {
        return 0; // Handle division by zero
    }
    a / b
}

#[no_mangle]
pub extern "C" fn nilang_math_mod(a: i64, b: i64) -> i64 {
    if b == 0 {
        return 0;
    }
    a % b
}

#[no_mangle]
pub extern "C" fn nilang_math_pow(base: f64, exp: f64) -> f64 {
    base.powf(exp)
}

#[no_mangle]
pub extern "C" fn nilang_math_sqrt(x: f64) -> f64 {
    if x < 0.0 {
        return f64::NAN;
    }
    x.sqrt()
}

// ============================================================
// System / Onuron OS Integration
// ============================================================

/// Print to stdout (for debugging and basic I/O)
#[no_mangle]
pub extern "C" fn nilang_sys_print(msg: *const c_char) {
    if msg.is_null() {
        return;
    }

    let cstr = unsafe { CStr::from_ptr(msg) };
    if let Ok(str_slice) = cstr.to_str() {
        print!("{}", str_slice);
    }
}

/// Print to stdout with newline
#[no_mangle]
pub extern "C" fn nilang_sys_println(msg: *const c_char) {
    if msg.is_null() {
        println!();
        return;
    }

    let cstr = unsafe { CStr::from_ptr(msg) };
    if let Ok(str_slice) = cstr.to_str() {
        println!("{}", str_slice);
    }
}

/// Get current timestamp in milliseconds
#[no_mangle]
pub extern "C" fn nilang_sys_time_ms() -> i64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as i64
}

/// Sleep for specified milliseconds
#[no_mangle]
pub extern "C" fn nilang_sys_sleep_ms(ms: i64) {
    if ms > 0 {
        std::thread::sleep(std::time::Duration::from_millis(ms as u64));
    }
}

// ============================================================
// Onuron OS Specific (HAL Integration Points)
// ============================================================

/// Get battery level (0-100)
/// TODO: Implement actual HAL call when Onuron OS HAL is ready
#[no_mangle]
pub extern "C" fn nilang_onuron_battery_level() -> c_int {
    // Placeholder - will call Onuron OS HAL when available
    100
}

/// Get device model name
#[no_mangle]
pub extern "C" fn nilang_onuron_device_model() -> *mut c_char {
    match CString::new("Onuron OS Device") {
        Ok(cstring) => cstring.into_raw(),
        Err(_) => ptr::null_mut(),
    }
}

/// Get OS version
#[no_mangle]
pub extern "C" fn nilang_onuron_os_version() -> *mut c_char {
    match CString::new("Onuron OS 1.0.0") {
        Ok(cstring) => cstring.into_raw(),
        Err(_) => ptr::null_mut(),
    }
}

// ============================================================
// Array Operations
// ============================================================

/// Sum of integer array
#[no_mangle]
pub extern "C" fn nilang_array_sum_i64(arr: *const i64, len: usize) -> i64 {
    if arr.is_null() || len == 0 {
        return 0;
    }

    let slice = unsafe { slice::from_raw_parts(arr, len) };
    slice.iter().sum()
}

/// Find max in integer array
#[no_mangle]
pub extern "C" fn nilang_array_max_i64(arr: *const i64, len: usize) -> i64 {
    if arr.is_null() || len == 0 {
        return i64::MIN;
    }

    let slice = unsafe { slice::from_raw_parts(arr, len) };
    slice.iter().max().copied().unwrap_or(i64::MIN)
}

// ============================================================
// Hashing & Cryptography (Basic)
// ============================================================

/// Simple FNV-1a hash for strings
#[no_mangle]
pub extern "C" fn nilang_hash_fnv1a(data: *const c_char, len: usize) -> u64 {
    if data.is_null() {
        return 0;
    }

    let bytes = unsafe { slice::from_raw_parts(data as *const u8, len) };
    let mut hash: u64 = 0xcbf29ce484222325;

    for byte in bytes {
        hash ^= *byte as u64;
        hash = hash.wrapping_mul(0x100000001b3);
    }

    hash
}

// ============================================================
// Version Info
// ============================================================

#[no_mangle]
pub extern "C" fn nilang_native_version() -> *mut c_char {
    match CString::new("0.1.0") {
        Ok(cstring) => cstring.into_raw(),
        Err(_) => ptr::null_mut(),
    }
}

// ============================================================
// Tests
// ============================================================

#[cfg(test)]
mod tests {
    use super::*;
    use std::ffi::CString;

    #[test]
    fn test_math_operations() {
        assert_eq!(nilang_math_add(5, 3), 8);
        assert_eq!(nilang_math_sub(5, 3), 2);
        assert_eq!(nilang_math_mul(5, 3), 15);
        assert_eq!(nilang_math_div(10, 3), 3);
        assert_eq!(nilang_math_mod(10, 3), 1);
    }

    #[test]
    fn test_string_operations() {
        let a = CString::new("Hello").unwrap();
        let b = CString::new(" World").unwrap();

        let result = nilang_string_concat(a.as_ptr(), b.as_ptr());
        assert!(!result.is_null());

        let result_str = unsafe { CStr::from_ptr(result) };
        assert_eq!(result_str.to_str().unwrap(), "Hello World");

        nilang_string_free(result);
    }

    #[test]
    fn test_array_operations() {
        let arr: Vec<i64> = vec![1, 2, 3, 4, 5];
        let sum = nilang_array_sum_i64(arr.as_ptr(), arr.len());
        assert_eq!(sum, 15);

        let max = nilang_array_max_i64(arr.as_ptr(), arr.len());
        assert_eq!(max, 5);
    }
}